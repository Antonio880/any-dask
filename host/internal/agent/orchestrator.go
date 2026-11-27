package agent

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/anydask/host/internal/capture"
	"github.com/anydask/host/internal/input"
	"github.com/anydask/host/internal/osctx"
	"github.com/anydask/host/internal/overlay"
	"github.com/anydask/host/internal/protocol"
)

type EventSink func(msgType string, payload any)

type ProviderFactory func(req protocol.AIStartTaskPayload) (LLMProvider, error)

// Orchestrator executes the on-demand Computer Use Vision -> Plan -> Act -> Verify loop.
type Orchestrator struct {
	arbiter *input.Arbiter
	factory ProviderFactory
	overlay overlay.Overlay

	mu          sync.Mutex
	confirmWait map[string]chan bool
}

func NewOrchestrator(arbiter *input.Arbiter, factory ProviderFactory, ov overlay.Overlay) *Orchestrator {
	return &Orchestrator{
		arbiter:     arbiter,
		factory:     factory,
		overlay:     ov,
		confirmWait: make(map[string]chan bool),
	}
}

func (o *Orchestrator) HandleConfirmResponse(resp protocol.AIConfirmResponsePayload) {
	o.mu.Lock()
	ch, ok := o.confirmWait[resp.ConfirmID]
	if ok {
		delete(o.confirmWait, resp.ConfirmID)
	}
	o.mu.Unlock()
	if ok {
		select {
		case ch <- resp.Approved:
		default:
		}
	}
}

// Abort immediately cancels any running AI task and releases input back to MANUAL mode (<10ms).
func (o *Orchestrator) Abort(sink EventSink) {
	wasActive := o.arbiter.AbortAI()
	// Unblock any pending confirmation waiters
	o.mu.Lock()
	for id, ch := range o.confirmWait {
		select {
		case ch <- false:
		default:
		}
		delete(o.confirmWait, id)
	}
	o.mu.Unlock()

	if wasActive {
		if o.overlay != nil {
			o.overlay.SetState(overlay.StateConnected, "AnyDask · Conectado")
		}
		if sink != nil {
			sink("ai.status", protocol.AIStatusPayload{
				State:   "ABORTED",
				Message: "Controle retomado pelo usuário",
			})
		}
	}
}

// StartTask launches a Computer Use session asynchronously.
func (o *Orchestrator) StartTask(req protocol.AIStartTaskPayload, sink EventSink) error {
	if strings.TrimSpace(req.APIKey) == "" {
		return errors.New("API Key não configurada. Abra Configurações para adicionar sua chave")
	}
	if strings.TrimSpace(req.Prompt) == "" && strings.TrimSpace(req.AudioBase64) == "" {
		return errors.New("informe uma instrução em texto ou áudio")
	}

	provider, err := o.factory(req)
	if err != nil {
		return err
	}

	taskID := randomID()
	ctx, cancel := context.WithCancel(context.Background())

	if err := o.arbiter.AcquireAI(taskID, cancel); err != nil {
		cancel()
		return err
	}

	maxSteps := req.MaxSteps
	if maxSteps <= 0 || maxSteps > 40 {
		maxSteps = 18
	}

	go o.runLoop(ctx, cancel, taskID, provider, req, maxSteps, sink)
	return nil
}

