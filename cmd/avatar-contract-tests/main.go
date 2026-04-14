package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"go-avatar-service/tests/contract"
)

func main() {
	var cfg contract.Config
	flag.StringVar(&cfg.BaseURL, "base-url", "", "service base URL; falls back to BASE_URL")
	flag.DurationVar(&cfg.Timeout, "timeout", contract.DefaultTimeout, "overall test timeout")
	flag.StringVar(&cfg.UserID, "user-id", contract.DefaultUserID, "base user id for generated test data")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "print scenario durations")
	flag.Parse()
	cfg.Out = os.Stdout

	runner, err := contract.NewRunner(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	report := runner.Run(ctx, nil)
	fmt.Fprintf(os.Stdout, "\n%d passed, %d failed\n", report.Passed(), report.Failed())
	if report.Failed() > 0 {
		os.Exit(1)
	}
}
