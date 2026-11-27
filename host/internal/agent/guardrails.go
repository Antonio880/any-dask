package agent

import (
	"regexp"
	"strings"
)

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-[a-z]*r[a-z]*f`),
	regexp.MustCompile(`(?i)\bremove-item\b.*-recurse`),
	regexp.MustCompile(`(?i)\bdel\s+/[sqf]`),
	regexp.MustCompile(`(?i)\brmdir\s+/s`),
	regexp.MustCompile(`(?i)\bformat\s+[a-z]:`),
	regexp.MustCompile(`(?i)\bsudo\b`),
	regexp.MustCompile(`(?i)\bshutdown\b|\breboot\b|\brestart-computer\b|\bstop-computer\b`),
	regexp.MustCompile(`(?i)\bgit\s+push\b.*(--force|-f)\b`),
	regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`),
	regexp.MustCompile(`(?i)\bdrop\s+(database|table)\b`),
	regexp.MustCompile(`(?i)\bkillall\b|\btaskkill\s+/f`),
}

// CheckGuardrail returns (needsConfirm, reason) if an action could be destructive.
func CheckGuardrail(act Action) (bool, string) {
	if act.Name == "shell_exec" {
		cmd := strings.TrimSpace(act.Command)
		for _, re := range dangerousPatterns {
			if re.MatchString(cmd) {
				return true, "Comando shell potencialmente destrutivo ou privilegiado"
			}
		}
	}
	return false, ""
}
