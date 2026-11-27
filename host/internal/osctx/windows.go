//go:build windows

package osctx

import (
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32Ctx               = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow = user32Ctx.NewProc("GetForegroundWindow")
	procGetWindowTextW      = user32Ctx.NewProc("GetWindowTextW")
)

func ActiveWindowTitle() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	buf := make([]uint16, 256)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

func SetClipboard(text string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Set-Clipboard -Value ([Console]::In.ReadToEnd())")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
