package input

import (
	"errors"
	"math"
	"sync"

	"github.com/anydask/host/internal/protocol"
)

type Mode string

const (
	ModeManual         Mode = "MANUAL"
	ModeAIActing       Mode = "AI_ACTING"
	ModeWaitingConfirm Mode = "AI_WAITING_CONFIRM"
)

var (
	ErrNotManualMode = errors.New("input locked by AI agent")
	ErrAIAborted     = errors.New("AI task is not active or was aborted")
)

// ScreenBounds describes the target display bounds.
type ScreenBounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Injector is implemented per OS (Windows via user32 SendInput, macOS via CoreGraphics CGEvent).
type Injector interface {
	Displays() []protocol.DisplayInfo
	ActiveDisplay() int
	SetActiveDisplay(index int) protocol.DisplayInfo
	ScreenBounds() ScreenBounds
	MoveRelative(dx, dy int) error
	MoveAbsolute(x, y int) error
	MouseButton(button uint8, action uint8) error
	Scroll(dx, dy int) error
	KeyEvent(key string, action string, modifiers []string) error
	TypeText(text string) error
	Hotkey(keys []string) error
	Drag(x1, y1, x2, y2 int) error
}

// Arbiter guarantees conflict-free control between human manual input and the AI Computer Use agent,
// with a deterministic <10ms Kill Switch.
type Arbiter struct {
	mu           sync.RWMutex
	mode         Mode
	activeTaskID string
	injector     Injector
	onAbort      func()
}

func NewArbiter(inj Injector) *Arbiter {
	return &Arbiter{
		mode:     ModeManual,
		injector: inj,
	}
}

func (a *Arbiter) Mode() Mode {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mode
}

func (a *Arbiter) ScreenBounds() ScreenBounds {
	return a.injector.ScreenBounds()
}

func (a *Arbiter) Displays() []protocol.DisplayInfo {
	return a.injector.Displays()
}

func (a *Arbiter) ActiveDisplay() int {
	return a.injector.ActiveDisplay()
}

func (a *Arbiter) SetActiveDisplay(index int) protocol.DisplayInfo {
	return a.injector.SetActiveDisplay(index)
}

// AcquireAI locks the arbiter for an AI task.
func (a *Arbiter) AcquireAI(taskID string, onAbort func()) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.mode != ModeManual {
		return errors.New("another AI task is already running")
	}
	a.mode = ModeAIActing
	a.activeTaskID = taskID
	a.onAbort = onAbort
	return nil
}

func (a *Arbiter) SetWaitingConfirm(taskID string, waiting bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeTaskID != taskID {
		return
	}
	if waiting {
		a.mode = ModeWaitingConfirm
	} else if a.mode == ModeWaitingConfirm {
		a.mode = ModeAIActing
	}
}

// ReleaseAI returns control to the user when a task finishes cleanly.
func (a *Arbiter) ReleaseAI(taskID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeTaskID == taskID {
		a.mode = ModeManual
		a.activeTaskID = ""
		a.onAbort = nil
	}
}

// AbortAI is the immediate Kill Switch (<1ms lock release + cancels AI context).
func (a *Arbiter) AbortAI() bool {
	a.mu.Lock()
	wasActive := a.mode != ModeManual
	cb := a.onAbort
	a.mode = ModeManual
	a.activeTaskID = ""
	a.onAbort = nil
	a.mu.Unlock()

	if cb != nil {
		cb()
	}
	return wasActive
}

// MapNormalized converts 0..65535 normalized coords into OS display coordinates.
func (a *Arbiter) MapNormalized(xNorm, yNorm uint16) (int, int) {
	b := a.injector.ScreenBounds()
	x := b.X + int(math.Round((float64(xNorm)/65535.0)*float64(b.Width-1)))
	y := b.Y + int(math.Round((float64(yNorm)/65535.0)*float64(b.Height-1)))
	return x, y
}

// MapImageCoords maps coordinates from a resized screenshot (imgW x imgH) to OS display coordinates.
func (a *Arbiter) MapImageCoords(imgX, imgY, imgW, imgH int) (int, int, float64, float64) {
	b := a.injector.ScreenBounds()
	if imgW <= 0 || imgH <= 0 {
		return b.X, b.Y, 0, 0
	}
	rx := clampFloat(float64(imgX)/float64(imgW), 0, 1)
	ry := clampFloat(float64(imgY)/float64(imgH), 0, 1)
	x := b.X + int(math.Round(rx*float64(b.Width-1)))
	y := b.Y + int(math.Round(ry*float64(b.Height-1)))
	return x, y, rx, ry
}

// HandleManualBinary executes a low-latency binary input event only when in MANUAL mode.
func (a *Arbiter) HandleManualBinary(ev protocol.BinaryInputEvent) error {
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()
	if mode != ModeManual {
		return ErrNotManualMode
	}

	switch ev.Type {
	case protocol.BinMouseMoveRel:
		return a.injector.MoveRelative(int(ev.DX), int(ev.DY))
	case protocol.BinMouseMoveAbs:
		x, y := a.MapNormalized(ev.XNorm, ev.YNorm)
		return a.injector.MoveAbsolute(x, y)
	case protocol.BinMouseButton:
		if ev.XNorm != 0xFFFF && ev.YNorm != 0xFFFF {
			x, y := a.MapNormalized(ev.XNorm, ev.YNorm)
			_ = a.injector.MoveAbsolute(x, y)
		}
		return a.injector.MouseButton(ev.Button, ev.Action)
	case protocol.BinMouseScroll:
		return a.injector.Scroll(int(ev.DX), int(ev.DY))
	}
	return nil
}

// HandleManualKey executes a keyboard event when in MANUAL mode.
func (a *Arbiter) HandleManualKey(ev protocol.KeyEventPayload) error {
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()
	if mode != ModeManual {
		return ErrNotManualMode
	}
	return a.injector.KeyEvent(ev.Key, ev.Action, ev.Modifiers)
}

// HandleManualText types Unicode text when in MANUAL mode.
func (a *Arbiter) HandleManualText(text string) error {
	a.mu.RLock()
	mode := a.mode
	a.mu.RUnlock()
	if mode != ModeManual {
		return ErrNotManualMode
	}
	return a.injector.TypeText(text)
}

// ExecuteAI validates that taskID still holds the lock before running fn on the OS injector.
func (a *Arbiter) ExecuteAI(taskID string, fn func(inj Injector) error) error {
	a.mu.RLock()
	ok := a.mode == ModeAIActing && a.activeTaskID == taskID
	a.mu.RUnlock()
	if !ok {
		return ErrAIAborted
	}
	return fn(a.injector)
}

func clampFloat(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
