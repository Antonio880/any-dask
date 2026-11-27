package osctx

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type ContextInfo struct {
	OS           string `json:"os"`
	Hostname     string `json:"hostname"`
	ActiveWindow string `json:"active_window"`
}

func Collect() ContextInfo {
	host, _ := os.Hostname()
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	return ContextInfo{
		OS:           osName,
		Hostname:     host,
		ActiveWindow: ActiveWindowTitle(),
	}
}

// ExecShell executes a shell command bound to ctx so Kill Switch cancels it immediately.
func ExecShell(ctx context.Context, command string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(runCtx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	} else {
		cmd = exec.CommandContext(runCtx, "/bin/zsh", "-lc", command)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	res := strings.TrimSpace(out.String())
	if len(res) > 4000 {
		res = res[:4000] + "\n...[truncated]"
	}
	return res, err
}
