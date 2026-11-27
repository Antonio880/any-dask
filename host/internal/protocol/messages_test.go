package protocol

import (
	"encoding/binary"
	"testing"
)

func TestDecodeBinaryInput(t *testing.T) {
	// Relative mouse move
	b := make([]byte, 6)
	b[0] = BinMouseMoveRel
	dx := int16(-42)
	dy := int16(17)
	binary.BigEndian.PutUint16(b[1:3], uint16(dx))
	binary.BigEndian.PutUint16(b[3:5], uint16(dy))
	ev, err := DecodeBinaryInput(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.DX != -42 || ev.DY != 17 {
		t.Fatalf("expected (-42,17), got (%d,%d)", ev.DX, ev.DY)
	}

	// Button click with normalized coordinates
	btn := make([]byte, 7)
	btn[0] = BinMouseButton
	btn[1] = MouseBtnRight
	btn[2] = MouseActionClick
	binary.BigEndian.PutUint16(btn[3:5], 32768)
	binary.BigEndian.PutUint16(btn[5:7], 65535)
	ev2, err := DecodeBinaryInput(btn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev2.Button != MouseBtnRight || ev2.Action != MouseActionClick || ev2.XNorm != 32768 || ev2.YNorm != 65535 {
		t.Fatalf("unexpected button event: %+v", ev2)
	}
}
