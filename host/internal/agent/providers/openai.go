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

// OpenAICompatProvider supports OpenAI, OpenRouter, Groq, Together, Ollama, and any OpenAI-compatible API.
type OpenAICompatProvider struct {
	APIKey  string
	Model   string
	BaseURL string
	client  *http.Client
}

func NewOpenAICompat(apiKey, model, baseURL string) *OpenAICompatProvider {
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o"
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAICompatProvider{
		APIKey:  strings.TrimSpace(apiKey),
		Model:   strings.TrimSpace(model),
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

func (p *OpenAICompatProvider) Name() string { return "openai" }

func (p *OpenAICompatProvider) NextAction(ctx context.Context, req agent.TurnRequest) (agent.TurnResponse, error) {
	defs := agent.CanonicalToolDefs()
	tools := make([]map[string]any, 0, len(defs))
	for _, d := range defs {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        d["name"],
				"description": d["description"],
				"parameters":  d["parameters"],
			},
		})
	}

	userText := fmt.Sprintf("Objetivo do usuário: %s\n\n%s\n\nPasso atual: %d/%d. Analise a imagem da tela e chame EXATAMENTE UMA função.",
		req.UserPrompt, agent.HistorySummary(req.History), req.Step, req.MaxSteps)

	body := map[string]any{
		"model":       p.Model,
		"max_tokens":  1024,
		"tools":       tools,
		"tool_choice": "required",
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": agent.SystemPrompt(req),
			},
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "image_url",
						"image_url": map[string]any{
							"url":    "data:image/jpeg;base64," + req.ScreenshotB64,
							"detail": "high",
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(rawBody))
	if err != nil {
		return agent.TurnResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("HTTP-Referer", "https://anydask.local")
	httpReq.Header.Set("X-Title", "AnyDask Computer Use")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return agent.TurnResponse{}, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return agent.TurnResponse{}, fmt.Errorf("openai-compat HTTP %d: %s", resp.StatusCode, trimErr(respBytes))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return agent.TurnResponse{}, err
	}
	if len(parsed.Choices) == 0 {
		return agent.TurnResponse{}, fmt.Errorf("openai-compat returned no choices")
	}

	msg := parsed.Choices[0].Message
	thought := strings.TrimSpace(msg.Content)
	if len(msg.ToolCalls) > 0 {
		tc := msg.ToolCalls[0]
		var act agent.Action
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &act); err != nil {
			return agent.TurnResponse{}, fmt.Errorf("openai-compat tool args parse error: %w", err)
		}
		act.Name = tc.Function.Name
		return agent.TurnResponse{Thought: thought, Action: act}, nil
	}

	if thought != "" {
		return agent.TurnResponse{
			Thought: thought,
			Action:  agent.Action{Name: "task_complete", Summary: thought},
		}, nil
	}
	return agent.TurnResponse{}, fmt.Errorf("openai-compat returned neither tool call nor text")
}
