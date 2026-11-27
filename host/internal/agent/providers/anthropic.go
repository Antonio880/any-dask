package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anydask/host/internal/agent"
)

type AnthropicProvider struct {
	APIKey  string
	Model   string
	BaseURL string
	client  *http.Client
}

func NewAnthropic(apiKey, model, baseURL string) *AnthropicProvider {
	if strings.TrimSpace(model) == "" {
		model = "claude-sonnet-4-5-20250929"
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicProvider{
		APIKey:  strings.TrimSpace(apiKey),
		Model:   strings.TrimSpace(model),
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) NextAction(ctx context.Context, req agent.TurnRequest) (agent.TurnResponse, error) {
	defs := agent.CanonicalToolDefs()
	tools := make([]map[string]any, 0, len(defs))
	for _, d := range defs {
		tools = append(tools, map[string]any{
			"name":         d["name"],
			"description":  d["description"],
			"input_schema": d["parameters"],
		})
	}

	userText := fmt.Sprintf("Objetivo do usuário: %s\n\n%s\n\nPasso atual: %d/%d. Observe o screenshot anexado e chame EXATAMENTE UMA ferramenta para o próximo passo.",
		req.UserPrompt, agent.HistorySummary(req.History), req.Step, req.MaxSteps)

	body := map[string]any{
		"model":      p.Model,
		"max_tokens": 1024,
		"system":     agent.SystemPrompt(req),
		"tools":      tools,
		"tool_choice": map[string]any{
			"type": "any",
		},
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/jpeg",
							"data":       req.ScreenshotB64,
						},
					},
					{
						"type": "text",
						"text": userText,
					},
				},
			},
		},
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return agent.TurnResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/v1/messages", bytes.NewReader(rawBody))
	if err != nil {
		return agent.TurnResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return agent.TurnResponse{}, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return agent.TurnResponse{}, fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, trimErr(respBytes))
	}

	var parsed struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return agent.TurnResponse{}, err
	}

	var thought string
	for _, block := range parsed.Content {
		if block.Type == "text" {
			thought = strings.TrimSpace(thought + " " + block.Text)
		}
		if block.Type == "tool_use" {
			var act agent.Action
			if err := json.Unmarshal(block.Input, &act); err != nil {
				return agent.TurnResponse{}, fmt.Errorf("failed parsing tool input %s: %w", block.Name, err)
			}
			act.Name = block.Name
			return agent.TurnResponse{Thought: thought, Action: act}, nil
		}
	}

	if thought != "" {
		return agent.TurnResponse{
			Thought: thought,
			Action:  agent.Action{Name: "task_complete", Summary: thought},
		}, nil
	}
	return agent.TurnResponse{}, fmt.Errorf("anthropic returned no tool call")
}

func trimErr(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}
