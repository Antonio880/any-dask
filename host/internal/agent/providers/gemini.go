package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anydask/host/internal/agent"
)

type GeminiProvider struct {
	APIKey  string
	Model   string
	BaseURL string
	client  *http.Client
}

func NewGemini(apiKey, model, baseURL string) *GeminiProvider {
	if strings.TrimSpace(model) == "" {
		model = "gemini-3-flash-preview"
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return &GeminiProvider{
		APIKey:  strings.TrimSpace(apiKey),
		Model:   strings.TrimSpace(model),
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) NextAction(ctx context.Context, req agent.TurnRequest) (agent.TurnResponse, error) {
	defs := agent.CanonicalToolDefs()
	funcDecls := make([]map[string]any, 0, len(defs))
	for _, d := range defs {
		funcDecls = append(funcDecls, map[string]any{
			"name":        d["name"],
			"description": d["description"],
			"parameters":  d["parameters"],
		})
	}

	parts := []map[string]any{
		{
			"inline_data": map[string]any{
				"mime_type": "image/jpeg",
				"data":      req.ScreenshotB64,
			},
		},
	}

	// Native multimodal audio support: if the user spoke into the Orb, send raw audio on Step 1!
	if req.AudioBase64 != "" && req.Step == 1 {
		mime := req.AudioMimeType
		if mime == "" {
			mime = "audio/webm"
		}
		parts = append(parts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": mime,
				"data":      req.AudioBase64,
			},
		})
	}

	promptText := req.UserPrompt
	if strings.TrimSpace(promptText) == "" && req.AudioBase64 != "" {
		promptText = "(Siga a instrução falada no áudio anexado.)"
	}

	userText := fmt.Sprintf("Objetivo do usuário: %s\n\n%s\n\nPasso atual: %d/%d. Observe o screenshot e chame EXATAMENTE UMA função para executar a próxima ação.",
		promptText, agent.HistorySummary(req.History), req.Step, req.MaxSteps)
	parts = append(parts, map[string]any{"text": userText})

	body := map[string]any{
		"system_instruction": map[string]any{
			"parts": []map[string]any{{"text": agent.SystemPrompt(req)}},
		},
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": parts,
			},
		},
		"tools": []map[string]any{
			{"function_declarations": funcDecls},
		},
		"tool_config": map[string]any{
			"function_calling_config": map[string]any{
				"mode": "ANY",
			},
		},
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return agent.TurnResponse{}, err
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		p.BaseURL, url.PathEscape(p.Model), url.QueryEscape(p.APIKey))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return agent.TurnResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return agent.TurnResponse{}, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return agent.TurnResponse{}, fmt.Errorf("gemini HTTP %d: %s", resp.StatusCode, trimErr(respBytes))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return agent.TurnResponse{}, err
	}
	if len(parsed.Candidates) == 0 {
		return agent.TurnResponse{}, fmt.Errorf("gemini returned empty candidates")
	}

	var thought string
	for _, part := range parsed.Candidates[0].Content.Parts {
		if part.Text != "" {
			thought = strings.TrimSpace(thought + " " + part.Text)
		}
		if part.FunctionCall != nil {
			var act agent.Action
			if len(part.FunctionCall.Args) > 0 {
				if err := json.Unmarshal(part.FunctionCall.Args, &act); err != nil {
					return agent.TurnResponse{}, fmt.Errorf("gemini tool args error: %w", err)
				}
			}
			act.Name = part.FunctionCall.Name
			return agent.TurnResponse{Thought: thought, Action: act}, nil
		}
	}

	if thought != "" {
		return agent.TurnResponse{
			Thought: thought,
			Action:  agent.Action{Name: "task_complete", Summary: thought},
		}, nil
	}
	return agent.TurnResponse{}, fmt.Errorf("gemini returned no function call")
}
