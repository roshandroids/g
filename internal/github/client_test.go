package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/process"
)

// fakeRunner answers the single invocation the client makes.
type fakeRunner struct {
	result process.Result
	err    error
	calls  []process.Spec
}

func (f *fakeRunner) Run(_ context.Context, spec process.Spec) (process.Result, error) {
	f.calls = append(f.calls, spec)
	if f.err != nil {
		return process.Result{}, f.err
	}
	return f.result, nil
}

func TestAvailableDetectsGitHubCLI(t *testing.T) {
	runner := &fakeRunner{result: process.Result{Stdout: "gh version 2.62.0\n"}}

	available, err := NewExecClient(runner).Available(context.Background())
	if err != nil {
		t.Fatalf("Available() error = %v, want nil", err)
	}
	if !available {
		t.Error("Available() = false, want true")
	}

	if len(runner.calls) != 1 {
		t.Fatalf("made %d invocations, want 1", len(runner.calls))
	}
	call := runner.calls[0]
	if call.Name != "gh" {
		t.Errorf("invoked %q, want gh", call.Name)
	}
	if want := []string{"--version"}; len(call.Args) != 1 || call.Args[0] != want[0] {
		t.Errorf("args = %v, want %v", call.Args, want)
	}
}

func TestAvailableIsFalseWhenGitHubCLIIsMissing(t *testing.T) {
	runner := &fakeRunner{err: fmt.Errorf("gh: %w", process.ErrNotFound)}

	available, err := NewExecClient(runner).Available(context.Background())
	if err != nil {
		t.Fatalf("Available() error = %v, want nil so Git-only workflows keep working", err)
	}
	if available {
		t.Error("Available() = true, want false")
	}
}

func TestAvailableReportsFailingGitHubCLI(t *testing.T) {
	runner := &fakeRunner{result: process.Result{ExitCode: 1, Stderr: "not logged in\n"}}

	available, err := NewExecClient(runner).Available(context.Background())
	if available {
		t.Error("Available() = true, want false")
	}
	if err == nil {
		t.Fatal("Available() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("Available() error = %v, want it to keep the gh diagnostic", err)
	}
}

func TestAvailableReportsRunnerFailure(t *testing.T) {
	want := errors.New("io failure")
	runner := &fakeRunner{err: want}

	available, err := NewExecClient(runner).Available(context.Background())
	if available {
		t.Error("Available() = true, want false")
	}
	if !errors.Is(err, want) {
		t.Errorf("Available() error = %v, want it to wrap %v", err, want)
	}
}
