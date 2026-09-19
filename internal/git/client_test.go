package git

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/process"
)

// testRoot is the work tree the scripted git responses report.
const testRoot = "/work/nexus_applicant"

// scriptedRunner answers git invocations from a table and records every
// invocation it receives. An unregistered invocation is an error, so a client
// that runs something unexpected fails the test instead of silently passing.
type scriptedRunner struct {
	results map[string]process.Result
	errs    map[string]error
	calls   []process.Spec
}

func newScriptedRunner() *scriptedRunner {
	return &scriptedRunner{
		results: map[string]process.Result{},
		errs:    map[string]error{},
	}
}

// respond registers the result for the invocation "git " + args.
func (r *scriptedRunner) respond(args string, result process.Result) *scriptedRunner {
	r.results["git "+args] = result
	return r
}

// fail registers an execution error for the invocation "git " + args.
func (r *scriptedRunner) fail(args string, err error) *scriptedRunner {
	r.errs["git "+args] = err
	return r
}

func (r *scriptedRunner) Run(_ context.Context, spec process.Spec) (process.Result, error) {
	r.calls = append(r.calls, spec)

	key := spec.String()
	if err, ok := r.errs[key]; ok {
		return process.Result{}, err
	}
	if result, ok := r.results[key]; ok {
		return result, nil
	}
	return process.Result{}, fmt.Errorf("unexpected invocation: %s", key)
}

// cleanRepository scripts a clean checkout of main that tracks origin/main and
// is two commits ahead of it. Tests override single responses to describe other
// states.
func cleanRepository() *scriptedRunner {
	return noOperation().
		respond("rev-parse --show-toplevel", process.Result{Stdout: testRoot + "\n"}).
		respond("symbolic-ref --short -q HEAD", process.Result{Stdout: "main\n"}).
		respond("rev-parse --abbrev-ref --symbolic-full-name @{upstream}", process.Result{Stdout: "origin/main\n"}).
		respond("rev-list --left-right --count @{upstream}...HEAD", process.Result{Stdout: "0\t2\n"}).
		respond("status --porcelain -z", process.Result{})
}

// noOperation scripts the marker refs as absent, which is what a repository
// with nothing paused in it reports.
func noOperation() *scriptedRunner {
	return newScriptedRunner().
		respond("rev-parse -q --verify REBASE_HEAD", process.Result{ExitCode: 1}).
		respond("rev-parse -q --verify MERGE_HEAD", process.Result{ExitCode: 1}).
		respond("rev-parse -q --verify CHERRY_PICK_HEAD", process.Result{ExitCode: 1}).
		respond("rev-parse -q --verify REVERT_HEAD", process.Result{ExitCode: 1})
}

func TestClientStatusCleanRepository(t *testing.T) {
	runner := cleanRepository()

	status, err := NewClient(runner).Status(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}

	if status.Root != testRoot {
		t.Errorf("Root = %q, want %q", status.Root, testRoot)
	}
	if status.Branch != "main" {
		t.Errorf("Branch = %q, want %q", status.Branch, "main")
	}
	if status.Detached {
		t.Error("Detached = true, want false")
	}
	if !status.HasUpstream() {
		t.Fatal("HasUpstream() = false, want true")
	}
	if status.Upstream != "origin/main" {
		t.Errorf("Upstream = %q, want %q", status.Upstream, "origin/main")
	}
	if status.Ahead != 2 {
		t.Errorf("Ahead = %d, want 2", status.Ahead)
	}
	if status.Behind != 0 {
		t.Errorf("Behind = %d, want 0", status.Behind)
	}
	if len(status.Files) != 0 {
		t.Errorf("Files = %v, want none", status.Files)
	}

	for _, call := range runner.calls {
		if call.Dir != testRoot {
			t.Errorf("invocation %q ran in %q, want %q", call, call.Dir, testRoot)
		}
		if call.Name != "git" {
			t.Errorf("invocation %q used %q, want git", call, call.Name)
		}
	}
}

func TestClientStatusDirtyRepository(t *testing.T) {
	const porcelain = "M  staged.txt\x00" +
		" M unstaged.txt\x00" +
		"?? untracked.txt\x00" +
		"R  renamed.txt\x00original.txt\x00" +
		"UU conflicted.txt\x00"

	runner := cleanRepository().respond("status --porcelain -z", process.Result{Stdout: porcelain})

	status, err := NewClient(runner).Status(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}

	want := []FileChange{
		{Path: "staged.txt", Index: Modified},
		{Path: "unstaged.txt", Worktree: Modified},
		{Path: "untracked.txt", Untracked: true},
		{Path: "renamed.txt", OriginalPath: "original.txt", Index: Renamed},
		{Path: "conflicted.txt", Index: Unmerged, Worktree: Unmerged, Unmerged: true},
	}
	if !reflect.DeepEqual(status.Files, want) {
		t.Errorf("Files = %+v, want %+v", status.Files, want)
	}
}

// The first porcelain entry starts with a space when its change is unstaged, so
// trimming git's output would shift every path by one character.
func TestClientStatusKeepsTheLeadingSpaceOfUnstagedEntries(t *testing.T) {
	runner := cleanRepository().respond(
		"status --porcelain -z",
		process.Result{Stdout: " M README.md\x00A  staged.txt\x00"},
	)

	status, err := NewClient(runner).Status(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}

	want := []FileChange{
		{Path: "README.md", Worktree: Modified},
		{Path: "staged.txt", Index: Added},
	}
	if !reflect.DeepEqual(status.Files, want) {
		t.Errorf("Files = %+v, want %+v", status.Files, want)
	}
}

