from __future__ import annotations

import argparse
from pathlib import Path

from quantflow.config import QuantFlowConfig
from quantflow.orchestrator import QuantFlowOrchestrator


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="quantflow",
        description="Autonomous agentic framework prototype for quantitative finance.",
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    run_parser = subparsers.add_parser("run", help="Run end-to-end quant thesis pipeline.")
    run_parser.add_argument("thesis", help="Natural language trading thesis.")
    run_parser.add_argument("--ticker", help="Override inferred ticker symbol.", default=None)
    run_parser.add_argument(
        "--db-path",
        help="Path to DuckDB analytics database.",
        default="quantflow.duckdb",
    )
    run_parser.add_argument(
        "--output-root",
        help="Directory for run artifacts.",
        default="runs",
    )
    run_parser.add_argument(
        "--lookback-days",
        help="Number of daily bars for backtest.",
        type=int,
        default=252,
    )
    run_parser.add_argument(
        "--offline",
        help="Disable external API calls and use deterministic fixtures.",
        action="store_true",
    )
    run_parser.add_argument(
        "--user-agent",
        help="HTTP user-agent used for SEC and Reddit requests.",
        default="QuantFlow/0.1 (contact: quantflow@example.com)",
    )
    run_parser.add_argument(
        "--verbose",
        help="Print stage-by-stage orchestration logs.",
        action="store_true",
    )

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    if args.command == "run":
        config = QuantFlowConfig(
            db_path=Path(args.db_path),
            output_root=Path(args.output_root),
            user_agent=args.user_agent,
            offline_mode=bool(args.offline),
        )
        orchestrator = QuantFlowOrchestrator(config)
        progress_hook = print if args.verbose else None
        result = orchestrator.run(
            thesis=args.thesis,
            ticker=args.ticker,
            lookback_days=args.lookback_days,
            progress=progress_hook,
        )
        print(f"QuantFlow run complete: {result.run_id}")
        print(f"Ticker: {result.ticker}")
        print(f"Output: {result.output_dir}")
        print(f"Report: {result.report_path}")
        print(f"README: {result.readme_path}")
        print(
            "Metrics: "
            f"Sharpe={result.backtest.sharpe}, "
            f"MaxDD={result.backtest.max_drawdown}, "
            f"Calmar={result.backtest.calmar}"
        )
        return 0

    parser.print_help()
    return 1
