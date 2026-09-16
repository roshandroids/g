// Package process is the single seam through which g executes external
// programs.
//
// Every Git and GitHub interaction goes through a Runner, which keeps the
// workflow logic testable without spawning real processes.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strings"
)

// ErrNotFound reports that an executable could not be located on PATH.
var ErrNotFound = errors.New("executable not found")

// Spec describes a single external command invocation.
type Spec struct {
	// Dir is the working directory of the command. Empty inherits the working
	// directory of g.
	Dir string
	// Name is the executable to run.
	Name string
	// Args are the arguments passed to the executable.
	Args []string
}

// String renders the invocation for error messages and test fixtures.
func (s Spec) String() string {
	return strings.Join(append([]string{s.Name}, s.Args...), " ")
}

// Result is the captured outcome of a process that ran to completion.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Failed reports whether the process exited with a non-zero status.
func (r Result) Failed() bool { return r.ExitCode != 0 }

// Runner executes external programs.
//
// Implementations report an error only when the process could not be started or
// observed. A process that runs and exits with a non-zero status is described
// by Result instead, leaving it to the caller to decide what a failure means.
type Runner interface {
	Run(ctx context.Context, spec Spec) (Result, error)
}

// ExecRunner runs real processes with os/exec.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, spec Spec) (Result, error) {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	switch {
	case err == nil:
		return result, nil
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return result, fmt.Errorf("%s: %w", spec.Name, ErrNotFound)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return result, fmt.Errorf("running %s: %w", spec, err)
}
