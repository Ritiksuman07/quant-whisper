from __future__ import annotations

from dataclasses import asdict
import json
from math import sqrt
from pathlib import Path
import subprocess

from quantflow.models import BacktestResult


class BacktestEngine:
    def __init__(self, rust_binary: Path | None = None):
        self._rust_binary = rust_binary

    def run(self, ticker: str, side: str, prices: list[tuple[str, float]]) -> BacktestResult:
        if len(prices) < 3:
            raise ValueError("Need at least 3 price points for backtest.")
        if self._rust_binary and self._rust_binary.exists():
            try:
                rust_result = self._run_rust(side=side, prices=prices)
                return BacktestResult(
                    ticker=ticker,
                    side=side,
                    start_date=prices[0][0],
                    end_date=prices[-1][0],
                    sharpe=float(rust_result["sharpe"]),
                    max_drawdown=float(rust_result["max_drawdown"]),
                    calmar=float(rust_result["calmar"]),
                    cagr=float(rust_result["cagr"]),
                    total_return=float(rust_result["total_return"]),
                    equity_curve=rust_result["equity_curve"],
                )
            except (subprocess.SubprocessError, json.JSONDecodeError, KeyError, OSError):
                pass
        return self._run_python(ticker=ticker, side=side, prices=prices)

    def _run_rust(self, side: str, prices: list[tuple[str, float]]) -> dict:
        payload = {
            "side": side,
            "prices": [{"date": day, "close": close} for day, close in prices],
        }
        proc = subprocess.run(
            [str(self._rust_binary)],
            input=json.dumps(payload),
            capture_output=True,
            text=True,
            check=True,
        )
        return json.loads(proc.stdout)

    @staticmethod
    def _run_python(ticker: str, side: str, prices: list[tuple[str, float]]) -> BacktestResult:
        normalized_side = side.lower()
        multiplier = -1.0 if normalized_side == "short" else 1.0
        returns: list[float] = []
        equity_curve: list[dict[str, str | float]] = [{"date": prices[0][0], "equity": 100_000.0}]
        equity = 100_000.0
        peak = equity
        max_drawdown = 0.0

        for index in range(1, len(prices)):
            previous_close = prices[index - 1][1]
            current_close = prices[index][1]
            daily_return = multiplier * ((current_close / previous_close) - 1.0)
            returns.append(daily_return)
            equity *= 1.0 + daily_return
            peak = max(peak, equity)
            drawdown = (equity / peak) - 1.0
            if drawdown < max_drawdown:
                max_drawdown = drawdown
            equity_curve.append({"date": prices[index][0], "equity": round(equity, 4)})

        mean_return = sum(returns) / len(returns)
        variance = sum((value - mean_return) ** 2 for value in returns) / max(1, len(returns) - 1)
        std_dev = variance**0.5
        sharpe = (mean_return / std_dev) * sqrt(252.0) if std_dev > 0 else 0.0
        years = len(returns) / 252.0
        total_return = (equity / 100_000.0) - 1.0
        cagr = ((1.0 + total_return) ** (1.0 / max(years, 1 / 252.0))) - 1.0
        calmar = cagr / abs(max_drawdown) if max_drawdown < 0 else 0.0

        return BacktestResult(
            ticker=ticker,
            side=normalized_side,
            start_date=prices[0][0],
            end_date=prices[-1][0],
            sharpe=round(sharpe, 4),
            max_drawdown=round(max_drawdown, 4),
            calmar=round(calmar, 4),
            cagr=round(cagr, 4),
            total_return=round(total_return, 4),
            equity_curve=equity_curve,
        )

    @staticmethod
    def as_dict(backtest: BacktestResult) -> dict:
        return asdict(backtest)
