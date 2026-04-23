package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/config"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/engine"
)

type runtimeFlags struct {
	broker              string
	symbol              string
	dbPath              string
	strategyPath        string
	confidenceThreshold float64
	maxDrawdownPct      float64
	maxTradesPerMinute  int
	positionSizePct     float64
	startingCapital     float64
	tickIntervalMS      int
	maxTicks            int
	ollamaURL           string
	ollamaModel         string
	enableCloudFallback bool
	cloudProvider       string
	cloudModel          string
	cloudAPIKey         string
}

func defaultRuntimeFlags(mode quantwhisperer.Mode) runtimeFlags {
	defaults := config.DefaultOptions(mode)
	return runtimeFlags{
		broker:              defaults.Broker,
		symbol:              defaults.Symbol,
		dbPath:              defaults.DBPath,
		strategyPath:        defaults.StrategyPath,
		confidenceThreshold: defaults.ConfidenceThreshold,
		maxDrawdownPct:      defaults.MaxDrawdownPct,
		maxTradesPerMinute:  defaults.MaxTradesPerMinute,
		positionSizePct:     defaults.PositionSizePct,
		startingCapital:     defaults.StartingCapital,
		tickIntervalMS:      int(defaults.TickInterval / time.Millisecond),
		maxTicks:            defaults.MaxTicks,
		ollamaURL:           defaults.OllamaURL,
		ollamaModel:         defaults.OllamaModel,
		enableCloudFallback: defaults.EnableCloudFallback,
		cloudProvider:       defaults.CloudProvider,
		cloudModel:          defaults.CloudModel,
		cloudAPIKey:         defaults.CloudAPIKey,
	}
}

type cmdFlagBinder interface {
	StringVar(*string, string, string, string)
	Float64Var(*float64, string, float64, string)
	IntVar(*int, string, int, string)
	BoolVar(*bool, string, bool, string)
}

func bindRuntimeFlags(flags cmdFlagBinder, values *runtimeFlags) {
	flags.StringVar(&values.broker, "broker", values.broker, "broker adapter (zerodha|dhan|ib)")
	flags.StringVar(&values.symbol, "symbol", values.symbol, "trading symbol")
	flags.StringVar(&values.dbPath, "db", values.dbPath, "SQLite path for paper/live history")
	flags.StringVar(&values.strategyPath, "strategy-file", values.strategyPath, "plain-text strategy file to inject into LLM prompt")
	flags.Float64Var(&values.confidenceThreshold, "confidence-threshold", values.confidenceThreshold, "minimum confidence required for live execution")
	flags.Float64Var(&values.maxDrawdownPct, "max-drawdown", values.maxDrawdownPct, "max daily drawdown percent kill-switch")
	flags.IntVar(&values.maxTradesPerMinute, "max-trades-per-minute", values.maxTradesPerMinute, "hard cap on trades per minute")
	flags.Float64Var(&values.positionSizePct, "position-size", values.positionSizePct, "max capital percent per trade")
	flags.Float64Var(&values.startingCapital, "capital", values.startingCapital, "starting portfolio capital")
	flags.IntVar(&values.tickIntervalMS, "tick-interval-ms", values.tickIntervalMS, "simulated tick interval in milliseconds")
	flags.IntVar(&values.maxTicks, "max-ticks", values.maxTicks, "number of ticks to process before exit")
	flags.StringVar(&values.ollamaURL, "ollama-url", values.ollamaURL, "local Ollama host URL")
	flags.StringVar(&values.ollamaModel, "ollama-model", values.ollamaModel, "local Ollama model")
	flags.BoolVar(&values.enableCloudFallback, "cloud-fallback", values.enableCloudFallback, "enable cloud API fallback when local model is unavailable")
	flags.StringVar(&values.cloudProvider, "cloud-provider", values.cloudProvider, "cloud provider for fallback (openai|anthropic|deepseek)")
	flags.StringVar(&values.cloudModel, "cloud-model", values.cloudModel, "cloud model for fallback")
	flags.StringVar(&values.cloudAPIKey, "cloud-api-key", values.cloudAPIKey, "cloud API key (or use QW_CLOUD_API_KEY env)")
}

func buildOptions(mode quantwhisperer.Mode, values runtimeFlags) config.Options {
	return config.Options{
		Mode:                mode,
		Broker:              strings.ToLower(strings.TrimSpace(values.broker)),
		Symbol:              strings.ToUpper(strings.TrimSpace(values.symbol)),
		DBPath:              strings.TrimSpace(values.dbPath),
		StrategyPath:        strings.TrimSpace(values.strategyPath),
		ConfidenceThreshold: values.confidenceThreshold,
		MaxDrawdownPct:      values.maxDrawdownPct,
		MaxTradesPerMinute:  values.maxTradesPerMinute,
		PositionSizePct:     values.positionSizePct,
		StartingCapital:     values.startingCapital,
		TickInterval:        time.Duration(values.tickIntervalMS) * time.Millisecond,
		MaxTicks:            values.maxTicks,
		OllamaURL:           strings.TrimSpace(values.ollamaURL),
		OllamaModel:         strings.TrimSpace(values.ollamaModel),
		EnableCloudFallback: values.enableCloudFallback,
		CloudProvider:       strings.ToLower(strings.TrimSpace(values.cloudProvider)),
		CloudModel:          strings.TrimSpace(values.cloudModel),
		CloudAPIKey:         strings.TrimSpace(values.cloudAPIKey),
	}
}

func runSession(ctx context.Context, mode quantwhisperer.Mode, options config.Options, out io.Writer) error {
	session, err := engine.NewSession(options)
	if err != nil {
		return err
	}
	defer session.Close()

	events := make(chan quantwhisperer.Event, 128)
	done := make(chan error, 1)

	go func() {
		done <- session.Run(ctx, events)
		close(events)
	}()

	fmt.Fprintf(out, "Starting Quant Whisperer in %s mode (%s/%s)\n", mode, options.Broker, options.Symbol)
	for event := range events {
		fmt.Fprintf(out, "%s [%s] %s\n", event.Timestamp.Format("15:04:05"), strings.ToUpper(event.Type), event.Message)
	}

	return <-done
}