func (o *Orchestrator) runLoop(
	ctx context.Context,
	cancel context.CancelFunc,
	taskID string,
	provider LLMProvider,
	req protocol.AIStartTaskPayload,
	maxSteps int,
	sink EventSink,
) {
	defer cancel()
	defer o.arbiter.ReleaseAI(taskID)
	defer func() {
		if o.overlay != nil {
			o.overlay.SetState(overlay.StateConnected, "AnyDask · Conectado")
		}
	}()

	var history []StepRecord

	for step := 1; step <= maxSteps; step++ {
		if ctx.Err() != nil {
			return
		}

		if o.overlay != nil {
			o.overlay.SetState(overlay.StateAIThinking, fmt.Sprintf("AnyDask AI · Passo %d", step))
		}
		sink("ai.status", protocol.AIStatusPayload{
			TaskID:  taskID,
			State:   "THINKING",
			Message: "📸 Capturando tela...",
			Step:    step,
			MaxStep: maxSteps,
		})

		// 1. Capture crisp screenshot (1024px wide -> ~50KB for ultra-fast <1.5s LLM roundtrip)
		b := o.arbiter.ScreenBounds()
		snap, err := capture.CaptureSnapshotForDisplay(1024, o.arbiter.ActiveDisplay(), b.X, b.Y, b.Width, b.Height)
		if err != nil {
			sink("ai.status", protocol.AIStatusPayload{
				TaskID:  taskID,
				State:   "ERROR",
				Message: "Falha ao capturar tela: " + err.Error(),
			})
			return
		}
		sysInfo := osctx.Collect()

		sink("ai.status", protocol.AIStatusPayload{
			TaskID:  taskID,
			State:   "THINKING",
			Message: fmt.Sprintf("🧠 Pensando (%s)...", provider.Name()),
			Step:    step,
			MaxStep: maxSteps,
		})

		turnReq := TurnRequest{
			UserPrompt:    req.Prompt,
			AudioBase64:   req.AudioBase64,
			AudioMimeType: req.AudioMimeType,
			ScreenshotB64: base64.StdEncoding.EncodeToString(snap.JPEGBytes),
			ImageWidth:    snap.Width,
			ImageHeight:   snap.Height,
			ScreenWidth:   snap.ScreenW,
			ScreenHeight:  snap.ScreenH,
			OSName:        sysInfo.OS,
			ActiveWindow:  sysInfo.ActiveWindow,
			History:       history,
			Step:          step,
			MaxSteps:      maxSteps,
		}

		// 2. Call active LLM adapter
		callStart := time.Now()
		resp, err := provider.NextAction(ctx, turnReq)
		callMs := time.Since(callStart).Milliseconds()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[agent] provider error step %d (%dms): %v", step, callMs, err)
			sink("ai.status", protocol.AIStatusPayload{
				TaskID:  taskID,
				State:   "ERROR",
				Message: err.Error(),
				Step:    step,
				MaxStep: maxSteps,
			})
			return
		}
		log.Printf("[agent] step %d LLM responded in %dms -> %s (%s)", step, callMs, resp.Action.Name, resp.Action.Reason)

		act := resp.Action
		if act.Name == "task_complete" {
			summary := strings.TrimSpace(act.Summary)
			if summary == "" {
				summary = "Tarefa concluída."
			}
			sink("ai.status", protocol.AIStatusPayload{
				TaskID:  taskID,
				State:   "COMPLETED",
				Message: summary,
				Step:    step,
				MaxStep: maxSteps,
			})
			return
		}

		// 3. Guardrail check for sensitive / destructive actions
		if req.RequireConfirm {
			if risky, reason := CheckGuardrail(act); risky {
				confirmID := randomID()
				ch := make(chan bool, 1)
				o.mu.Lock()
				o.confirmWait[confirmID] = ch
				o.mu.Unlock()

				o.arbiter.SetWaitingConfirm(taskID, true)
				sink("ai.status", protocol.AIStatusPayload{
					TaskID:  taskID,
					State:   "CONFIRM",
					Message: "Aguardando sua aprovação...",
					Step:    step,
					MaxStep: maxSteps,
				})
				sink("ai.confirm_required", protocol.AIConfirmRequiredPayload{
					TaskID:    taskID,
					ConfirmID: confirmID,
					Action:    act.Name,
					Command:   act.Command,
					Reason:    reason,
				})

				var approved bool
				select {
				case approved = <-ch:
				case <-ctx.Done():
					return
				case <-time.After(90 * time.Second):
					approved = false
				}
				o.arbiter.SetWaitingConfirm(taskID, false)

				if !approved {
					history = append(history, StepRecord{
						Step:    step,
						Thought: resp.Thought,
						Action:  act,
						Result:  "Usuário recusou a execução deste comando.",
						Success: false,
					})
					sink("ai.step_done", protocol.AIStepDonePayload{
						TaskID:  taskID,
						Step:    step,
						Action:  act.Name,
						Success: false,
						Output:  "Ação recusada pelo usuário",
					})
					continue
				}
			}
		}

		// 4. Emit preview + normalized target point for Android overlay click halo
		preview := protocol.AIStepPreviewPayload{
			TaskID:      taskID,
			Step:        step,
			Action:      act.Name,
			Description: formatActionLabel(act),
			Thought:     resp.Thought,
		}
		if hasPoint(act.Name) {
			_, _, rx, ry := o.arbiter.MapImageCoords(act.X, act.Y, snap.Width, snap.Height)
			preview.HasPoint = true
			preview.XNorm = rx
			preview.YNorm = ry
		}
		if o.overlay != nil {
			o.overlay.SetState(overlay.StateAIActing, "AnyDask AI · "+preview.Description)
		}
		sink("ai.status", protocol.AIStatusPayload{
			TaskID:  taskID,
			State:   "ACTING",
			Message: preview.Description,
			Step:    step,
			MaxStep: maxSteps,
		})
		sink("ai.step_preview", preview)

		// Small visual lead-in so user sees the target halo & liquid ripple before the click lands
		if preview.HasPoint {
			if o.overlay != nil {
				sx, sy, _, _ := o.arbiter.MapImageCoords(act.X, act.Y, snap.Width, snap.Height)
				o.overlay.TriggerRipple(sx, sy, true)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(140 * time.Millisecond):
			}
		}

		// 5. Execute action through InputArbiter / Shell
		outStr, execErr := o.executeAction(ctx, taskID, act, snap.Width, snap.Height)
		if ctx.Err() != nil {
			return
		}
		success := execErr == nil
		if execErr != nil {
			outStr = execErr.Error()
		}

		history = append(history, StepRecord{
			Step:    step,
			Thought: resp.Thought,
			Action:  act,
			Result:  outStr,
			Success: success,
		})

		sink("ai.step_done", protocol.AIStepDonePayload{
			TaskID:  taskID,
			Step:    step,
			Action:  act.Name,
			Success: success,
			Output:  outStr,
		})

		// Allow UI animations / window focus transitions to settle before capturing next frame
		settle := 320 * time.Millisecond
		if act.Name == "wait" {
			settle = 50 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(settle):
		}
	}

	sink("ai.status", protocol.AIStatusPayload{
		TaskID:  taskID,
		State:   "COMPLETED",
		Message: fmt.Sprintf("Limite de %d passos atingido. Controle devolvido.", maxSteps),
		Step:    maxSteps,
		MaxStep: maxSteps,
	})
}

