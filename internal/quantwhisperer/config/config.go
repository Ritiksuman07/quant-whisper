package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
)

type Options struct {
	Mode                quantwhisperer.Mode
	Broker              string
	Symbol              string
	DBPath              string
	StrategyPath        string
	ConfidenceThreshold float64
	MaxDrawdownPct      float64
	MaxTradesPerMinute  int
	PositionSizePct     float64
	StartingCapital     float64
	TickInterval        time.Duration
	MaxTicks            int
	OllamaURL           string
	OllamaModel         string
	EnableCloudFallback bool
	CloudProvider       string
	CloudModel          string
	CloudAPIKey         string
}

func DefaultOptions(mode quantwhisperer.Mode) Options {
	return Options{
		Mode:                mode,
		Broker:              envOrDefault("QW_BROKER", "zerodha"),
		Symbol:              strings.ToUpper(envOrDefault("QW_SYMBOL", "NIFTY50")),
		DBPath:              envOrDefault("QW_DB_PATH", "quant_whisperer.db"),
		StrategyPath:        envOrDefault("QW_STRATEGY_PATH", ""),
		ConfidenceThreshold: envFloatOrDefault("QW_CONFIDENCE_THRESHOLD", 0.85),
		MaxDrawdownPct:      envFloatOrDefault("QW_MAX_DRAWDOWN_PCT", 3.0),
		MaxTradesPerMinute:  envIntOrDefault("QW_MAX_TRADES_PER_MIN", 3),
		PositionSizePct:     envFloatOrDefault("QW_POSITION_SIZE_PCT", 10.0),
		StartingCapital:     envFloatOrDefault("QW_STARTING_CAPITAL", 100000),
		TickInterval:        time.Duration(envIntOrDefault("QW_TICK_INTERVAL_MS", 1000)) * time.Millisecond,
		MaxTicks:            envIntOrDefault("QW_MAX_TICKS", 120),
		OllamaURL:           envOrDefault("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:         envOrDefault("QW_OLLAMA_MODEL", "qwen3:0.8b"),
		EnableCloudFallback: strings.EqualFold(envOrDefault("QW_ENABLE_CLOUD_FALLBACK", "false"), "true"),
		CloudProvider:       strings.ToLower(envOrDefault("QW_CLOUD_PROVIDER", "openai")),
		CloudModel:          envOrDefault("QW_CLOUD_MODEL", "gpt-4o-mini"),
		CloudAPIKey:         envOrDefault("QW_CLOUD_API_KEY", ""),
	}
}

func (o Options) Validate() error {
	if o.Broker == "" {
		return fmt.Errorf("broker is required")
	}
	if o.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if o.ConfidenceThreshold < 0 || o.ConfidenceThreshold > 1 {
		return fmt.Errorf("confidence threshold must be between 0 and 1")
	}
	if o.MaxDrawdownPct <= 0 {
		return fmt.Errorf("max drawdown pct must be > 0")
	}
	if o.MaxTradesPerMinute <= 0 {
		return fmt.Errorf("max trades per minute must be > 0")
	}
	if o.PositionSizePct <= 0 || o.PositionSizePct > 100 {
		return fmt.Errorf("position size pct must be in (0, 100]")
	}
	if o.StartingCapital <= 0 {
		return fmt.Errorf("starting capital must be > 0")
	}
	if o.TickInterval < 100*time.Millisecond {
		return fmt.Errorf("tick interval too low")
	}
	if o.MaxTicks <= 0 {
		return fmt.Errorf("max ticks must be > 0")
	}
	return nil
}

func (o Options) Credentials() quantwhisperer.Credentials {
	prefix := strings.ToUpper(strings.ReplaceAll(o.Broker, "-", "_"))
	if prefix == "INTERACTIVE_BROKERS" {
		prefix = "IB"
	}
	return quantwhisperer.Credentials{
		APIKey:      os.Getenv(prefix + "_API_KEY"),
		APISecret:   os.Getenv(prefix + "_API_SECRET"),
		AccessToken: os.Getenv(prefix + "_ACCESS_TOKEN"),
	}
}

func (o Options) StrategyText() string {
	if strings.TrimSpace(o.StrategyPath) == "" {
		return "Default strategy: momentum and mean-reversion blend."
	}
	bytes, err := os.ReadFile(o.StrategyPath)
	if err != nil {
		return fmt.Sprintf("Strategy file unavailable (%s): fallback to baseline momentum strategy.", err.Error())
	}
	content := strings.TrimSpace(string(bytes))
	if content == "" {
		return "Strategy file is empty; fallback to baseline momentum strategy."
	}
	return content
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envFloatOrDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	out, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return out
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var out int
	_, err := fmt.Sscanf(value, "%d", &out)
	if err != nil {
		return fallback
	}
	return out
}
