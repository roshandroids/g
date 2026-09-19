package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/process"
)

func TestClientLocalBranches(t *testing.T) {
	runner := newScriptedRunner().respond(
		"for-each-ref --format=%(refname:short) refs/heads",
		process.Result{Stdout: "main\nfeature/x\nHCM-1-work\n\n"},
	)

	got, err := NewClient(runner).LocalBranches(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("LocalBranches() error = %v, want nil", err)
	}

	want := []string{"HCM-1-work", "feature/x", "main"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LocalBranches() = %v, want %v", got, want)
	}
}

func TestClientLocalBranchesEmptyRepository(t *testing.T) {
	runner := newScriptedRunner().respond(
		"for-each-ref --format=%(refname:short) refs/heads",
		process.Result{},
	)

	got, err := NewClient(runner).LocalBranches(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("LocalBranches() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("LocalBranches() = %v, want no branches", got)
	}
}

func TestClientRemotes(t *testing.T) {
	runner := newScriptedRunner().respond("remote", process.Result{Stdout: "origin\nupstream\n"})

	got, err := NewClient(runner).Remotes(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Remotes() error = %v, want nil", err)
	}

	want := []string{"origin", "upstream"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Remotes() = %v, want %v", got, want)
	}
}

func TestClientRemotesWhenNoneConfigured(t *testing.T) {
	runner := newScriptedRunner().respond("remote", process.Result{Stdout: "\n"})

	got, err := NewClient(runner).Remotes(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Remotes() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("Remotes() = %v, want none", got)
	}
}

func TestClientRemoteBranchExists(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		want   bool
	}{
		{name: "present", stdout: "origin/HCM-1-work\n", want: true},
		{name: "absent", stdout: "", want: false},
		{name: "blank", stdout: "\n", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newScriptedRunner().respond(
				"for-each-ref --format=%(refname:short) refs/remotes/origin/HCM-1-work",
				process.Result{Stdout: test.stdout},
			)

			got, err := NewClient(runner).RemoteBranchExists(context.Background(), testRoot, "origin", "HCM-1-work")
			if err != nil {
				t.Fatalf("RemoteBranchExists() error = %v, want nil", err)
			}
			if got != test.want {
				t.Errorf("RemoteBranchExists() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestClientCreateBranch(t *testing.T) {
	runner := newScriptedRunner().respond("switch -c HCM-1-work main", process.Result{})

	err := NewClient(runner).CreateBranch(context.Background(), testRoot, "HCM-1-work", "main")
	if err != nil {
		t.Fatalf("CreateBranch() error = %v, want nil", err)
	}

	want := "git switch -c HCM-1-work main"
	if len(runner.calls) != 1 || runner.calls[0].String() != want {
		t.Errorf("invocations = %v, want exactly %q", runner.calls, want)
	}
	if dir := runner.calls[0].Dir; dir != testRoot {
		t.Errorf("invocation ran in %q, want %q", dir, testRoot)
	}
}

func TestClientCreateBranchReportsGitFailure(t *testing.T) {
	runner := newScriptedRunner().respond("switch -c HCM-1-work nope", process.Result{
		ExitCode: 128,
		Stderr:   "fatal: invalid reference: nope\n",
	})

	err := NewClient(runner).CreateBranch(context.Background(), testRoot, "HCM-1-work", "nope")

	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("CreateBranch() error = %v, want *CommandError", err)
	}
	if !strings.Contains(commandErr.Stderr, "invalid reference") {
		t.Errorf("Stderr = %q, want it to keep git's diagnostic", commandErr.Stderr)
	}
}

func TestClientSwitchBranch(t *testing.T) {
	runner := newScriptedRunner().respond("switch feature/x", process.Result{})

	err := NewClient(runner).SwitchBranch(context.Background(), testRoot, "feature/x")
	if err != nil {
		t.Fatalf("SwitchBranch() error = %v, want nil", err)
	}

	want := "git switch feature/x"
	if len(runner.calls) != 1 || runner.calls[0].String() != want {
		t.Errorf("invocations = %v, want exactly %q", runner.calls, want)
	}
}

// A failed switch is how git reports that switching would overwrite local
// changes, so the diagnostic has to survive.
func TestClientSwitchBranchReportsDirtyTreeRefusal(t *testing.T) {
	runner := newScriptedRunner().respond("switch main", process.Result{
		ExitCode: 1,
		Stderr:   "error: Your local changes to the following files would be overwritten by checkout:\n\tREADME.md\n",
	})

	err := NewClient(runner).SwitchBranch(context.Background(), testRoot, "main")
	if err == nil {
		t.Fatal("SwitchBranch() error = nil, want git's refusal")
	}
	if !strings.Contains(err.Error(), "would be overwritten") {
		t.Errorf("SwitchBranch() error = %v, want it to keep git's explanation", err)
	}
}

func TestClientOperation(t *testing.T) {
	t.Run("rebase", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".git", "rebase-merge"), 0o700); err != nil {
			t.Fatal(err)
		}
		runner := noOperation()
		got, err := NewClient(runner).operation(context.Background(), dir)
		if err != nil {
			t.Fatalf("Operation() error = %v, want nil", err)
		}
		if got != OperationRebase {
			t.Errorf("Operation() = %q, want %q", got, OperationRebase)
		}
	})

	for _, test := range []struct {
		name   string
		marker string
		want   Operation
	}{
		{name: "merge", marker: "MERGE_HEAD", want: OperationMerge},
		{name: "cherry-pick", marker: "CHERRY_PICK_HEAD", want: OperationCherryPick},
		{name: "revert", marker: "REVERT_HEAD", want: OperationRevert},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := noOperation().respond(
				"rev-parse -q --verify "+test.marker,
				process.Result{Stdout: "0123456789abcdef\n"},
			)

			got, err := NewClient(runner).operation(context.Background(), testRoot)
			if err != nil {
				t.Fatalf("Operation() error = %v, want nil", err)
			}
			if got != test.want {
				t.Errorf("Operation() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestClientOperationReportsNone(t *testing.T) {
	runner := noOperation()

	got, err := NewClient(runner).operation(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Operation() error = %v, want nil", err)
	}
	if got != OperationNone {
		t.Errorf("Operation() = %q, want %q", got, OperationNone)
	}
}

// During a rebase of a merge commit git leaves both a rebase state and MERGE_HEAD.
// That known overlap resolves as a rebase; other overlaps are ambiguous.
func TestClientOperationPrefersRebaseOverMergeMarker(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "rebase-merge"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := noOperation().respond(
		"rev-parse -q --verify MERGE_HEAD",
		process.Result{Stdout: "bbb\n"},
	)

	got, err := NewClient(runner).operation(context.Background(), dir)
	if err != nil {
		t.Fatalf("Operation() error = %v, want nil", err)
	}
	if got != OperationRebase {
		t.Errorf("Operation() = %q, want %q", got, OperationRebase)
	}
}

func TestClientOperationReportsAmbiguousMarkers(t *testing.T) {
	runner := noOperation().
		respond("rev-parse -q --verify MERGE_HEAD", process.Result{Stdout: "aaa\n"}).
		respond("rev-parse -q --verify CHERRY_PICK_HEAD", process.Result{Stdout: "bbb\n"})

	_, err := NewClient(runner).operation(context.Background(), testRoot)
	var ambiguous *AmbiguousOperationError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("Operation() error = %v, want AmbiguousOperationError", err)
	}
}

func TestClientOperationReportsExecutionFailure(t *testing.T) {
	runner := newScriptedRunner().fail(
		"rev-parse --git-path rebase-merge",
		errors.New("git: executable not found"),
	)

	_, err := NewClient(runner).operation(context.Background(), testRoot)
	if err == nil {
		t.Fatal("Operation() error = nil, want the execution failure")
	}
	if !strings.Contains(err.Error(), "executable not found") {
		t.Errorf("Operation() error = %v, want it to keep the cause", err)
	}
}

func TestClientStatusReportsOperationInProgress(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "rebase-merge"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := cleanRepository().
		respond("rev-parse --show-toplevel", process.Result{Stdout: dir + "\n"})

	status, err := NewClient(runner).Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}
	if status.Operation != OperationRebase {
		t.Errorf("Operation = %q, want %q", status.Operation, OperationRebase)
	}
}

func TestStatusClean(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "no changes", status: Status{}, want: true},
		{name: "staged", status: Status{Files: []FileChange{{Path: "a", Index: Added}}}, want: false},
		{name: "untracked", status: Status{Files: []FileChange{{Path: "a", Untracked: true}}}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.status.Clean(); got != test.want {
				t.Errorf("Clean() = %t, want %t", got, test.want)
			}
		})
	}
}