func TestClientStatusDetachedHead(t *testing.T) {
	runner := noOperation().
		respond("rev-parse --show-toplevel", process.Result{Stdout: testRoot + "\n"}).
		respond("symbolic-ref --short -q HEAD", process.Result{ExitCode: 1}).
		respond("rev-parse --short HEAD", process.Result{Stdout: "a1b2c3d\n"}).
		respond("rev-parse --abbrev-ref --symbolic-full-name @{upstream}", process.Result{
			ExitCode: 128,
			Stderr:   "fatal: HEAD does not point to a branch\n",
		}).
		respond("status --porcelain -z", process.Result{})

	status, err := NewClient(runner).Status(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}

	if !status.Detached {
		t.Error("Detached = false, want true")
	}
	if status.Branch != "" {
		t.Errorf("Branch = %q, want empty for a detached HEAD", status.Branch)
	}
	if status.Head != "a1b2c3d" {
		t.Errorf("Head = %q, want %q", status.Head, "a1b2c3d")
	}
	if status.HasUpstream() {
		t.Errorf("HasUpstream() = true, want false for a detached HEAD")
	}
	if status.Ahead != 0 || status.Behind != 0 {
		t.Errorf("Ahead/Behind = %d/%d, want 0/0 without an upstream", status.Ahead, status.Behind)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call.String(), "git rev-list") {
			t.Errorf("invocation %q should not run without an upstream", call)
		}
	}
}

func TestClientStatusWithoutUpstream(t *testing.T) {
	runner := cleanRepository().respond(
		"rev-parse --abbrev-ref --symbolic-full-name @{upstream}",
		process.Result{ExitCode: 128, Stderr: "fatal: no upstream configured for branch 'main'\n"},
	)

	status, err := NewClient(runner).Status(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}

	if status.HasUpstream() {
		t.Errorf("HasUpstream() = true, want false")
	}
	if status.Branch != "main" {
		t.Errorf("Branch = %q, want %q", status.Branch, "main")
	}
	if status.Ahead != 0 || status.Behind != 0 {
		t.Errorf("Ahead/Behind = %d/%d, want 0/0", status.Ahead, status.Behind)
	}
}

func TestClientStatusNotARepository(t *testing.T) {
	runner := newScriptedRunner().respond("rev-parse --show-toplevel", process.Result{
		ExitCode: 128,
		Stderr:   "fatal: not a git repository (or any of the parent directories): .git\n",
	})

	_, err := NewClient(runner).Status(context.Background(), "/tmp")
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("Status() error = %v, want ErrNotARepository", err)
	}
	if !strings.Contains(err.Error(), "/tmp") {
		t.Errorf("Status() error = %v, want it to name the directory", err)
	}
}

func TestClientStatusReportsGitFailure(t *testing.T) {
	runner := cleanRepository().respond("status --porcelain -z", process.Result{
		ExitCode: 128,
		Stderr:   "fatal: index file corrupt\n",
	})

	_, err := NewClient(runner).Status(context.Background(), testRoot)

	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Status() error = %v, want *CommandError", err)
	}
	if commandErr.ExitCode != 128 {
		t.Errorf("ExitCode = %d, want 128", commandErr.ExitCode)
	}
	if want := "status --porcelain -z"; strings.Join(commandErr.Args, " ") != want {
		t.Errorf("Args = %v, want %q", commandErr.Args, want)
	}
	if !strings.Contains(commandErr.Stderr, "index file corrupt") {
		t.Errorf("Stderr = %q, want it to contain the git diagnostic", commandErr.Stderr)
	}
	if want := "exit status 128"; !strings.Contains(commandErr.Error(), want) {
		t.Errorf("Error() = %q, want it to contain %q", commandErr.Error(), want)
	}
}

func TestClientStatusReportsExecutionFailure(t *testing.T) {
	runner := newScriptedRunner().fail("rev-parse --show-toplevel", fmt.Errorf("git: %w", process.ErrNotFound))

	_, err := NewClient(runner).Status(context.Background(), testRoot)
	if !errors.Is(err, process.ErrNotFound) {
		t.Fatalf("Status() error = %v, want it to wrap process.ErrNotFound", err)
	}
	if errors.Is(err, ErrNotARepository) {
		t.Error("a git executable that cannot be run must not be reported as not-a-repository")
	}
}

func TestClientStatusRejectsUnexpectedRevListOutput(t *testing.T) {
	runner := cleanRepository().respond(
		"rev-list --left-right --count @{upstream}...HEAD",
		process.Result{Stdout: "garbage\n"},
	)

	_, err := NewClient(runner).Status(context.Background(), testRoot)
	if err == nil {
		t.Fatal("Status() error = nil, want a parse error")
	}
	if !strings.Contains(err.Error(), "unexpected git rev-list output") {
		t.Errorf("Status() error = %v, want it to describe the rev-list output", err)
	}
}

func TestClientStatusRejectsMalformedStatusOutput(t *testing.T) {
	runner := cleanRepository().respond("status --porcelain -z", process.Result{Stdout: "X\x00"})

	_, err := NewClient(runner).Status(context.Background(), testRoot)
	if err == nil {
		t.Fatal("Status() error = nil, want a parse error")
	}
	if !strings.Contains(err.Error(), "malformed status entry") {
		t.Errorf("Status() error = %v, want it to describe the malformed entry", err)
	}
}
