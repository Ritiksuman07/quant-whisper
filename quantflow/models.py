from dataclasses import dataclass


@dataclass(slots=True)
class FilingRecord:
    ticker: str
    cik: str
    form: str
    filed_at: str
    accession_no: str
    primary_doc: str
    risk_flag: str
    extracted_signal: str


@dataclass(slots=True)
class SentimentSnapshot:
    ticker: str
    subreddit: str
    mentions: int
    sentiment_score: float
    anomaly_score: float
    captured_at: str


@dataclass(slots=True)
class StrategyPlan:
    thesis: str
    ticker: str
    side: str
    rationale: str
    rules: list[str]
    generated_code: str


@dataclass(slots=True)
class BacktestResult:
    ticker: str
    side: str
    start_date: str
    end_date: str
    sharpe: float
    max_drawdown: float
    calmar: float
    cagr: float
    total_return: float
    equity_curve: list[dict[str, float | str]]


@dataclass(slots=True)
class PipelineResult:
    run_id: str
    ticker: str
    output_dir: str
    report_path: str
    readme_path: str
    chart_path: str
    strategy_path: str
    backtest: BacktestResult
