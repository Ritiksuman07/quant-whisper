# Quant Whisperer

Terminal-native algorithmic trading execution engine built in Go with local-first AI inference, deterministic execution controls, and SQLite-backed paper/live trade history.

Tagline: **"A hedge fund on your local machine. BYOB."**

## What Is Implemented

- P0 core loop with paper trading simulator.
- Broker adapter normalization layer for `zerodha`, `dhan`, and `ib` payload shapes.
- Local LLM processing via Ollama (`qwen3:0.8b` default).
- Strict JSON decision protocol:

```json
{"action":"BUY|SELL|HOLD","confidence":0.0,"reasoning":"brief string"}
```

- SQLite persistence for:
  - `decisions`
  - `paper_trades`
  - `live_trades`
  - `pnl_history`
- P1 execution wall for live trading:
  - confidence threshold gate
  - max daily drawdown kill-switch
  - max trades/minute limiter
  - hard position size cap
- Bubble Tea live dashboard with broker status, ticker stream, confidence, trades, and P&L.
- P2 strategy injection from plain text files.
- Optional cloud fallback (OpenAI / Anthropic / DeepSeek) when local inference is unavailable.

## Important Implementation Note

The current broker adapters include schema normalization and deterministic order flow hooks, while market data/order execution are simulated for local development. This keeps the architecture broker-ready without exposing API credentials externally.

## Quick Start

```powershell
go mod tidy
go run . paper --broker zerodha --symbol NIFTY50 --max-ticks 120
```

## Commands

- `go run . paper [flags]`
- `go run . live [flags]`
- `go run . run --mode paper|live [flags]`
- `go run . tui --mode paper|live [flags]`

Useful flags:

- `--broker zerodha|dhan|ib`
- `--symbol NIFTY50`
- `--db quant_whisperer.db`
- `--confidence-threshold 0.85`
- `--max-drawdown 3`
- `--max-trades-per-minute 3`
- `--position-size 10`
- `--capital 100000`
- `--tick-interval-ms 1000`
- `--max-ticks 120`
- `--strategy-file .\strategy.txt`
- `--ollama-url http://localhost:11434`
- `--ollama-model qwen3:0.8b`
- `--cloud-fallback`
- `--cloud-provider openai|anthropic|deepseek`
- `--cloud-model gpt-4o-mini`
- `--cloud-api-key <key>`

## BYOB Credentials (Local Only)

Set credentials in local environment variables:

- Zerodha: `ZERODHA_API_KEY`, `ZERODHA_API_SECRET`, `ZERODHA_ACCESS_TOKEN`
- Dhan: `DHAN_API_KEY`, `DHAN_API_SECRET`, `DHAN_ACCESS_TOKEN`
- Interactive Brokers: `IB_API_KEY`, `IB_API_SECRET`, `IB_ACCESS_TOKEN`

Cloud fallback (optional):

- `QW_ENABLE_CLOUD_FALLBACK=true`
- `QW_CLOUD_PROVIDER=openai` (or `anthropic`, `deepseek`)
- `QW_CLOUD_MODEL=gpt-4o-mini`
- `QW_CLOUD_API_KEY=...`

## Strategy Injection

Create a plain text strategy file and pass it into runtime prompts:

```text
Look for mean reversion after a sharp intraday drop with volume spike confirmation.
```

Run with:

```powershell
go run . paper --strategy-file .\strategy.txt
```

## Data and Security Posture

- API keys are read from local env vars only.
- Local model inference is default.
- Cloud inference is opt-in only.
- Trade and P&L history is written to local SQLite only.

## Development

```powershell
go test ./...
```
