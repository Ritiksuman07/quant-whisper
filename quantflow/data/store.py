from __future__ import annotations

from dataclasses import asdict
from pathlib import Path

from quantflow.models import BacktestResult, FilingRecord, SentimentSnapshot, StrategyPlan

try:
    import duckdb
except ImportError as exc:  # pragma: no cover
    raise RuntimeError(
        "duckdb is required. Install dependencies with `pip install -r requirements.txt`."
    ) from exc


class DuckDBStore:
    def __init__(self, db_path: Path):
        self._db_path = db_path
        self._conn = duckdb.connect(str(db_path))
        self._ensure_schema()

    def close(self) -> None:
        self._conn.close()

    def __enter__(self) -> "DuckDBStore":
        return self

    def __exit__(self, exc_type, exc, exc_tb) -> None:
        self.close()

    def _ensure_schema(self) -> None:
        self._conn.execute(
            """
            CREATE TABLE IF NOT EXISTS filings (
                run_id TEXT,
                ticker TEXT,
                cik TEXT,
                form TEXT,
                filed_at DATE,
                accession_no TEXT,
                primary_doc TEXT,
                risk_flag TEXT,
                extracted_signal TEXT
            );
            """
        )
        self._conn.execute(
            """
            CREATE TABLE IF NOT EXISTS sentiment (
                run_id TEXT,
                ticker TEXT,
                subreddit TEXT,
                mentions INTEGER,
                sentiment_score DOUBLE,
                anomaly_score DOUBLE,
                captured_at TIMESTAMP
            );
            """
        )
        self._conn.execute(
            """
            CREATE TABLE IF NOT EXISTS strategies (
                run_id TEXT,
                ticker TEXT,
                thesis TEXT,
                side TEXT,
                rationale TEXT,
                rules_json TEXT,
                generated_code TEXT
            );
            """
        )
        self._conn.execute(
            """
            CREATE TABLE IF NOT EXISTS backtests (
                run_id TEXT,
                ticker TEXT,
                side TEXT,
                start_date DATE,
                end_date DATE,
                sharpe DOUBLE,
                max_drawdown DOUBLE,
                calmar DOUBLE,
                cagr DOUBLE,
                total_return DOUBLE
            );
            """
        )

    def insert_filings(self, run_id: str, rows: list[FilingRecord]) -> None:
        payload = [
            (
                run_id,
                row.ticker,
                row.cik,
                row.form,
                row.filed_at,
                row.accession_no,
                row.primary_doc,
                row.risk_flag,
                row.extracted_signal,
            )
            for row in rows
        ]
        if payload:
            self._conn.executemany(
                """
                INSERT INTO filings (
                    run_id, ticker, cik, form, filed_at, accession_no, primary_doc, risk_flag, extracted_signal
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                payload,
            )

    def insert_sentiment(self, run_id: str, rows: list[SentimentSnapshot]) -> None:
        payload = [
            (
                run_id,
                row.ticker,
                row.subreddit,
                row.mentions,
                row.sentiment_score,
                row.anomaly_score,
                row.captured_at,
            )
            for row in rows
        ]
        if payload:
            self._conn.executemany(
                """
                INSERT INTO sentiment (
                    run_id, ticker, subreddit, mentions, sentiment_score, anomaly_score, captured_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                payload,
            )

    def insert_strategy(self, run_id: str, strategy: StrategyPlan) -> None:
        self._conn.execute(
            """
            INSERT INTO strategies (
                run_id, ticker, thesis, side, rationale, rules_json, generated_code
            ) VALUES (?, ?, ?, ?, ?, ?, ?)
            """,
            (
                run_id,
                strategy.ticker,
                strategy.thesis,
                strategy.side,
                strategy.rationale,
                str(strategy.rules),
                strategy.generated_code,
            ),
        )

    def insert_backtest(self, run_id: str, backtest: BacktestResult) -> None:
        self._conn.execute(
            """
            INSERT INTO backtests (
                run_id, ticker, side, start_date, end_date, sharpe, max_drawdown, calmar, cagr, total_return
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                run_id,
                backtest.ticker,
                backtest.side,
                backtest.start_date,
                backtest.end_date,
                backtest.sharpe,
                backtest.max_drawdown,
                backtest.calmar,
                backtest.cagr,
                backtest.total_return,
            ),
        )

    def as_dict(self, backtest: BacktestResult) -> dict:
        return asdict(backtest)
