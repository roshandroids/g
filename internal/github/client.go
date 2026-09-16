// Package github is the architectural boundary for GitHub operations performed
// through the GitHub CLI (`gh`).
//
// Basic Git functionality must never depend on this package; callers that use
// it check Available first and fall back to plain Git when `gh` is missing.
package github

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/roshandroids/g/internal/process"
)

// executable is the GitHub CLI binary.
const executable = "gh"

// Client exposes GitHub operations to the application layer.
type Client interface {
	// Available reports whether the GitHub CLI is installed and usable.
	Available(ctx context.Context) (bool, error)
}

// ExecClient implements Client by invoking the `gh` executable.
type ExecClient struct {
	runner process.Runner
}

// NewExecClient returns a Client that runs `gh` through runner.
func NewExecClient(runner process.Runner) *ExecClient {
	return &ExecClient{runner: runner}
}

// Available implements Client.
//
// A missing or unauthenticated CLI is reported as unavailable rather than as an
// error, so that Git-only workflows keep working. An error means the check
// itself could not be carried out.
func (c *ExecClient) Available(ctx context.Context) (bool, error) {
	result, err := c.runner.Run(ctx, process.Spec{Name: executable, Args: []string{"--version"}})
	if errors.Is(err, process.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking for %s: %w", executable, err)
	}
	if result.Failed() {
		return false, fmt.Errorf("%s --version: exit status %d: %s", executable, result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return true, nil
}
