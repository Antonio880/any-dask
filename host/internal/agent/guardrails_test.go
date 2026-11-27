package agent

import "testing"

func TestGuardrails(t *testing.T) {
	cases := []struct {
		cmd   string
		risky bool
	}{
		{"ls -la", false},
		{"git status", false},
		{"rm -rf /tmp/project", true},
		{"Remove-Item -Recurse -Force C:\\temp", true},
		{"sudo apt update", true},
		{"git push origin main --force", true},
	}
	for _, tc := range cases {
		got, _ := CheckGuardrail(Action{Name: "shell_exec", Command: tc.cmd})
		if got != tc.risky {
			t.Errorf("cmd %q: expected risky=%v, got %v", tc.cmd, tc.risky, got)
		}
	}
}
