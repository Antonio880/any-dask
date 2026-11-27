package agent

import (
	"context"
	"fmt"
)

// Action represents a canonical Computer Use action independent of LLM provider.
type Action struct {
	Name    string  `json:"name"`
	X       int     `json:"x,omitempty"`
	Y       int     `json:"y,omitempty"`
	EndX    int     `json:"end_x,omitempty"`
	EndY    int     `json:"end_y,omitempty"`
	DeltaX  int     `json:"delta_x,omitempty"`
	DeltaY  int     `json:"delta_y,omitempty"`
	Text    string  `json:"text,omitempty"`
	Keys    []string `json:"keys,omitempty"`
	Command string  `json:"command,omitempty"`
	Seconds float64 `json:"seconds,omitempty"`
	Summary string  `json:"summary,omitempty"`
	Reason  string  `json:"reason,omitempty"`
}

// StepRecord records prior steps in the conversation loop so the LLM has memory of what it already tried.
type StepRecord struct {
	Step    int
	Thought string
	Action  Action
	Result  string
	Success bool
}

// TurnRequest holds all multimodal context for a single Computer Use reasoning step.
type TurnRequest struct {
	UserPrompt    string
	AudioBase64   string // optional: only sent on Step 1 when user spoke via Orb
	AudioMimeType string
	ScreenshotB64 string // base64 JPEG
	ImageWidth    int
	ImageHeight   int
	ScreenWidth   int
	ScreenHeight  int
	OSName        string
	ActiveWindow  string
	History       []StepRecord
	Step          int
	MaxSteps      int
}

// TurnResponse is the normalized output from any LLM provider adapter.
type TurnResponse struct {
	Thought string
	Action  Action
}

// LLMProvider is the Model-Agnostic Strategy interface.
type LLMProvider interface {
	Name() string
	NextAction(ctx context.Context, req TurnRequest) (TurnResponse, error)
}

// SystemPrompt builds a crisp, coordinate-aware Computer Use prompt shared across all providers.
func SystemPrompt(req TurnRequest) string {
	primaryMod := "CTRL"
	if req.OSName == "macos" {
		primaryMod = "CMD"
	}
	return fmt.Sprintf(`You are AnyDask Computer Use Agent, operating the user's real %s machine on demand while they watch live on their phone.
Current display screenshot dimensions: %dx%d pixels (origin 0,0 at top-left).
Active window: %q. Primary modifier key on this OS: %s.

Rules:
1. Inspect the latest screenshot carefully before choosing ONE next action via tool calling.
2. Coordinates (x, y) MUST be pixel coordinates inside the %dx%d screenshot you see. Aim precisely at the center of buttons, inputs, tabs, or icons.
3. Prefer "shell_exec" whenever running terminal/CLI tasks (like git, npm, claude code, opening apps, inspecting files) is faster or more reliable than clicking multiple menus.
4. Keep "reason" short (max 8 words, in Portuguese) so it renders cleanly in the phone's Dynamic Island pill.
5. When the user's goal is achieved (or if blocked and needing human input), call "task_complete" with a concise Portuguese summary.
6. Never repeat the exact same failed click more than twice—try an alternative hotkey, scroll, or shell command.`,
		req.OSName, req.ImageWidth, req.ImageHeight, req.ActiveWindow, primaryMod, req.ImageWidth, req.ImageHeight)
}

// HistorySummary formats previous actions concisely to keep token usage fast and affordable.
func HistorySummary(history []StepRecord) string {
	if len(history) == 0 {
		return "Nenhuma ação executada ainda (este é o passo 1)."
	}
	out := "Ações já executadas nesta tarefa:\n"
	for _, h := range history {
		status := "OK"
		if !h.Success {
			status = "ERRO"
		}
		out += fmt.Sprintf("- Passo %d [%s]: %s (%s) -> %s\n", h.Step, status, h.Action.Name, h.Action.Reason, h.Result)
	}
	return out
}

// CanonicalToolDefs returns the JSON Schema tool definitions shared across Anthropic, Gemini, and OpenAI adapters.
func CanonicalToolDefs() []map[string]any {
	return []map[string]any{
		{
			"name":        "left_click",
			"description": "Move mouse to (x, y) on the screenshot and perform a left click.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x":      map[string]any{"type": "integer", "description": "X pixel coordinate in screenshot"},
					"y":      map[string]any{"type": "integer", "description": "Y pixel coordinate in screenshot"},
					"reason": map[string]any{"type": "string", "description": "Short Portuguese label of what is being clicked"},
				},
				"required": []string{"x", "y", "reason"},
			},
		},
		{
			"name":        "double_click",
			"description": "Double-click at (x, y) on the screenshot.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x":      map[string]any{"type": "integer"},
					"y":      map[string]any{"type": "integer"},
					"reason": map[string]any{"type": "string"},
				},
				"required": []string{"x", "y", "reason"},
			},
		},
		{
			"name":        "right_click",
			"description": "Right-click at (x, y) on the screenshot to open context menu.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x":      map[string]any{"type": "integer"},
					"y":      map[string]any{"type": "integer"},
					"reason": map[string]any{"type": "string"},
				},
				"required": []string{"x", "y", "reason"},
			},
		},
		{
			"name":        "drag",
			"description": "Click and drag from (x, y) to (end_x, end_y).",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x":      map[string]any{"type": "integer"},
					"y":      map[string]any{"type": "integer"},
					"end_x":  map[string]any{"type": "integer"},
					"end_y":  map[string]any{"type": "integer"},
					"reason": map[string]any{"type": "string"},
				},
				"required": []string{"x", "y", "end_x", "end_y", "reason"},
			},
		},
		{
			"name":        "scroll",
			"description": "Scroll at (x, y). Positive delta_y scrolls down, negative scrolls up.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x":       map[string]any{"type": "integer"},
					"y":       map[string]any{"type": "integer"},
					"delta_y": map[string]any{"type": "integer", "description": "Positive=down, Negative=up (e.g. 5 or -5)"},
					"reason":  map[string]any{"type": "string"},
				},
				"required": []string{"x", "y", "delta_y", "reason"},
			},
		},
		{
			"name":        "type_text",
			"description": "Type Unicode text into the currently focused element. Include trailing \\n to press Enter.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text":   map[string]any{"type": "string"},
					"reason": map[string]any{"type": "string"},
				},
				"required": []string{"text", "reason"},
			},
		},
		{
			"name":        "hotkey",
			"description": "Press a keyboard shortcut combo, e.g. [\"PRIMARY\", \"c\"], [\"ALT\", \"Tab\"], [\"Enter\"], [\"Escape\"].",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"keys": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
					"reason": map[string]any{"type": "string"},
				},
				"required": []string{"keys", "reason"},
			},
		},
		{
			"name":        "shell_exec",
			"description": "Run a shell command directly on the host machine (PowerShell on Windows, zsh on macOS) and receive stdout/stderr.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{"type": "string"},
					"reason":  map[string]any{"type": "string"},
				},
				"required": []string{"command", "reason"},
			},
		},
		{
			"name":        "wait",
			"description": "Wait a short duration (1-5 seconds) for an app, build, or page to finish loading.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"seconds": map[string]any{"type": "number"},
					"reason":  map[string]any{"type": "string"},
				},
				"required": []string{"seconds", "reason"},
			},
		},
		{
			"name":        "task_complete",
			"description": "Call this as soon as the user's request is completed or if you need user intervention.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary": map[string]any{"type": "string", "description": "Concise Portuguese summary of what was done"},
				},
				"required": []string{"summary"},
			},
		},
	}
}
