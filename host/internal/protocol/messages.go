package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
)

// Binary input event types (DataChannel: "input", unordered, maxRetransmits=0).
const (
	BinMouseMoveRel uint8 = 0x01
	BinMouseMoveAbs uint8 = 0x02
	BinMouseButton  uint8 = 0x03
	BinMouseScroll  uint8 = 0x04
)

// Mouse button constants.
const (
	MouseBtnLeft   uint8 = 1
	MouseBtnRight  uint8 = 2
	MouseBtnMiddle uint8 = 3
)

// Mouse button actions.
const (
	MouseActionUp       uint8 = 0
	MouseActionDown     uint8 = 1
	MouseActionClick    uint8 = 2
	MouseActionDblClick uint8 = 3
)

// BinaryInputEvent represents a decoded low-latency binary input packet.
type BinaryInputEvent struct {
	Type   uint8
	DX     int16
	DY     int16
	XNorm  uint16 // 0..65535 (0xFFFF means omit / use current cursor pos for button events)
	YNorm  uint16 // 0..65535
	Button uint8
	Action uint8
	Flags  uint8
}

// DecodeBinaryInput parses a 6-byte binary input packet.
func DecodeBinaryInput(data []byte) (BinaryInputEvent, error) {
	if len(data) < 6 {
		return BinaryInputEvent{}, errors.New("binary input packet too short")
	}
	ev := BinaryInputEvent{Type: data[0]}
	switch ev.Type {
	case BinMouseMoveRel, BinMouseScroll:
		ev.DX = int16(binary.BigEndian.Uint16(data[1:3]))
		ev.DY = int16(binary.BigEndian.Uint16(data[3:5]))
		ev.Flags = data[5]
	case BinMouseMoveAbs:
		ev.XNorm = binary.BigEndian.Uint16(data[1:3])
		ev.YNorm = binary.BigEndian.Uint16(data[3:5])
		ev.Flags = data[5]
	case BinMouseButton:
		ev.Button = data[1]
		ev.Action = data[2]
		if len(data) >= 7 {
			ev.XNorm = binary.BigEndian.Uint16(data[3:5])
			ev.YNorm = binary.BigEndian.Uint16(data[5:7])
		} else {
			ev.XNorm = 0xFFFF
			ev.YNorm = 0xFFFF
		}
	default:
		return BinaryInputEvent{}, errors.New("unknown binary input event type")
	}
	return ev, nil
}

// ControlMessage is the JSON envelope for DataChannel "control".
type ControlMessage struct {
	Type    string          `json:"type"`
	TS      int64           `json:"ts,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func MarshalControl(msgType string, ts int64, payload any) ([]byte, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return json.Marshal(ControlMessage{
		Type:    msgType,
		TS:      ts,
		Payload: raw,
	})
}

type HostInfo struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"` // "windows" | "macos" | "linux"
	Arch        string `json:"arch"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
	Encoder     string `json:"encoder"`
	CaptureMode string `json:"capture_mode"`
	PrimaryMod  string `json:"primary_mod"` // "Ctrl" or "Cmd"
}

type KeyEventPayload struct {
	Key       string   `json:"key"`                 // e.g. "Enter", "Tab", "Escape", "c", "ArrowUp"
	Code      string   `json:"code,omitempty"`      // JS KeyboardEvent.code
	Action    string   `json:"action"`              // "down" | "up" | "press"
	Modifiers []string `json:"modifiers,omitempty"` // "PRIMARY", "CTRL", "CMD", "ALT", "SHIFT"
}

type KeyTextPayload struct {
	Text string `json:"text"`
}

type ClipboardPayload struct {
	Text string `json:"text"`
}

type AIStartTaskPayload struct {
	Prompt         string `json:"prompt"`
	AudioBase64    string `json:"audio_base64,omitempty"`
	AudioMimeType  string `json:"audio_mime_type,omitempty"`
	Provider       string `json:"provider"` // "anthropic" | "gemini" | "openai"
	Model          string `json:"model"`
	APIKey         string `json:"api_key"`
	BaseURL        string `json:"base_url,omitempty"`
	MaxSteps       int    `json:"max_steps,omitempty"`
	RequireConfirm bool   `json:"require_confirm"`
}

type AIStatusPayload struct {
	TaskID  string `json:"task_id,omitempty"`
	State   string `json:"state"` // "IDLE" | "THINKING" | "ACTING" | "CONFIRM" | "COMPLETED" | "ABORTED" | "ERROR"
	Message string `json:"message,omitempty"`
	Step    int    `json:"step,omitempty"`
	MaxStep int    `json:"max_step,omitempty"`
}

type AIStepPreviewPayload struct {
	TaskID      string  `json:"task_id"`
	Step        int     `json:"step"`
	Action      string  `json:"action"`
	Description string  `json:"description"`
	Thought     string  `json:"thought,omitempty"`
	HasPoint    bool    `json:"has_point,omitempty"`
	XNorm       float64 `json:"x_norm,omitempty"` // 0..1 for overlay click halo
	YNorm       float64 `json:"y_norm,omitempty"` // 0..1 for overlay click halo
}

type AIStepDonePayload struct {
	TaskID  string `json:"task_id"`
	Step    int    `json:"step"`
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
}

type AIConfirmRequiredPayload struct {
	TaskID      string `json:"task_id"`
	ConfirmID   string `json:"confirm_id"`
	Action      string `json:"action"`
	Command     string `json:"command"`
	Reason      string `json:"reason"`
}

type AIConfirmResponsePayload struct {
	TaskID    string `json:"task_id"`
	ConfirmID string `json:"confirm_id"`
	Approved  bool   `json:"approved"`
}
