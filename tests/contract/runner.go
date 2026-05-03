// Package contract provides black-box HTTP smoke tests for the avatar API.
package contract

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// DefaultTimeout is the fallback timeout used by the contract runner.
	DefaultTimeout = 30 * time.Second
	// DefaultUserID is the default base user identifier used in scenarios.
	DefaultUserID = "contract-user"
)

// Config configures the contract runner.
type Config struct {
	BaseURL string
	Timeout time.Duration
	UserID  string
	Verbose bool
	Out     io.Writer
	Client  *http.Client
}

// Runner executes HTTP smoke scenarios against a running service.
type Runner struct {
	baseURL string
	timeout time.Duration
	userID  string
	verbose bool
	out     io.Writer
	client  *http.Client
	state   state
}

type state struct {
	AvatarID string
}

// Scenario describes one black-box contract check.
type Scenario struct {
	Name string
	Run  func(context.Context, *Runner) error
}

// Result captures the outcome of a single scenario run.
type Result struct {
	Name     string
	Passed   bool
	Duration time.Duration
	Err      error
}

// Report aggregates all scenario results.
type Report struct {
	Results []Result
}

// NewRunner validates Config and creates a Runner.
func NewRunner(cfg Config) (*Runner, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("BASE_URL")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required via -base-url or BASE_URL")
	}

	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("base URL must use http or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""

	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.UserID == "" {
		cfg.UserID = DefaultUserID
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: cfg.Timeout}
	}

	return &Runner{
		baseURL: parsed.String(),
		timeout: cfg.Timeout,
		userID:  cfg.UserID,
		verbose: cfg.Verbose,
		out:     cfg.Out,
		client:  cfg.Client,
	}, nil
}

// DefaultScenarios returns the standard smoke test suite for the HTTP API.
func DefaultScenarios() []Scenario {
	return []Scenario{
		{Name: "health", Run: scenarioHealth},
		{Name: "web upload page", Run: scenarioWebUpload},
		{Name: "web gallery invalid user", Run: scenarioWebGalleryInvalidUser},
		{Name: "upload requires X-User-ID", Run: scenarioUploadMissingUser},
		{Name: "upload requires multipart field file", Run: scenarioUploadWrongField},
		{Name: "upload rejects invalid image bytes", Run: scenarioUploadInvalidImage},
		{Name: "upload rejects oversized file", Run: scenarioUploadTooLarge},
		{Name: "upload accepts valid original", Run: scenarioUploadValid},
		{Name: "metadata exposes active avatar", Run: scenarioMetadata},
		{Name: "list exposes user avatars", Run: scenarioList},
		{Name: "read original by avatar id", Run: scenarioReadOriginal},
		{Name: "read rejects unsupported size", Run: scenarioUnsupportedSize},
		{Name: "read rejects unsupported format", Run: scenarioUnsupportedFormat},
		{Name: "read thumbnail contract", Run: scenarioReadThumbnail},
		{Name: "read current user avatar", Run: scenarioReadUserAvatar},
		{Name: "delete requires owner", Run: scenarioDeleteRequiresOwner},
		{Name: "delete rejects wrong owner", Run: scenarioDeleteWrongOwner},
		{Name: "delete hides avatar", Run: scenarioDeleteOwner},
		{Name: "delete current user avatar", Run: scenarioDeleteCurrentUserAvatar},
	}
}

// Run executes scenarios and prints progress to the configured writer.
func (r *Runner) Run(ctx context.Context, scenarios []Scenario) Report {
	if len(scenarios) == 0 {
		scenarios = DefaultScenarios()
	}

	report := Report{Results: make([]Result, 0, len(scenarios))}
	for _, scenario := range scenarios {
		start := time.Now()
		err := scenario.Run(ctx, r)
		result := Result{
			Name:     scenario.Name,
			Passed:   err == nil,
			Duration: time.Since(start),
			Err:      err,
		}
		report.Results = append(report.Results, result)
		r.printResult(result)
	}

	return report
}

// URL joins path with the configured service base URL.
func (r *Runner) URL(path string) string {
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	return r.baseURL + path
}

// UserID returns the configured base user ID with an optional suffix.
func (r *Runner) UserID(suffix string) string {
	base := strings.TrimSpace(r.userID)
	if suffix == "" {
		return base
	}
	return base + "-" + suffix
}

func (r *Runner) printResult(result Result) {
	if r.out == nil {
		return
	}
	status := "PASS"
	if !result.Passed {
		status = "FAIL"
	}
	if result.Passed && !r.verbose {
		fmt.Fprintf(r.out, "%s %s\n", status, result.Name)
		return
	}
	if result.Err != nil {
		fmt.Fprintf(r.out, "%s %s (%s): %v\n", status, result.Name, result.Duration.Round(time.Millisecond), result.Err)
		return
	}
	fmt.Fprintf(r.out, "%s %s (%s)\n", status, result.Name, result.Duration.Round(time.Millisecond))
}

// Failed returns the number of failed scenario results.
func (r Report) Failed() int {
	failed := 0
	for _, result := range r.Results {
		if !result.Passed {
			failed++
		}
	}
	return failed
}

// Passed returns the number of successful scenario results.
func (r Report) Passed() int {
	return len(r.Results) - r.Failed()
}
