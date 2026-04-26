# LLM + DuckDB Design

## Current State

The current `StrategyOrchestratorAgent` is deterministic and heuristic-driven for reproducibility in open-source runs.

## Why DuckDB Is Central

DuckDB stores all key intermediate outputs:

- filing extracts and risk flags
- sentiment snapshots by subreddit
- strategy payloads and generated code
- backtest summary metrics

This makes it a natural memory layer for future LLM-assisted reasoning.

## LLM Integration Pattern (Planned)

1. Retrieve thesis and recent run context from DuckDB.
2. Build a constrained prompt with normalized market/sentiment/fundamental signals.
3. Generate a structured strategy object (JSON schema constrained).
4. Run deterministic validation + backtest before writing artifacts.
5. Persist prompt metadata, model ID, and strategy provenance.

## Guardrails

- Keep deterministic backtest logic independent of model output.
- Reject strategies with malformed or unsafe rule payloads.
- Surface explainable rationale fields in exported reports.
- Run lints for look-ahead and survivorship bias in future iterations.

## Fine-Tuning Status

No LoRA or custom fine-tuning pipeline is currently applied in this repository.  
The integration surface is designed so those additions can be implemented without rewiring data storage or execution.
