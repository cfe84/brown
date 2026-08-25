package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charlesfeval/brown/internal/checks"
	"github.com/charlesfeval/brown/internal/output"
	"github.com/charlesfeval/brown/internal/runner"
)

func main() {
	diagnose := flag.Bool("diagnose-connectivity", false, "run the gateway versus internet connectivity test")
	monitor := flag.Bool("monitor", false, "continuously monitor gateway and internet connectivity")
	exitOnError := flag.Bool("exit-on-error", false, "stop monitoring when a connectivity problem is detected")
	report := flag.String("report", "", "write the monitor report to an HTML file")
	flag.Parse()

	if *report != "" && !*monitor {
		fmt.Fprintln(os.Stderr, "brown: --report requires --monitor")
		os.Exit(2)
	}
	if *monitor {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := checks.Monitor(ctx, os.Stdout, *exitOnError, *report); err != nil {
			fmt.Fprintln(os.Stderr, "brown:", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("brown — network diagnostic tool")
	fmt.Println()

	registry := checks.DefaultRegistry()
	if *diagnose {
		registry = checks.ConnectivityRegistry()
	}
	results := runner.RunAllWithCallback(registry, output.PrintResult)
	fmt.Println()

	if runner.HasFailures(results) {
		os.Exit(1)
	}
}
