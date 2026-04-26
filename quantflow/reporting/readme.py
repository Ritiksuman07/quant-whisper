from __future__ import annotations

from quantflow.models import BacktestResult, StrategyPlan


def build_agentic_alpha_readme(
    thesis: str,
    ticker: str,
    strategy: StrategyPlan,
    sec_summary: dict[str, float | int | str],
    sentiment_summary: dict[str, float | int | str],
    backtest: BacktestResult,
    chart_relative_path: str = "equity_curve.svg",
) -> str:
    return f"""# Agentic Alpha: {ticker}

## Thesis
{thesis}

## Strategy Rationale
{strategy.rationale}

## Signal Snapshot

### SEC Filing Agent
- Filing Count: {sec_summary.get("filing_count", 0)}
- Recent 8-K Count: {sec_summary.get("recent_8k_count", 0)}
- Earnings Delta Score: {sec_summary.get("earnings_delta_score", 0)}
- Risk Score: {sec_summary.get("risk_score", 0)}

### Reddit Sentiment Agent
- Mention Volume: {sentiment_summary.get("mention_volume", 0)}
- Overall Sentiment: {sentiment_summary.get("overall_sentiment", 0)}
- Anomaly Score: {sentiment_summary.get("anomaly_score", 0)}

## Backtest Summary
| Metric | Value |
|---|---|
| Side | {backtest.side} |
| Sharpe | {backtest.sharpe} |
| Max Drawdown | {backtest.max_drawdown} |
| Calmar | {backtest.calmar} |
| CAGR | {backtest.cagr} |
| Total Return | {backtest.total_return} |
| Period | {backtest.start_date} to {backtest.end_date} |

## Equity Curve
![Equity Curve]({chart_relative_path})

## Generated Strategy Rules
{chr(10).join(f"- {rule}" for rule in strategy.rules)}

## Run Command
```bash
python -m quantflow run "{thesis}"
```

## Disclaimer
Backtest outputs are research artifacts and not investment advice.
"""
