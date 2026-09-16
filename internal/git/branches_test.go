package git

import (
	"context"
	"errors"
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
	tests := []struct {
		name   string
		marker string
		want   Operation
	}{
		{name: "rebase", marker: "REBASE_HEAD", want: OperationRebase},
		{name: "merge", marker: "MERGE_HEAD", want: OperationMerge},
		{name: "cherry-pick", marker: "CHERRY_PICK_HEAD", want: OperationCherryPick},
		{name: "revert", marker: "REVERT_HEAD", want: OperationRevert},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Every earlier marker in the precedence order has to be scripted
			// as absent, or the runner reports the invocation as unexpected.
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

// During a rebase git keeps other marker refs around, so the check has to stop
// at the first match in precedence order rather than reporting whichever it
// happens to find.
func TestClientOperationPrefersRebaseOverOtherMarkers(t *testing.T) {
	runner := newScriptedRunner().
		respond("rev-parse -q --verify REBASE_HEAD", process.Result{Stdout: "aaa\n"}).
		respond("rev-parse -q --verify MERGE_HEAD", process.Result{Stdout: "bbb\n"})

	got, err := NewClient(runner).operation(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("Operation() error = %v, want nil", err)
	}
	if got != OperationRebase {
		t.Errorf("Operation() = %q, want %q", got, OperationRebase)
	}
	if len(runner.calls) != 1 {
		t.Errorf("Operation() made %d invocations, want it to stop at the first match", len(runner.calls))
	}
}

func TestClientOperationReportsExecutionFailure(t *testing.T) {
	runner := newScriptedRunner().fail(
		"rev-parse -q --verify REBASE_HEAD",
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
	runner := cleanRepository().respond(
		"rev-parse -q --verify REBASE_HEAD",
		process.Result{Stdout: "0123456789abcdef\n"},
	)

	status, err := NewClient(runner).Status(context.Background(), testRoot)
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
