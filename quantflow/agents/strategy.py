from __future__ import annotations

from quantflow.models import StrategyPlan

SHORT_HINTS = {"short", "bear", "bearish", "rejection", "reject", "downside"}
LONG_HINTS = {"long", "bull", "bullish", "rebound", "upside"}


class StrategyOrchestratorAgent:
    def generate(
        self,
        thesis: str,
        ticker: str,
        sec_summary: dict[str, float | int | str],
        sentiment_summary: dict[str, float | int | str],
    ) -> StrategyPlan:
        thesis_lower = thesis.lower()
        sec_risk_score = float(sec_summary.get("risk_score", 0.0))
        sentiment_score = float(sentiment_summary.get("overall_sentiment", 0.0))
        sentiment_anomaly = float(sentiment_summary.get("anomaly_score", 0.0))

        short_hint = any(hint in thesis_lower for hint in SHORT_HINTS)
        long_hint = any(hint in thesis_lower for hint in LONG_HINTS)
        directional_confidence = sec_risk_score + max(0.0, -sentiment_score) + (sentiment_anomaly * 0.08)

        if short_hint and not long_hint:
            side = "short"
        elif long_hint and not short_hint:
            side = "long"
        elif directional_confidence >= 0.65:
            side = "short"
        else:
            side = "long"

        rules = [
            "Hold one position at a time.",
            "Enter on first bar, rebalance daily.",
            "Exit after 21 trading sessions or 9% adverse move.",
            "Size position at 95% notional exposure.",
        ]
        rationale = (
            f"SEC risk score={sec_risk_score:.2f}, sentiment={sentiment_score:.2f}, "
            f"anomaly={sentiment_anomaly:.2f}; selected {side} bias."
        )

        generated_code = self._render_strategy_code(ticker=ticker, side=side, rules=rules)
        return StrategyPlan(
            thesis=thesis,
            ticker=ticker,
            side=side,
            rationale=rationale,
            rules=rules,
            generated_code=generated_code,
        )

    @staticmethod
    def _render_strategy_code(ticker: str, side: str, rules: list[str]) -> str:
        return f"""import backtrader as bt

class QuantFlowStrategy(bt.Strategy):
    params = dict(
        ticker="{ticker}",
        side="{side}",
        max_holding_days=21,
        stop_loss=0.09,
    )

    def __init__(self):
        self.days_held = 0

    def next(self):
        if not self.position:
            size = int((self.broker.getcash() * 0.95) / self.data.close[0])
            if self.p.side == "short":
                self.sell(size=size)
            else:
                self.buy(size=size)
            self.days_held = 0
            return

        self.days_held += 1
        pnl_pct = (self.data.close[0] - self.position.price) / self.position.price
        if self.p.side == "short":
            pnl_pct = -pnl_pct
        if self.days_held >= self.p.max_holding_days or pnl_pct <= -self.p.stop_loss:
            self.close()

# strategy rules:
{chr(10).join(f"# - {rule}" for rule in rules)}
"""
