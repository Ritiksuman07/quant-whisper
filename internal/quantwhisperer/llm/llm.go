package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/config"
)

const maxValidationAttempts = 3

type Client struct {
	httpClient *http.Client
	cfg        config.Options
	logger     *slog.Logger
}

type Input struct {
	Tick      quantwhisperer.Tick
	Momentum  float64
	History   []float64
	Strategy  string
	Mode      quantwhisperer.Mode
	Broker    string
	Symbol    string
	Timestamp time.Time
	TickCount int
}

type rawGenerator func(context.Context, string) (string, error)

func NewClient(cfg config.Options, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil)).With(slog.String("component", "llm"))
	}
	return &Client{
		httpClient: &http.Client{Timeout: 12 * time.Second},
		cfg:        cfg,
		logger:     logger,
	}
}

func (c *Client) Decide(ctx context.Context, input Input) (quantwhisperer.Decision, string) {
	prompt := buildPrompt(input)

	decision, raw, parseFailure, err := c.decideWithValidation(ctx, input, prompt, "ollama", c.localOllamaRaw)
	if err == nil {
		decision.Raw = raw
		return decision, "ollama"
	}
	if parseFailure {
		failure := c.defaultParseFailure(input, raw)
		return failure, "llm_parse_failure"
	}

	if c.cfg.EnableCloudFallback && strings.TrimSpace(c.cfg.CloudAPIKey) != "" {
		provider := strings.ToLower(strings.TrimSpace(c.cfg.CloudProvider))
		cloudDecision, cloudRaw, cloudParseFailure, cloudErr := c.decideWithValidation(ctx, input, prompt, provider, c.cloudRaw)
		if cloudErr == nil {
			cloudDecision.Raw = cloudRaw
			return cloudDecision, provider
		}
		if cloudParseFailure {
			failure := c.defaultParseFailure(input, cloudRaw)
			return failure, "llm_parse_failure"
		}
	}

	heuristic := heuristicDecision(input.Momentum)
	heuristic.Raw = `{"source":"heuristic","reason":"ollama/cloud unavailable"}`
	c.logger.Warn("llm_unavailable_fallback",
		slog.String("symbol", input.Symbol),
		slog.Int("tick_count", input.TickCount),
		slog.Group("decision",
			slog.String("action", heuristic.Action),
			slog.Float64("confidence", heuristic.Confidence),
		),
		slog.String("error", err.Error()),
	)
	return heuristic, "heuristic"
}

func (c *Client) decideWithValidation(ctx context.Context, input Input, basePrompt string, provider string, generator rawGenerator) (quantwhisperer.Decision, string, bool, error) {
	prompt := basePrompt
	lastRaw := ""

	for attempt := 1; attempt <= maxValidationAttempts; attempt++ {
		raw, err := generator(ctx, prompt)
		if err != nil {
			c.logger.Warn("llm_provider_error",
				slog.String("symbol", input.Symbol),
				slog.Int("tick_count", input.TickCount),
				slog.String("provider", provider),
				slog.String("error", err.Error()),
			)
			return quantwhisperer.Decision{}, lastRaw, false, err
		}
		lastRaw = raw

		decision, validationErr := parseDecisionJSONStrict(raw)
		if validationErr == nil {
			c.logger.Info("llm_decision_valid",
				slog.String("symbol", input.Symbol),
				slog.Int("tick_count", input.TickCount),
				slog.String("provider", provider),
				slog.Group("decision",
					slog.String("action", decision.Action),
					slog.Float64("confidence", decision.Confidence),
				),
			)
			return decision, raw, false, nil
		}

		c.logger.Warn("llm_decision_invalid",
			slog.String("symbol", input.Symbol),
			slog.Int("tick_count", input.TickCount),
			slog.String("provider", provider),
			slog.Int("attempt", attempt),
			slog.String("error", validationErr.Error()),
		)

		if attempt == maxValidationAttempts {
			return quantwhisperer.Decision{}, raw, true, validationErr
		}
		prompt = buildRetryPrompt(basePrompt, validationErr, raw)
	}

	return quantwhisperer.Decision{}, lastRaw, true, fmt.Errorf("validation retries exhausted")
}

