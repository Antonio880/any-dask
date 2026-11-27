//go:build darwin

package input

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include <ApplicationServices/ApplicationServices.h>

static void macScreenSize(int *w, int *h) {
    CGRect bounds = CGDisplayBounds(CGMainDisplayID());
    *w = (int)bounds.size.width;
    *h = (int)bounds.size.height;
}

static void macGetCursor(double *x, double *y) {
    CGEventRef ev = CGEventCreate(NULL);
    CGPoint pt = CGEventGetLocation(ev);
    CFRelease(ev);
    *x = pt.x;
    *y = pt.y;
}

static void macMoveMouse(double x, double y) {
    CGPoint pt = CGPointMake(x, y);
    CGEventRef ev = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, pt, kCGMouseButtonLeft);
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
}

static void macMouseBtn(int button, int down, double x, double y) {
    CGPoint pt = CGPointMake(x, y);
    CGEventType et = kCGEventLeftMouseDown;
    CGMouseButton mb = kCGMouseButtonLeft;
    if (button == 2) {
        mb = kCGMouseButtonRight;
        et = down ? kCGEventRightMouseDown : kCGEventRightMouseUp;
    } else if (button == 3) {
        mb = kCGMouseButtonCenter;
        et = down ? kCGEventOtherMouseDown : kCGEventOtherMouseUp;
    } else {
        mb = kCGMouseButtonLeft;
        et = down ? kCGEventLeftMouseDown : kCGEventLeftMouseUp;
    }
    CGEventRef ev = CGEventCreateMouseEvent(NULL, et, pt, mb);
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
}

static void macMouseDrag(double x, double y) {
    CGPoint pt = CGPointMake(x, y);
    CGEventRef ev = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseDragged, pt, kCGMouseButtonLeft);
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
}

static void macScroll(int dx, int dy) {
    CGEventRef ev = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitLine, 2, -dy, -dx);
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
}

static void macKeyEvent(uint16_t keycode, int down, uint64_t flags) {
    CGEventRef ev = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keycode, down != 0);
    if (flags != 0) {
        CGEventSetFlags(ev, (CGEventFlags)flags);
    }
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
}

static void macUnicodeChar(uint16_t ch) {
    UniChar uc = (UniChar)ch;
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0, true);
    CGEventKeyboardSetUnicodeString(down, 1, &uc);
    CGEventPost(kCGHIDEventTap, down);
    CFRelease(down);

    CGEventRef up = CGEventCreateKeyboardEvent(NULL, 0, false);
    CGEventKeyboardSetUnicodeString(up, 1, &uc);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(up);
}
*/
import "C"

import (
	"strings"
	"time"
	"unicode/utf16"

	"github.com/anydask/host/internal/protocol"
)

type darwinInjector struct{}

func NewInjector() Injector {
	return &darwinInjector{}
}

func (d *darwinInjector) ScreenBounds() ScreenBounds {
	var w, h C.int
	C.macScreenSize(&w, &h)
	return ScreenBounds{X: 0, Y: 0, Width: int(w), Height: int(h)}
}

func (d *darwinInjector) MoveRelative(dx, dy int) error {
	var cx, cy C.double
	C.macGetCursor(&cx, &cy)
	b := d.ScreenBounds()
	nx := float64(cx) + float64(dx)
	ny := float64(cy) + float64(dy)
	if nx < 0 {
		nx = 0
	}
	if ny < 0 {
		ny = 0
	}
	if nx >= float64(b.Width) {
		nx = float64(b.Width - 1)
	}
	if ny >= float64(b.Height) {
		ny = float64(b.Height - 1)
	}
	C.macMoveMouse(C.double(nx), C.double(ny))
	return nil
}

func (d *darwinInjector) MoveAbsolute(x, y int) error {
	C.macMoveMouse(C.double(x), C.double(y))
	return nil
}

func (d *darwinInjector) MouseButton(button uint8, action uint8) error {
	var cx, cy C.double
	C.macGetCursor(&cx, &cy)
	switch action {
	case protocol.MouseActionDown:
		C.macMouseBtn(C.int(button), 1, cx, cy)
	case protocol.MouseActionUp:
		C.macMouseBtn(C.int(button), 0, cx, cy)
	case protocol.MouseActionClick:
		C.macMouseBtn(C.int(button), 1, cx, cy)
		time.Sleep(12 * time.Millisecond)
		C.macMouseBtn(C.int(button), 0, cx, cy)
	case protocol.MouseActionDblClick:
		C.macMouseBtn(C.int(button), 1, cx, cy)
		time.Sleep(12 * time.Millisecond)
		C.macMouseBtn(C.int(button), 0, cx, cy)
		time.Sleep(35 * time.Millisecond)
		C.macMouseBtn(C.int(button), 1, cx, cy)
		time.Sleep(12 * time.Millisecond)
		C.macMouseBtn(C.int(button), 0, cx, cy)
	}
	return nil
}

func (d *darwinInjector) Scroll(dx, dy int) error {
	C.macScroll(C.int(dx), C.int(dy))
	return nil
}

func (d *darwinInjector) Drag(x1, y1, x2, y2 int) error {
	C.macMoveMouse(C.double(x1), C.double(y1))
	time.Sleep(25 * time.Millisecond)
	C.macMouseBtn(1, 1, C.double(x1), C.double(y1))
	steps := 12
	for i := 1; i <= steps; i++ {
		nx := float64(x1) + float64(x2-x1)*float64(i)/float64(steps)
		ny := float64(y1) + float64(y2-y1)*float64(i)/float64(steps)
		C.macMouseDrag(C.double(nx), C.double(ny))
		time.Sleep(10 * time.Millisecond)
	}
	C.macMouseBtn(1, 0, C.double(x2), C.double(y2))
	return nil
}