func (o *Orchestrator) executeAction(ctx context.Context, taskID string, act Action, imgW, imgH int) (string, error) {
	switch act.Name {
	case "left_click":
		sx, sy, _, _ := o.arbiter.MapImageCoords(act.X, act.Y, imgW, imgH)
		err := o.arbiter.ExecuteAI(taskID, func(inj input.Injector) error {
			if err := inj.MoveAbsolute(sx, sy); err != nil {
				return err
			}
			time.Sleep(35 * time.Millisecond)
			return inj.MouseButton(protocol.MouseBtnLeft, protocol.MouseActionClick)
		})
		return fmt.Sprintf("Clique esquerdo em (%d,%d)", sx, sy), err

	case "double_click":
		sx, sy, _, _ := o.arbiter.MapImageCoords(act.X, act.Y, imgW, imgH)
		err := o.arbiter.ExecuteAI(taskID, func(inj input.Injector) error {
			if err := inj.MoveAbsolute(sx, sy); err != nil {
				return err
			}
			time.Sleep(35 * time.Millisecond)
			return inj.MouseButton(protocol.MouseBtnLeft, protocol.MouseActionDblClick)
		})
		return fmt.Sprintf("Duplo clique em (%d,%d)", sx, sy), err

	case "right_click":
		sx, sy, _, _ := o.arbiter.MapImageCoords(act.X, act.Y, imgW, imgH)
		err := o.arbiter.ExecuteAI(taskID, func(inj input.Injector) error {
			if err := inj.MoveAbsolute(sx, sy); err != nil {
				return err
			}
			time.Sleep(35 * time.Millisecond)
			return inj.MouseButton(protocol.MouseBtnRight, protocol.MouseActionClick)
		})
		return fmt.Sprintf("Clique direito em (%d,%d)", sx, sy), err

	case "drag":
		sx1, sy1, _, _ := o.arbiter.MapImageCoords(act.X, act.Y, imgW, imgH)
		sx2, sy2, _, _ := o.arbiter.MapImageCoords(act.EndX, act.EndY, imgW, imgH)
		err := o.arbiter.ExecuteAI(taskID, func(inj input.Injector) error {
			return inj.Drag(sx1, sy1, sx2, sy2)
		})
		return fmt.Sprintf("Arrastou (%d,%d)->(%d,%d)", sx1, sy1, sx2, sy2), err

	case "scroll":
		sx, sy, _, _ := o.arbiter.MapImageCoords(act.X, act.Y, imgW, imgH)
		err := o.arbiter.ExecuteAI(taskID, func(inj input.Injector) error {
			_ = inj.MoveAbsolute(sx, sy)
			return inj.Scroll(act.DeltaX, act.DeltaY)
		})
		return fmt.Sprintf("Scroll dy=%d", act.DeltaY), err

	case "type_text":
		err := o.arbiter.ExecuteAI(taskID, func(inj input.Injector) error {
			return inj.TypeText(act.Text)
		})
		return "Texto digitado", err

	case "hotkey":
		err := o.arbiter.ExecuteAI(taskID, func(inj input.Injector) error {
			return inj.Hotkey(act.Keys)
		})
		return "Atalho " + strings.Join(act.Keys, "+"), err

	case "shell_exec":
		out, err := osctx.ExecShell(ctx, act.Command)
		if out == "" && err == nil {
			out = "(sem saída)"
		}
		return out, err

	case "wait":
		sec := act.Seconds
		if sec <= 0 {
			sec = 1
		}
		if sec > 8 {
			sec = 8
		}
		select {
		case <-ctx.Done():
			return "cancelado", ctx.Err()
		case <-time.After(time.Duration(sec * float64(time.Second))):
			return fmt.Sprintf("Aguardou %.1fs", sec), nil
		}
	default:
		return "", fmt.Errorf("ação desconhecida: %s", act.Name)
	}
}

func hasPoint(actionName string) bool {
	switch actionName {
	case "left_click", "double_click", "right_click", "drag", "scroll":
		return true
	}
	return false
}

func formatActionLabel(act Action) string {
	if strings.TrimSpace(act.Reason) != "" {
		return act.Reason
	}
	switch act.Name {
	case "left_click":
		return "Clicando na tela..."
	case "double_click":
		return "Duplo clique..."
	case "right_click":
		return "Abrindo menu..."
	case "type_text":
		return "Digitando texto..."
	case "hotkey":
		return "Atalho " + strings.Join(act.Keys, "+")
	case "shell_exec":
		return "Executando no terminal..."
	case "scroll":
		return "Rolando tela..."
	case "wait":
		return "Aguardando..."
	default:
		return act.Name
	}
}

func randomID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
