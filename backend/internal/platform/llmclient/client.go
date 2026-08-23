package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/llmconfig"
)

type Config struct {
	OwnerKey    string
	Provider    string
	DisplayName string
	APIKey      string
	BaseURL     string
	Model       string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string                 `json:"model"`
	Messages       []chatMessage          `json:"messages"`
	Temperature    float64                `json:"temperature"`
	MaxTokens      int                    `json:"max_tokens"`
	ResponseFormat map[string]interface{} `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error interface{} `json:"error,omitempty"`
}

type geminiRequest struct {
	Contents          []geminiContent        `json:"contents"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	GenerationConfig  map[string]interface{} `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	Error interface{} `json:"error,omitempty"`
}

func ResolveDefault(ownerKey string) (*Config, error) {
	rows, err := llmconfig.GetProvidersForOwner(ownerKey)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		if row.Enabled && row.IsDefault {
			return configFromRow(ownerKey, row.Provider, row.DisplayName, row.APIKey, row.BaseURL, row.DefaultModel)
		}
	}

	for _, row := range rows {
		if row.Enabled {
			return configFromRow(ownerKey, row.Provider, row.DisplayName, row.APIKey, row.BaseURL, row.DefaultModel)
		}
	}

	return nil, fmt.Errorf("no enabled LLM provider configured")
}

func configFromRow(ownerKey, provider, displayName, apiKey, baseURL, model string) (*Config, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		return nil, fmt.Errorf("LLM provider %s has no API key saved", provider)
	}

	if displayName = strings.TrimSpace(displayName); displayName == "" {
		displayName = provider
	}

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	model = strings.TrimSpace(model)

	switch provider {
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if model == "" {
			model = "gpt-4.1-mini"
		}
	case "openrouter":
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		if model == "" {
			model = "openai/gpt-4.1-mini"
		}
	case "groq":
		if baseURL == "" {
			baseURL = "https://api.groq.com/openai/v1"
		}
		if model == "" {
			model = "llama-3.1-8b-instant"
		}
	case "gemini":
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com/v1beta"
		}
		if model == "" {
			model = "gemini-1.5-flash"
		}
	case "custom":
		if baseURL == "" {
			return nil, fmt.Errorf("custom LLM provider requires base_url")
		}
		if model == "" {
			return nil, fmt.Errorf("custom LLM provider requires default_model")
		}
	default:
		return nil, fmt.Errorf("provider %s is not supported by LLM client v1", provider)
	}

	return &Config{
		OwnerKey:    ownerKey,
		Provider:    provider,
		DisplayName: displayName,
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
	}, nil
}

type JSONRequest struct {
	SystemPrompt    string
	UserPayload     map[string]interface{}
	Temperature     float64
	MaxTokens       int
	ResponseVersion string
}

func GenerateJSON(ctx context.Context, cfg *Config, req JSONRequest) (map[string]interface{}, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM config is required")
	}

	systemPrompt := strings.TrimSpace(req.SystemPrompt)
	if systemPrompt == "" {
		return nil, fmt.Errorf("system prompt is required")
	}

	if req.UserPayload == nil {
		req.UserPayload = map[string]interface{}{}
	}

	if req.MaxTokens <= 0 {
		req.MaxTokens = 1200
	}
	if req.Temperature < 0 {
		req.Temperature = 0
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	userJSON, _ := json.Marshal(req.UserPayload)

	var out map[string]interface{}
	var err error

	if cfg.Provider == "gemini" {
		out, err = generateGeminiJSON(ctx, cfg, systemPrompt, string(userJSON), req.Temperature, req.MaxTokens)
	} else {
		out, err = generateOpenAICompatibleJSON(ctx, cfg, systemPrompt, string(userJSON), req.Temperature, req.MaxTokens)
	}

	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("LLM returned empty JSON object")
	}

	out["llm_provider"] = cfg.Provider
	out["llm_model"] = cfg.Model
	if strings.TrimSpace(req.ResponseVersion) != "" {
		out["response_version"] = strings.TrimSpace(req.ResponseVersion)
	}

	return out, nil
}

func generateOpenAICompatibleJSON(ctx context.Context, cfg *Config, systemPrompt string, userJSON string, temperature float64, maxTokens int) (map[string]interface{}, error) {
	reqBody := chatRequest{
		Model:       cfg.Model,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userJSON},
		},
		ResponseFormat: map[string]interface{}{"type": "json_object"},
	}

	rawReq, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(rawReq))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if cfg.Provider == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://hunt-engine.local")
		req.Header.Set("X-Title", "Hunt Engine")
	}

	client := http.Client{Timeout: 65 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode LLM response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("LLM API returned status %d: %v", resp.StatusCode, parsed.Error)
	}

	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("LLM API returned empty content")
	}

	return decodeJSONMap(parsed.Choices[0].Message.Content)
}

func generateGeminiJSON(ctx context.Context, cfg *Config, systemPrompt string, userJSON string, temperature float64, maxTokens int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf(
		"%s/models/%s:generateContent?key=%s",
		strings.TrimRight(cfg.BaseURL, "/"),
		url.PathEscape(cfg.Model),
		url.QueryEscape(cfg.APIKey),
	)

	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{{
					Text: userJSON + "\n\nReturn valid JSON only. Do not wrap it in markdown.",
				}},
			},
		},
		GenerationConfig: map[string]interface{}{
			"temperature":      temperature,
			"maxOutputTokens":  maxTokens,
			"responseMimeType": "application/json",
		},
	}

	rawReq, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawReq))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 65 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode Gemini response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini API returned status %d: %v", resp.StatusCode, parsed.Error)
	}

	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("Gemini API returned empty content")
	}

	content := strings.TrimSpace(parsed.Candidates[0].Content.Parts[0].Text)
	if content == "" {
		return nil, fmt.Errorf("Gemini API returned empty text")
	}

	return decodeJSONMap(content)
}

func decodeJSONMap(content string) (map[string]interface{}, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("LLM returned non-JSON response: %w", err)
	}
	return out, nil
}

// GenerateTargetNarrative generates a human-readable security narrative for a target.
func GenerateTargetNarrative(ctx context.Context, cfg *Config, snapshot map[string]interface{}, deterministicOutput map[string]interface{}) (map[string]interface{}, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM config is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	systemPrompt := strings.TrimSpace(`
You are a senior application security consultant writing commercial-grade bug bounty / ASM report narrative.

Hard rules:
- The deterministic Hunt Engine risk_score, risk_level, confidence_score, exposure_score, coverage_score, finding_quality_score, risk_drivers, prioritized_findings, and methodology are the source of truth.
- Do not change, reinterpret, upgrade, downgrade, or override any numeric score, severity, finding priority, or risk level.
- Coverage gaps are not vulnerabilities.
- Reconnaissance-only signals are not vulnerabilities unless deterministic evidence says so.
- Write clear customer-facing security language.
- Avoid fear, hype, or inflated severity.
- Return JSON only.
`)

	userPayload := map[string]interface{}{
		"task": "Create narrative fields for this target report without changing deterministic risk outputs.",
		"required_json_schema": map[string]interface{}{
			"executive_summary": "string, concise commercial summary",
			"customer_summary":  "string, less technical summary for stakeholders",
			"remediation_plan":  []string{"ordered, practical remediation or validation steps"},
			"report_notes":      []string{"reporting caveats or useful notes"},
			"validation_notes":  []string{"manual validation checks the operator should perform"},
		},
		"deterministic_output": deterministicOutput,
		"snapshot":             snapshot,
	}

	userJSON, _ := json.Marshal(userPayload)

	var narrative map[string]interface{}
	var err error

	if cfg.Provider == "gemini" {
		narrative, err = generateGeminiNarrative(ctx, cfg, systemPrompt, string(userJSON))
	} else {
		narrative, err = generateOpenAICompatibleNarrative(ctx, cfg, systemPrompt, string(userJSON))
	}

	if err != nil {
		return nil, err
	}

	out := sanitizeNarrative(narrative)
	if len(out) == 0 {
		return nil, fmt.Errorf("LLM narrative did not include supported fields")
	}

	out["llm_assisted"] = true
	out["llm_provider"] = cfg.Provider
	out["llm_model"] = cfg.Model
	out["llm_narrative_version"] = "target-narrative-v1"

	return out, nil
}

func generateOpenAICompatibleNarrative(ctx context.Context, cfg *Config, systemPrompt string, userJSON string) (map[string]interface{}, error) {
	reqBody := chatRequest{
		Model:       cfg.Model,
		Temperature: 0.2,
		MaxTokens:   1200,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userJSON},
		},
		ResponseFormat: map[string]interface{}{"type": "json_object"},
	}

	rawReq, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(rawReq))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if cfg.Provider == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://hunt-engine.local")
		req.Header.Set("X-Title", "Hunt Engine")
	}

	client := http.Client{Timeout: 65 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode LLM response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("LLM API returned status %d: %v", resp.StatusCode, parsed.Error)
	}

	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("LLM API returned empty content")
	}

	return decodeNarrativeJSON(parsed.Choices[0].Message.Content)
}

func generateGeminiNarrative(ctx context.Context, cfg *Config, systemPrompt string, userJSON string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf(
		"%s/models/%s:generateContent?key=%s",
		strings.TrimRight(cfg.BaseURL, "/"),
		url.PathEscape(cfg.Model),
		url.QueryEscape(cfg.APIKey),
	)

	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{{
					Text: userJSON + "\n\nReturn valid JSON only. Do not wrap it in markdown.",
				}},
			},
		},
		GenerationConfig: map[string]interface{}{
			"temperature":      0.2,
			"maxOutputTokens":  1200,
			"responseMimeType": "application/json",
		},
	}

	rawReq, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawReq))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 65 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode Gemini response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini API returned status %d: %v", resp.StatusCode, parsed.Error)
	}

	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("Gemini API returned empty content")
	}

	content := strings.TrimSpace(parsed.Candidates[0].Content.Parts[0].Text)
	if content == "" {
		return nil, fmt.Errorf("Gemini API returned empty text")
	}

	return decodeNarrativeJSON(content)
}

func decodeNarrativeJSON(content string) (map[string]interface{}, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var narrative map[string]interface{}
	if err := json.Unmarshal([]byte(content), &narrative); err != nil {
		return nil, fmt.Errorf("LLM returned non-JSON narrative: %w", err)
	}

	return narrative, nil
}

func sanitizeNarrative(input map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}

	if v := cleanString(input["executive_summary"]); v != "" {
		out["executive_summary"] = v
	}
	if v := cleanString(input["customer_summary"]); v != "" {
		out["customer_summary"] = v
	}
	if v := cleanStringList(input["remediation_plan"]); len(v) > 0 {
		out["remediation_plan"] = v
	}
	if v := cleanStringList(input["report_notes"]); len(v) > 0 {
		out["report_notes"] = v
	}
	if v := cleanStringList(input["validation_notes"]); len(v) > 0 {
		out["validation_notes"] = v
	}

	return out
}

func cleanString(value interface{}) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func cleanStringList(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		text := cleanString(item)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}
