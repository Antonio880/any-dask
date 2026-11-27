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

var geminiFallbackModels = []string{
	"gemini-3.1-flash-lite",
	"gemini-flash-lite-latest",
	"gemini-3.1-flash-lite-preview",
	"gemini-3-flash-preview",
}

func NewGemini(apiKey, model, baseURL string) *GeminiProvider {
	m := strings.TrimSpace(model)
	// Auto-upgrade legacy/slow default models to fast verified 1.4s model
	if m == "" || m == "gemini-2.5-flash" || m == "gemini-2.0-flash" {
		m = "gemini-3.1-flash-lite"
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return &GeminiProvider{
		APIKey:  strings.TrimSpace(apiKey),
		Model:   m,
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		// Crisp 18s per-attempt timeout so a congested model triggers instant fallback instead of hanging 2 minutes!
		client: &http.Client{Timeout: 18 * time.Second},
	}
}

func (p *GeminiProvider) Name() string { return "gemini (" + p.Model + ")" }

func (p *GeminiProvider) NextAction(ctx context.Context, req agent.TurnRequest) (agent.TurnResponse, error) {
	candidates := []string{p.Model}
	for _, fb := range geminiFallbackModels {
		if fb != p.Model {
			candidates = append(candidates, fb)
		}
	}

	var lastErr error
	for _, modelID := range candidates {
		if ctx.Err() != nil {
			return agent.TurnResponse{}, ctx.Err()
		}
		resp, retryable, err := p.callModel(ctx, modelID, req)
		if err == nil {
			p.Model = modelID // stick to the fastest healthy model for remaining steps in this task!
			return resp, nil
		}
		lastErr = err
		if !retryable {
			return agent.TurnResponse{}, err
		}
	}
	return agent.TurnResponse{}, lastErr
}

func (p *GeminiProvider) callModel(ctx context.Context, modelID string, req agent.TurnRequest) (agent.TurnResponse, bool, error) {
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

	// Native multimodal audio support: send raw voice clip from Floating Bot Hold-to-Talk!
	if req.AudioBase64 != "" && req.Step == 1 {
		mime := req.AudioMimeType
		if mime == "" {
			mime = "audio/mp4"
		}
		parts = append(parts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": mime,
				"data":      req.AudioBase64,
			},
		})
	}

	promptText := strings.TrimSpace(req.UserPrompt)
	if promptText == "" && req.AudioBase64 != "" {
		promptText = "(Ouça o áudio anexado em português e execute exatamente o que o usuário pediu na tela.)"
	}

	userText := fmt.Sprintf("Objetivo do usuário: %s\n\n%s\n\nPasso atual: %d/%d. Analise o screenshot e chame EXATAMENTE UMA função para agir rápido.",
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
		"generationConfig": map[string]any{
			"temperature":     0.2,
			"maxOutputTokens": 512,
		},
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return agent.TurnResponse{}, false, err
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		p.BaseURL, url.PathEscape(modelID), url.QueryEscape(p.APIKey))

	reqCtx, cancel := context.WithTimeout(ctx, 16*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return agent.TurnResponse{}, false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		// Timeout or transient network error -> retryable on fallback model (unless parent ctx was canceled by user STOP)
		if ctx.Err() != nil {
			return agent.TurnResponse{}, false, ctx.Err()
		}
		return agent.TurnResponse{}, true, fmt.Errorf("gemini (%s) timeout/rede: %w", modelID, err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		retryable := resp.StatusCode == 404 || resp.StatusCode == 429 || resp.StatusCode >= 500
		return agent.TurnResponse{}, retryable, fmt.Errorf("gemini (%s) HTTP %d: %s", modelID, resp.StatusCode, trimErr(respBytes))
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
		return agent.TurnResponse{}, false, err
	}
	if len(parsed.Candidates) == 0 {
		return agent.TurnResponse{}, true, fmt.Errorf("gemini (%s) retornou resposta vazia", modelID)
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
					return agent.TurnResponse{}, false, fmt.Errorf("gemini tool args error: %w", err)
				}
			}
			act.Name = part.FunctionCall.Name
			return agent.TurnResponse{Thought: thought, Action: act}, false, nil
		}
	}

	if thought != "" {
		return agent.TurnResponse{
			Thought: thought,
			Action:  agent.Action{Name: "task_complete", Summary: thought},
		}, false, nil
	}
	return agent.TurnResponse{}, false, fmt.Errorf("gemini (%s) não retornou ação", modelID)
}
