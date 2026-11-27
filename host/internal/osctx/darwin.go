//go:build darwin

package osctx

import (
	"os/exec"
	"strings"
)

func ActiveWindowTitle() string {
	script := `tell application "System Events" to get name of first application process whose frontmost is true`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func SetClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
