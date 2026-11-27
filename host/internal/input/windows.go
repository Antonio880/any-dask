//go:build windows

package input

import (
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/anydask/host/internal/protocol"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procSendInput        = user32.NewProc("SendInput")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procSetCursorPos     = user32.NewProc("SetCursorPos")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procVkKeyScanW       = user32.NewProc("VkKeyScanW")
	procSetProcessDPI    = user32.NewProc("SetProcessDPIAware")
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	mouseEventMove        = 0x0001
	mouseEventLeftDown    = 0x0002
	mouseEventLeftUp      = 0x0004
	mouseEventRightDown   = 0x0008
	mouseEventRightUp     = 0x0010
	mouseEventMiddleDown  = 0x0020
	mouseEventMiddleUp    = 0x0040
	mouseEventWheel       = 0x0800
	mouseEventHWheel      = 0x1000
	mouseEventVirtualDesk = 0x4000
	mouseEventAbsolute    = 0x8000

	keyEventExtended = 0x0001
	keyEventKeyUp    = 0x0002
	keyEventUnicode  = 0x0004

	smCXScreen = 0
	smCYScreen = 1
)

type mouseInput struct {
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// Win32 INPUT struct on 64-bit Windows is 40 bytes (uint32 type + 4 bytes padding + 32-byte union).
type winMouseInput struct {
	inputType uint32
	_pad      uint32
	mi        mouseInput
}

type winKeybdInput struct {
	inputType uint32
	_pad      uint32
	ki        keybdInput
	_pad2     [8]byte
}

type point struct {
	X int32
	Y int32
}

type windowsInjector struct{}

func NewInjector() Injector {
	// Ensure physical pixel coordinates match DXGI/GDI screen capture even on HiDPI (125%/150%/200%).
	_, _, _ = procSetProcessDPI.Call()
	return &windowsInjector{}
}

func (w *windowsInjector) ScreenBounds() ScreenBounds {
	cx, _, _ := procGetSystemMetrics.Call(smCXScreen)
	cy, _, _ := procGetSystemMetrics.Call(smCYScreen)
	width := int(cx)
	height := int(cy)
	if width <= 0 {
		width = 1920
	}
	if height <= 0 {
		height = 1080
	}
	return ScreenBounds{X: 0, Y: 0, Width: width, Height: height}
}

func (w *windowsInjector) MoveRelative(dx, dy int) error {
	var pt point
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	b := w.ScreenBounds()
	nx := int(pt.X) + dx
	ny := int(pt.Y) + dy
	if nx < 0 {
		nx = 0
	}
	if ny < 0 {
		ny = 0
	}
	if nx >= b.Width {
		nx = b.Width - 1
	}
	if ny >= b.Height {
		ny = b.Height - 1
	}
	_, _, _ = procSetCursorPos.Call(uintptr(nx), uintptr(ny))
	return nil
}

func (w *windowsInjector) MoveAbsolute(x, y int) error {
	_, _, _ = procSetCursorPos.Call(uintptr(x), uintptr(y))
	return nil
}

func (w *windowsInjector) sendMouseFlag(flags uint32, data uint32) {
	in := winMouseInput{
		inputType: inputMouse,
		mi: mouseInput{
			mouseData: data,
			dwFlags:   flags,
		},
	}
	_, _, _ = procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func (w *windowsInjector) MouseButton(button uint8, action uint8) error {
	var downFlag, upFlag uint32
	switch button {
	case protocol.MouseBtnRight:
		downFlag, upFlag = mouseEventRightDown, mouseEventRightUp
	case protocol.MouseBtnMiddle:
		downFlag, upFlag = mouseEventMiddleDown, mouseEventMiddleUp
	default:
		downFlag, upFlag = mouseEventLeftDown, mouseEventLeftUp
	}

	switch action {
	case protocol.MouseActionDown:
		w.sendMouseFlag(downFlag, 0)
	case protocol.MouseActionUp:
		w.sendMouseFlag(upFlag, 0)
	case protocol.MouseActionClick:
		w.sendMouseFlag(downFlag, 0)
		time.Sleep(12 * time.Millisecond)
		w.sendMouseFlag(upFlag, 0)
	case protocol.MouseActionDblClick:
		w.sendMouseFlag(downFlag, 0)
		time.Sleep(12 * time.Millisecond)
		w.sendMouseFlag(upFlag, 0)
		time.Sleep(35 * time.Millisecond)
		w.sendMouseFlag(downFlag, 0)
		time.Sleep(12 * time.Millisecond)
		w.sendMouseFlag(upFlag, 0)
	}
	return nil
}

func (w *windowsInjector) Scroll(dx, dy int) error {
	if dy != 0 {
		// Windows WHEEL_DELTA is 120 per notch. Negative dy from browser wheel means scroll down.
		delta := int32(-dy * 40)
		w.sendMouseFlag(mouseEventWheel, uint32(delta))
	}
	if dx != 0 {
		delta := int32(dx * 40)
		w.sendMouseFlag(mouseEventHWheel, uint32(delta))
	}
	return nil
}

func (w *windowsInjector) Drag(x1, y1, x2, y2 int) error {
	_ = w.MoveAbsolute(x1, y1)
	time.Sleep(25 * time.Millisecond)
	w.sendMouseFlag(mouseEventLeftDown, 0)
	steps := 12
	for i := 1; i <= steps; i++ {
		nx := x1 + ((x2 - x1) * i / steps)
		ny := y1 + ((y2 - y1) * i / steps)
		_ = w.MoveAbsolute(nx, ny)
		time.Sleep(10 * time.Millisecond)
	}
	w.sendMouseFlag(mouseEventLeftUp, 0)
	return nil
}

func (w *windowsInjector) sendVK(vk uint16, keyUp bool) {
	flags := uint32(0)
	if isExtendedVK(vk) {
		flags |= keyEventExtended
	}
	if keyUp {
		flags |= keyEventKeyUp
	}
	in := winKeybdInput{
		inputType: inputKeyboard,
		ki: keybdInput{
			wVk:     vk,
			dwFlags: flags,
		},
	}
	_, _, _ = procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func (w *windowsInjector) KeyEvent(key string, action string, modifiers []string) error {
	modVKs := mapModifiersWindows(modifiers)

	// Single printable character without modifiers: use Unicode injection for full PT-BR/ABNT2 fidelity.
	if len(modVKs) == 0 && len([]rune(key)) == 1 && action == "press" {
		return w.TypeText(key)
	}

	vk, needShift := keyNameToVK(key)
	if vk == 0 {
		if len([]rune(key)) == 1 && (action == "press" || action == "down") {
			return w.TypeText(key)
		}
		return nil
	}

	if needShift && !containsVK(modVKs, 0x10) {
		modVKs = append(modVKs, 0x10)
	}

	switch action {
	case "down":
		w.sendVK(vk, false)
	case "up":
		w.sendVK(vk, true)
	default: // "press"
		for _, m := range modVKs {
			w.sendVK(m, false)
		}
		w.sendVK(vk, false)
		time.Sleep(10 * time.Millisecond)
		w.sendVK(vk, true)
		for i := len(modVKs) - 1; i >= 0; i-- {
			w.sendVK(modVKs[i], true)
		}
	}
	return nil
}

func (w *windowsInjector) TypeText(text string) error {
	for _, r := range text {
		switch r {
		case '\n':
			w.sendVK(0x0D, false)
			w.sendVK(0x0D, true)
			continue
		case '\t':
			w.sendVK(0x09, false)
			w.sendVK(0x09, true)
			continue
		}
		units := utf16.Encode([]rune{r})
		for _, u := range units {
			down := winKeybdInput{
				inputType: inputKeyboard,
				ki: keybdInput{
					wScan:   u,
					dwFlags: keyEventUnicode,
				},
			}
			up := winKeybdInput{
				inputType: inputKeyboard,
				ki: keybdInput{
					wScan:   u,
					dwFlags: keyEventUnicode | keyEventKeyUp,
				},
			}
			_, _, _ = procSendInput.Call(1, uintptr(unsafe.Pointer(&down)), unsafe.Sizeof(down))
			_, _, _ = procSendInput.Call(1, uintptr(unsafe.Pointer(&up)), unsafe.Sizeof(up))
		}
		time.Sleep(4 * time.Millisecond)
	}
	return nil
}

func (w *windowsInjector) Hotkey(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	var vks []uint16
	for _, k := range keys {
		vk, _ := keyNameToVK(k)
		if vk != 0 {
			vks = append(vks, vk)
		}
	}
	if len(vks) == 0 {
		return nil
	}
	for _, vk := range vks {
		w.sendVK(vk, false)
		time.Sleep(8 * time.Millisecond)
	}
	time.Sleep(15 * time.Millisecond)
	for i := len(vks) - 1; i >= 0; i-- {
		w.sendVK(vks[i], true)
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func mapModifiersWindows(mods []string) []uint16 {
	var out []uint16
	for _, m := range mods {
		switch strings.ToUpper(strings.TrimSpace(m)) {
		case "PRIMARY", "CTRL", "CONTROL":
			if !containsVK(out, 0x11) {
				out = append(out, 0x11) // VK_CONTROL
			}
		case "CMD", "META", "SUPER", "WIN":
			if !containsVK(out, 0x5B) {
				out = append(out, 0x5B) // VK_LWIN
			}
		case "ALT", "OPT", "OPTION":
			if !containsVK(out, 0x12) {
				out = append(out, 0x12) // VK_MENU
			}
		case "SHIFT":
			if !containsVK(out, 0x10) {
				out = append(out, 0x10) // VK_SHIFT
			}
		}
	}
	return out
}

func containsVK(list []uint16, v uint16) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func isExtendedVK(vk uint16) bool {
	switch vk {
	case 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x2D, 0x2E, 0x5B, 0x5C:
		return true
	}
	return false
}

func keyNameToVK(key string) (uint16, bool) {
	k := strings.TrimSpace(key)
	lower := strings.ToLower(k)
	switch lower {
	case "enter", "return":
		return 0x0D, false
	case "tab":
		return 0x09, false
	case "escape", "esc":
		return 0x1B, false
	case "backspace":
		return 0x08, false
	case "delete", "del":
		return 0x2E, false
	case "insert", "ins":
		return 0x2D, false
	case "space", " ":
		return 0x20, false
	case "arrowup", "up":
		return 0x26, false
	case "arrowdown", "down":
		return 0x28, false
	case "arrowleft", "left":
		return 0x25, false
	case "arrowright", "right":
		return 0x27, false
	case "home":
		return 0x24, false
	case "end":
		return 0x23, false
	case "pageup", "pgup":
		return 0x21, false
	case "pagedown", "pgdn":
		return 0x22, false
	case "primary", "ctrl", "control":
		return 0x11, false
	case "shift":
		return 0x10, false
	case "alt", "option", "opt":
		return 0x12, false
	case "cmd", "command", "meta", "super", "win", "windows":
		return 0x5B, false
	case "f1":
		return 0x70, false
	case "f2":
		return 0x71, false
	case "f3":
		return 0x72, false
	case "f4":
		return 0x73, false
	case "f5":
		return 0x74, false
	case "f6":
		return 0x75, false
	case "f7":
		return 0x76, false
	case "f8":
		return 0x77, false
	case "f9":
		return 0x78, false
	case "f10":
		return 0x79, false
	case "f11":
		return 0x7A, false
	case "f12":
		return 0x7B, false
	}

	runes := []rune(k)
	if len(runes) == 1 {
		r := runes[0]
		if r >= 'a' && r <= 'z' {
			return uint16(r - 'a' + 'A'), false
		}
		if r >= 'A' && r <= 'Z' {
			return uint16(r), false
		}
		if r >= '0' && r <= '9' {
			return uint16(r), false
		}
		res, _, _ := procVkKeyScanW.Call(uintptr(r))
		val := int16(res)
		if val != -1 {
			vk := uint16(val & 0xFF)
			shift := (val & 0x0100) != 0
			return vk, shift
		}
	}
	return 0, false
}
