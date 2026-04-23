package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ritiksuman07/quantflow/internal/quantwhisperer"
	"github.com/ritiksuman07/quantflow/internal/quantwhisperer/config"
)

type Client struct {
	httpClient *http.Client
	cfg        config.Options
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
}

func NewClient(cfg config.Options) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 12 * time.Second},
		cfg:        cfg,
	}
}

func (c *Client) Decide(ctx context.Context, input Input) (quantwhisperer.Decision, string) {
	prompt := buildPrompt(input)

	decision, raw, err := c.localOllamaDecision(ctx, prompt)
	if err == nil {
		decision.Raw = raw
		return normalizeDecision(decision, input.Momentum), "ollama"
	}

	if c.cfg.EnableCloudFallback && strings.TrimSpace(c.cfg.CloudAPIKey) != "" {
		cloudDecision, cloudRaw, cloudErr := c.cloudDecision(ctx, prompt)
		if cloudErr == nil {
			cloudDecision.Raw = cloudRaw
			return normalizeDecision(cloudDecision, input.Momentum), c.cfg.CloudProvider
		}
	}

	heuristic := heuristicDecision(input.Momentum)
	heuristic.Raw = `{"source":"heuristic","reason":"ollama/cloud unavailable"}`
	return heuristic, "heuristic"
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
- Confidence must be a number from 0.0 to 1.0.
- Keep reasoning under 20 words.
- Favor HOLD if signal is weak.

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

func (c *Client) localOllamaDecision(ctx context.Context, prompt string) (quantwhisperer.Decision, string, error) {
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
		return quantwhisperer.Decision{}, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return quantwhisperer.Decision{}, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return quantwhisperer.Decision{}, string(body), fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return quantwhisperer.Decision{}, string(body), err
	}

	decision, err := parseDecisionJSON(parsed.Response)
	return decision, parsed.Response, err
}

func (c *Client) cloudDecision(ctx context.Context, prompt string) (quantwhisperer.Decision, string, error) {
	provider := strings.ToLower(strings.TrimSpace(c.cfg.CloudProvider))
	switch provider {
	case "openai", "deepseek":
		return c.chatCompletionsDecision(ctx, provider, prompt)
	case "anthropic":
		return c.anthropicDecision(ctx, prompt)
	default:
		return quantwhisperer.Decision{}, "", fmt.Errorf("unsupported cloud provider: %s", provider)
	}
}

func (c *Client) chatCompletionsDecision(ctx context.Context, provider string, prompt string) (quantwhisperer.Decision, string, error) {
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
		return quantwhisperer.Decision{}, "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.CloudAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return quantwhisperer.Decision{}, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return quantwhisperer.Decision{}, string(body), fmt.Errorf("%s returned %d", provider, resp.StatusCode)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return quantwhisperer.Decision{}, string(body), err
	}
	if len(parsed.Choices) == 0 {
		return quantwhisperer.Decision{}, string(body), fmt.Errorf("no choices from %s", provider)
	}
	raw := parsed.Choices[0].Message.Content
	decision, err := parseDecisionJSON(raw)
	return decision, raw, err
}

func (c *Client) anthropicDecision(ctx context.Context, prompt string) (quantwhisperer.Decision, string, error) {
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
		return quantwhisperer.Decision{}, "", err
	}
	req.Header.Set("x-api-key", c.cfg.CloudAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return quantwhisperer.Decision{}, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return quantwhisperer.Decision{}, string(body), fmt.Errorf("anthropic returned %d", resp.StatusCode)
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return quantwhisperer.Decision{}, string(body), err
	}
	if len(parsed.Content) == 0 {
		return quantwhisperer.Decision{}, string(body), fmt.Errorf("no content from anthropic")
	}
	raw := parsed.Content[0].Text
	decision, err := parseDecisionJSON(raw)
	return decision, raw, err
}

var jsonBlock = regexp.MustCompile(`\{[\s\S]*\}`)

func parseDecisionJSON(raw string) (quantwhisperer.Decision, error) {
	candidate := strings.TrimSpace(raw)
	if !strings.HasPrefix(candidate, "{") {
		matches := jsonBlock.FindString(candidate)
		candidate = strings.TrimSpace(matches)
	}
	if candidate == "" {
		return quantwhisperer.Decision{}, fmt.Errorf("no json object found")
	}

	var decision quantwhisperer.Decision
	if err := json.Unmarshal([]byte(candidate), &decision); err != nil {
		return quantwhisperer.Decision{}, err
	}
	decision.Action = strings.ToUpper(strings.TrimSpace(decision.Action))
	if decision.Action != "BUY" && decision.Action != "SELL" && decision.Action != "HOLD" {
		return quantwhisperer.Decision{}, fmt.Errorf("invalid action: %s", decision.Action)
	}
	if decision.Confidence < 0 || decision.Confidence > 1 {
		return quantwhisperer.Decision{}, fmt.Errorf("invalid confidence: %f", decision.Confidence)
	}
	if strings.TrimSpace(decision.Reasoning) == "" {
		decision.Reasoning = "No reasoning provided"
	}
	return decision, nil
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

func normalizeDecision(decision quantwhisperer.Decision, momentum float64) quantwhisperer.Decision {
	decision.Action = strings.ToUpper(strings.TrimSpace(decision.Action))
	switch decision.Action {
	case "BUY", "SELL", "HOLD":
	default:
		decision = heuristicDecision(momentum)
	}
	if decision.Confidence < 0 || decision.Confidence > 1 {
		decision.Confidence = clamp(math.Abs(momentum)*80 + 0.5)
	}
	if strings.TrimSpace(decision.Reasoning) == "" {
		decision.Reasoning = "Model returned empty reasoning"
	}
	return decision
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
