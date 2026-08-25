package output

import (
	"fmt"

	"github.com/charlesfeval/brown/internal/checks"
)

var statusIcon = map[checks.Status]string{
	checks.OK:   "✓",
	checks.Warn: "⚠",
	checks.Fail: "✗",
	checks.Skip: "—",
}

// Print renders check results to stdout.
func Print(results []checks.Result) {
	for _, r := range results {
		PrintResult(r)
	}
	fmt.Println()
}

// PrintResult renders one completed check result to stdout.
func PrintResult(r checks.Result) {
	icon := statusIcon[r.Status]
	fmt.Printf("  %s  %-22s %s\n", icon, r.Name, r.Message)
	for _, d := range r.Details {
		fmt.Printf("       %s\n", d)
	}
}