func (c *Client) defaultParseFailure(input Input, raw string) quantwhisperer.Decision {
	decision := quantwhisperer.Decision{
		Action:     "HOLD",
		Confidence: 0.0,
		Reasoning:  "llm_parse_failure",
		Raw:        raw,
	}
	c.logger.Error("llm_parse_failure",
		slog.String("symbol", input.Symbol),
		slog.Int("tick_count", input.TickCount),
		slog.Group("decision",
			slog.String("action", decision.Action),
			slog.Float64("confidence", decision.Confidence),
		),
	)
	return decision
}

func buildPrompt(input Input) string {
	history := make([]string, 0, len(input.History))
	for _, point := range input.History {
		history = append(history, fmt.Sprintf("%.4f", point))
	}

	return strings.TrimSpace(fmt.Sprintf(`
You are Quant Whisperer, an execution policy model.
Return exactly one JSON object and no markdown.
Allowed schema:
{"action":"BUY|SELL|HOLD","confidence":0.0,"reasoning":"brief string"}

Constraints:
- confidence must be a number from 0.0 to 1.0.
- reasoning must be non-empty and <= 500 characters.
- no extra JSON fields are allowed.

Runtime Context:
- mode: %s
- broker: %s
- symbol: %s
- timestamp_utc: %s
- last_price: %.4f
- bid: %.4f
- ask: %.4f
- volume: %d
- momentum: %.6f
- recent_prices: [%s]
- strategy_text: %s
`, input.Mode, input.Broker, input.Symbol, input.Timestamp.UTC().Format(time.RFC3339), input.Tick.LastPrice, input.Tick.Bid, input.Tick.Ask, input.Tick.Volume, input.Momentum, strings.Join(history, ", "), input.Strategy))
}

func buildRetryPrompt(basePrompt string, validationErr error, raw string) string {
	trimmedRaw := strings.TrimSpace(raw)
	if len(trimmedRaw) > 1200 {
		trimmedRaw = trimmedRaw[:1200]
	}
	return fmt.Sprintf(`%s

Your previous response was invalid: %s
Previous response:
%s

Return again with EXACTLY one JSON object containing only keys: action, confidence, reasoning.`, basePrompt, validationErr.Error(), trimmedRaw)
}

func (c *Client) localOllamaRaw(ctx context.Context, prompt string) (string, error) {
	type request struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
		Format string `json:"format,omitempty"`
	}
	type response struct {
		Response string `json:"response"`
	}

	payload, _ := json.Marshal(request{
		Model:  c.cfg.OllamaModel,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	})

	endpoint := strings.TrimRight(c.cfg.OllamaURL, "/") + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return string(body), fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return string(body), err
	}
	return parsed.Response, nil
}

func (c *Client) cloudRaw(ctx context.Context, prompt string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(c.cfg.CloudProvider))
	switch provider {
	case "openai", "deepseek":
		return c.chatCompletionsRaw(ctx, provider, prompt)
	case "anthropic":
		return c.anthropicRaw(ctx, prompt)
	default:
		return "", fmt.Errorf("unsupported cloud provider: %s", provider)
	}
}

