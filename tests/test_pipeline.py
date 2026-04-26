from pathlib import Path

from quantflow.config import QuantFlowConfig
from quantflow.orchestrator import QuantFlowOrchestrator


def test_offline_pipeline_generates_artifacts(tmp_path: Path) -> None:
    config = QuantFlowConfig(
        db_path=tmp_path / "quantflow.duckdb",
        output_root=tmp_path / "runs",
        offline_mode=True,
    )
    orchestrator = QuantFlowOrchestrator(config)
    result = orchestrator.run('short small-cap biotech on FDA rejection patterns', ticker="XBI", lookback_days=90)

    assert result.ticker == "XBI"
    assert Path(result.report_path).exists()
    assert Path(result.readme_path).exists()
    assert Path(result.chart_path).exists()
    assert Path(result.strategy_path).exists()
    assert result.backtest.side in {"short", "long"}
