package runner

import (
	"github.com/charlesfeval/brown/internal/checks"
)

// RunAll executes every check in the registry in order and collects results.
func RunAll(registry *checks.Registry) []checks.Result {
	return RunAllWithCallback(registry, nil)
}

// RunAllWithCallback executes checks in order and calls callback after each
// check completes.
func RunAllWithCallback(registry *checks.Registry, callback func(checks.Result)) []checks.Result {
	var results []checks.Result
	for _, c := range registry.All() {
		result := c.Run()
		results = append(results, result)
		if callback != nil {
			callback(result)
		}
	}
	return results
}

// HasFailures returns true if any result has a Fail status.
func HasFailures(results []checks.Result) bool {
	for _, r := range results {
		if r.Status == checks.Fail {
			return true
		}
	}
	return false
}
