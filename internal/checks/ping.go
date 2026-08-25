package checks

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// PingCheck verifies basic internet reachability by pinging an external host.
type PingCheck struct{}

func (c *PingCheck) Name() string { return "Internet Reachability" }

func (c *PingCheck) Run() Result {
	target := "8.8.8.8"

	// Build a platform-appropriate ping: 3 packets, 5s deadline.
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"-c", "3", "-t", "5", target}
	default: // linux and other unix
		args = []string{"-c", "3", "-W", "5", target}
	}

	start := time.Now()
	out, err := exec.Command("ping", args...).CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		return Result{Name: c.Name(), Status: Fail, Message: fmt.Sprintf("ping %s failed: %v", target, err)}
	}

	// Extract the summary line (usually the last non-empty line containing "avg").
	summary := extractPingSummary(string(out))
	return Result{
		Name:    c.Name(),
		Status:  OK,
		Message: fmt.Sprintf("%s reachable (%s) — %s", target, elapsed.Round(time.Millisecond), summary),
	}
}

func extractPingSummary(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "avg") {
			return strings.TrimSpace(line)
		}
	}
	return "no summary"
}
