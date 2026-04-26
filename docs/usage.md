# Usage

## Python CLI

```bash
python -m quantflow run "<thesis>" --ticker XBI --lookback-days 252 --offline --verbose
```

Flags:

- `--ticker`: override ticker inference
- `--lookback-days`: control bar count for backtest
- `--offline`: deterministic fixture mode
- `--verbose`: print stage-by-stage progress

## Go CLI Wrapper

```bash
go run . run "<thesis>" --ticker XBI --offline --verbose
```

## Go Bubble Tea TUI

```bash
go run . tui
```

### TUI Controls

- `tab` / `up` / `down`: switch focused input
- `o`: toggle offline mode
- `enter`: run pipeline
- `r`: rerun
- `q`: quit

## DuckDB Queries

Example:

```sql
SELECT run_id, ticker, sharpe, max_drawdown, calmar
FROM backtests
ORDER BY run_id DESC
LIMIT 10;
```