func (c *Client) chatCompletionsRaw(ctx context.Context, provider string, prompt string) (string, error) {
	base := "https://api.openai.com"
	if provider == "deepseek" {
		base = "https://api.deepseek.com"
	}
	url := base + "/v1/chat/completions"

	payload := map[string]any{
		"model": c.cfg.CloudModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.CloudAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return string(body), fmt.Errorf("%s returned %d", provider, resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return string(body), err
	}
	if len(parsed.Choices) == 0 {
		return string(body), fmt.Errorf("no choices from %s", provider)
	}
	return parsed.Choices[0].Message.Content, nil
}

func (c *Client) anthropicRaw(ctx context.Context, prompt string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"
	payload := map[string]any{
		"model":      c.cfg.CloudModel,
		"max_tokens": 200,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", c.cfg.CloudAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return string(body), fmt.Errorf("anthropic returned %d", resp.StatusCode)
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return string(body), err
	}
	if len(parsed.Content) == 0 {
		return string(body), fmt.Errorf("no content from anthropic")
	}
	return parsed.Content[0].Text, nil
}

var jsonBlock = regexp.MustCompile(`\{[\s\S]*\}`)

func parseDecisionJSONStrict(raw string) (quantwhisperer.Decision, error) {
	candidate := strings.TrimSpace(raw)
	if !strings.HasPrefix(candidate, "{") {
		candidate = strings.TrimSpace(jsonBlock.FindString(candidate))
	}
	if candidate == "" {
		return quantwhisperer.Decision{}, fmt.Errorf("no json object found")
	}

	decoder := json.NewDecoder(strings.NewReader(candidate))
	decoder.UseNumber()
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return quantwhisperer.Decision{}, fmt.Errorf("invalid json: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return quantwhisperer.Decision{}, fmt.Errorf("unexpected trailing json")
	}

	allowed := map[string]struct{}{
		"action":     {},
		"confidence": {},
		"reasoning":  {},
	}
	if len(fields) != len(allowed) {
		return quantwhisperer.Decision{}, fmt.Errorf("expected exactly 3 fields")
	}
	for key := range fields {
		if _, ok := allowed[key]; !ok {
			return quantwhisperer.Decision{}, fmt.Errorf("extra field: %s", key)
		}
	}
	for key := range allowed {
		if _, ok := fields[key]; !ok {
			return quantwhisperer.Decision{}, fmt.Errorf("missing required field: %s", key)
		}
	}

	var action string
	if err := json.Unmarshal(fields["action"], &action); err != nil {
		return quantwhisperer.Decision{}, fmt.Errorf("invalid action field")
	}
	if action != "BUY" && action != "SELL" && action != "HOLD" {
		return quantwhisperer.Decision{}, fmt.Errorf("action must be BUY, SELL, or HOLD")
	}

	var confidence float64
	if err := json.Unmarshal(fields["confidence"], &confidence); err != nil {
		return quantwhisperer.Decision{}, fmt.Errorf("invalid confidence field")
	}
	if confidence < 0.0 || confidence > 1.0 {
		return quantwhisperer.Decision{}, fmt.Errorf("confidence outside [0.0, 1.0]")
	}

	var reasoning string
	if err := json.Unmarshal(fields["reasoning"], &reasoning); err != nil {
		return quantwhisperer.Decision{}, fmt.Errorf("invalid reasoning field")
	}
	reasoning = strings.TrimSpace(reasoning)
	if reasoning == "" {
		return quantwhisperer.Decision{}, fmt.Errorf("reasoning is empty")
	}
	if len(reasoning) > 500 {
		return quantwhisperer.Decision{}, fmt.Errorf("reasoning exceeds 500 characters")
	}

	return quantwhisperer.Decision{
		Action:     action,
		Confidence: confidence,
		Reasoning:  reasoning,
	}, nil
}

func heuristicDecision(momentum float64) quantwhisperer.Decision {
	score := math.Min(1, math.Abs(momentum)*120)
	switch {
	case momentum > 0.0025:
		return quantwhisperer.Decision{Action: "BUY", Confidence: clamp(0.55 + score), Reasoning: "Positive momentum continuation"}
	case momentum < -0.0025:
		return quantwhisperer.Decision{Action: "SELL", Confidence: clamp(0.55 + score), Reasoning: "Negative momentum continuation"}
	default:
		return quantwhisperer.Decision{Action: "HOLD", Confidence: 0.62, Reasoning: "No clear directional edge"}
	}
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
