from dataclasses import dataclass
from pathlib import Path


@dataclass(slots=True)
class QuantFlowConfig:
    db_path: Path = Path("quantflow.duckdb")
    output_root: Path = Path("runs")
    user_agent: str = "QuantFlow/0.1 (contact: quantflow@example.com)"
    http_timeout_seconds: float = 12.0
    offline_mode: bool = False
    sec_forms: tuple[str, ...] = ("10-K", "10-Q", "8-K")
