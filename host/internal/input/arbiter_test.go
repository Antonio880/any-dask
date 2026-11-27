package input

import (
	"testing"

	"github.com/anydask/host/internal/protocol"
)

type fakeInjector struct {
	bounds ScreenBounds
	moves  int
}

func (f *fakeInjector) ScreenBounds() ScreenBounds                   { return f.bounds }
func (f *fakeInjector) MoveRelative(dx, dy int) error                { f.moves++; return nil }
func (f *fakeInjector) MoveAbsolute(x, y int) error                  { f.moves++; return nil }
func (f *fakeInjector) MouseButton(button uint8, action uint8) error { return nil }
func (f *fakeInjector) Scroll(dx, dy int) error                      { return nil }
func (f *fakeInjector) KeyEvent(key, action string, m []string) error { return nil }
func (f *fakeInjector) TypeText(text string) error                   { return nil }
func (f *fakeInjector) Hotkey(keys []string) error                   { return nil }
func (f *fakeInjector) Drag(x1, y1, x2, y2 int) error                { return nil }

func TestArbiterLockAndKillSwitch(t *testing.T) {
	inj := &fakeInjector{bounds: ScreenBounds{Width: 2560, Height: 1440}}
	arb := NewArbiter(inj)

	if err := arb.HandleManualBinary(protocol.BinaryInputEvent{Type: protocol.BinMouseMoveRel, DX: 5, DY: 5}); err != nil {
		t.Fatalf("manual move should succeed: %v", err)
	}

	aborted := false
	if err := arb.AcquireAI("task-1", func() { aborted = true }); err != nil {
		t.Fatalf("acquire AI failed: %v", err)
	}

	// Manual input must be rejected while AI holds lock
	if err := arb.HandleManualBinary(protocol.BinaryInputEvent{Type: protocol.BinMouseMoveRel, DX: 5, DY: 5}); err != ErrNotManualMode {
		t.Fatalf("expected ErrNotManualMode while AI active, got %v", err)
	}

	// Kill switch releases lock immediately and invokes cancel callback
	if !arb.AbortAI() || !aborted {
		t.Fatalf("expected AbortAI to cancel active task")
	}
	if arb.Mode() != ModeManual {
		t.Fatalf("expected MANUAL mode after abort, got %s", arb.Mode())
	}

	// CoordinateMapper HiDPI projection check
	sx, sy, rx, ry := arb.MapImageCoords(640, 360, 1280, 720)
	if sx != 1280 || sy != 720 || rx != 0.5 || ry != 0.5 {
		t.Fatalf("unexpected projection: (%d,%d) ratio (%.2f,%.2f)", sx, sy, rx, ry)
	}
}
