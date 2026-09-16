package process

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpecString(t *testing.T) {
	spec := Spec{Name: "git", Args: []string{"status", "--porcelain"}}
	if got, want := spec.String(), "git status --porcelain"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestExecRunnerCapturesStreamsAndExitCode(t *testing.T) {
	result, err := ExecRunner{}.Run(context.Background(), Spec{
		Name: "sh",
		Args: []string{"-c", "printf out; printf err >&2"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Stdout != "out" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "out")
	}
	if result.Stderr != "err" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "err")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Failed() {
		t.Error("Failed() = true, want false")
	}
}

func TestExecRunnerReportsFailureWithoutError(t *testing.T) {
	result, err := ExecRunner{}.Run(context.Background(), Spec{Name: "sh", Args: []string{"-c", "exit 3"}})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil so callers inspect the result", err)
	}
	if result.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", result.ExitCode)
	}
	if !result.Failed() {
		t.Error("Failed() = false, want true")
	}
}

func TestExecRunnerReportsMissingExecutable(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), Spec{Name: "g-missing-executable"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Run() error = %v, want ErrNotFound", err)
	}
}

func TestExecRunnerUsesWorkingDirectory(t *testing.T) {
	dir := t.TempDir()

	result, err := ExecRunner{}.Run(context.Background(), Spec{Dir: dir, Name: "pwd"})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := resolve(t, strings.TrimSpace(result.Stdout)), resolve(t, dir); got != want {
		t.Errorf("working directory = %q, want %q", got, want)
	}
}

func TestExecRunnerReportsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ExecRunner{}.Run(ctx, Spec{Name: "sh", Args: []string{"-c", "sleep 5"}})
	if err == nil {
		t.Fatal("Run() error = nil, want an error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run() error = %v, want it to wrap context.Canceled", err)
	}
}

// resolve follows symlinks so that temporary directories compare equal on
// platforms where /tmp is a symlink.
func resolve(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolving %q: %v", path, err)
	}
	return resolved
}
