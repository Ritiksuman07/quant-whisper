# Architecture Deep Dive

## End-to-End Flow

1. **Thesis Input**  
   User provides a natural-language thesis and optional ticker override.
2. **SEC Filing Agent**  
   Pulls `10-K`, `10-Q`, `8-K` metadata and derives risk-oriented signals.
3. **Reddit Sentiment Agent**  
   Measures ticker mentions, directional sentiment, and anomaly score.
4. **Strategy Orchestrator**  
   Converts thesis + agent signals into a directional strategy plan and generated strategy code.
5. **Deterministic Backtest Engine**  
   Simulates returns, computes Sharpe / max drawdown / Calmar / CAGR.
6. **DuckDB Analytics Layer**  
   Persists filings, sentiment snapshots, strategy payloads, and backtest metrics.
7. **Artifact Exporter**  
   Writes report JSON, per-run README, strategy file, and equity-curve SVG.

## Deterministic Backtest Design

- Input is an explicit daily close series.
- Position direction is fixed by strategy side (`long` or `short`).
- Metrics are computed from daily return sequence in a deterministic order.
- Replay of identical input data returns identical output metrics and curve.

## Runtime Surfaces

- **Python runtime**: orchestration and data agents.
- **Go runtime**: operator experience via Bubble Tea TUI.
- **Rust runtime (scaffold)**: future high-performance deterministic simulation binary.

## Failure and Fallback Strategy

- SEC/Reddit fetchers fallback to offline fixtures when API calls fail.
- Market data provider falls back to synthetic deterministic bars.
- Rust backtest invocation falls back to Python engine on subprocess failure.