func (d *darwinInjector) KeyEvent(key string, action string, modifiers []string) error {
	flags := mapModifiersMac(modifiers)
	if flags == 0 && len([]rune(key)) == 1 && action == "press" {
		return d.TypeText(key)
	}
	code, ok := macKeyCode(key)
	if !ok {
		if len([]rune(key)) == 1 && (action == "press" || action == "down") {
			return d.TypeText(key)
		}
		return nil
	}
	switch action {
	case "down":
		C.macKeyEvent(C.uint16_t(code), 1, C.uint64_t(flags))
	case "up":
		C.macKeyEvent(C.uint16_t(code), 0, C.uint64_t(flags))
	default:
		C.macKeyEvent(C.uint16_t(code), 1, C.uint64_t(flags))
		time.Sleep(10 * time.Millisecond)
		C.macKeyEvent(C.uint16_t(code), 0, C.uint64_t(flags))
	}
	return nil
}

func (d *darwinInjector) TypeText(text string) error {
	for _, r := range text {
		switch r {
		case '\n':
			C.macKeyEvent(36, 1, 0)
			C.macKeyEvent(36, 0, 0)
			continue
		case '\t':
			C.macKeyEvent(48, 1, 0)
			C.macKeyEvent(48, 0, 0)
			continue
		}
		for _, u := range utf16.Encode([]rune{r}) {
			C.macUnicodeChar(C.uint16_t(u))
		}
		time.Sleep(4 * time.Millisecond)
	}
	return nil
}

func (d *darwinInjector) Hotkey(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	var mods []string
	var mainKey string
	for _, k := range keys {
		switch strings.ToUpper(strings.TrimSpace(k)) {
		case "PRIMARY", "CMD", "COMMAND", "META", "SUPER", "CTRL", "CONTROL", "ALT", "OPT", "OPTION", "SHIFT":
			mods = append(mods, k)
		default:
			mainKey = k
		}
	}
	if mainKey == "" {
		return nil
	}
	return d.KeyEvent(mainKey, "press", mods)
}

func mapModifiersMac(mods []string) uint64 {
	var flags uint64
	for _, m := range mods {
		switch strings.ToUpper(strings.TrimSpace(m)) {
		case "PRIMARY", "CMD", "COMMAND", "META", "SUPER":
			flags |= uint64(C.kCGEventFlagMaskCommand)
		case "CTRL", "CONTROL":
			flags |= uint64(C.kCGEventFlagMaskControl)
		case "ALT", "OPT", "OPTION":
			flags |= uint64(C.kCGEventFlagMaskAlternate)
		case "SHIFT":
			flags |= uint64(C.kCGEventFlagMaskShift)
		}
	}
	return flags
}

func macKeyCode(key string) (uint16, bool) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "a":
		return 0, true
	case "s":
		return 1, true
	case "d":
		return 2, true
	case "f":
		return 3, true
	case "h":
		return 4, true
	case "g":
		return 5, true
	case "z":
		return 6, true
	case "x":
		return 7, true
	case "c":
		return 8, true
	case "v":
		return 9, true
	case "b":
		return 11, true
	case "q":
		return 12, true
	case "w":
		return 13, true
	case "e":
		return 14, true
	case "r":
		return 15, true
	case "y":
		return 16, true
	case "t":
		return 17, true
	case "1":
		return 18, true
	case "2":
		return 19, true
	case "3":
		return 20, true
	case "4":
		return 21, true
	case "6":
		return 22, true
	case "5":
		return 23, true
	case "=":
		return 24, true
	case "9":
		return 25, true
	case "7":
		return 26, true
	case "-":
		return 27, true
	case "8":
		return 28, true
	case "0":
		return 29, true
	case "]":
		return 30, true
	case "o":
		return 31, true
	case "u":
		return 32, true
	case "[":
		return 33, true
	case "i":
		return 34, true
	case "p":
		return 35, true
	case "enter", "return":
		return 36, true
	case "l":
		return 37, true
	case "j":
		return 38, true
	case "'":
		return 39, true
	case "k":
		return 40, true
	case ";":
		return 41, true
	case "\\":
		return 42, true
	case ",":
		return 43, true
	case "/":
		return 44, true
	case "n":
		return 45, true
	case "m":
		return 46, true
	case ".":
		return 47, true
	case "tab":
		return 48, true
	case "space", " ":
		return 49, true
	case "`":
		return 50, true
	case "backspace":
		return 51, true
	case "escape", "esc":
		return 53, true
	case "delete", "del":
		return 117, true
	case "arrowleft", "left":
		return 123, true
	case "arrowright", "right":
		return 124, true
	case "arrowdown", "down":
		return 125, true
	case "arrowup", "up":
		return 126, true
	case "home":
		return 115, true
	case "end":
		return 119, true
	case "pageup", "pgup":
		return 116, true
	case "pagedown", "pgdn":
		return 121, true
	case "f1":
		return 122, true
	case "f2":
		return 120, true
	case "f3":
		return 99, true
	case "f4":
		return 118, true
	case "f5":
		return 96, true
	case "f6":
		return 97, true
	case "f7":
		return 98, true
	case "f8":
		return 100, true
	case "f9":
		return 101, true
	case "f10":
		return 109, true
	case "f11":
		return 103, true
	case "f12":
		return 111, true
	}
	return 0, false
}
