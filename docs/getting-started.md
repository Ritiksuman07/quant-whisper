# Getting Started

QuantFlow turns a natural-language trading thesis into reproducible research artifacts.

## Requirements

- Python 3.11+
- Go 1.22+ (for the Bubble Tea TUI)
- Rust toolchain (optional, for `engine-rs/`)

## Install

```bash
git clone https://github.com/Ritiksuman07/quantflow.git
cd quantflow
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
```

## First Pipeline Run

```bash
python -m quantflow run "short small-cap biotech on FDA rejection patterns" --ticker XBI --offline --verbose
```

Artifacts:

- `runs/<run_id>/report.json`
- `runs/<run_id>/README.md`
- `runs/<run_id>/equity_curve.svg`
- `runs/<run_id>/strategy.py`

Analytics DB:

- `quantflow.duckdb`

## First TUI Run

```bash
go run . tui
```
