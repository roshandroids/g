package git

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/process"
)

func TestClientCommit(t *testing.T) {
	runner := newScriptedRunner().respond("commit -m fix: resolve the thing", process.Result{})

	err := NewClient(runner).Commit(context.Background(), testRoot, "fix: resolve the thing")
	if err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("Commit() made %d invocations, want 1", len(runner.calls))
	}

	call := runner.calls[0]
	if call.Name != "git" {
		t.Errorf("executable = %q, want git", call.Name)
	}
	if call.Dir != testRoot {
		t.Errorf("directory = %q, want %q", call.Dir, testRoot)
	}
	if want := "commit\x00-m\x00fix: resolve the thing"; strings.Join(call.Args, "\x00") != want {
		t.Errorf("args = %q, want %q", strings.Join(call.Args, " "), want)
	}
}

// A message is passed as one argv entry, so punctuation and spaces in it cannot
// be reinterpreted by a shell or split into extra arguments.
func TestClientCommitPassesTheMessageAsOneArgument(t *testing.T) {
	const message = "fix: don't split; this has $VAR and \"quotes\""

	runner := newScriptedRunner().respond("commit -m "+message, process.Result{})

	if err := NewClient(runner).Commit(context.Background(), testRoot, message); err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("Commit() made %d invocations, want 1", len(runner.calls))
	}
	args := runner.calls[0].Args
	if len(args) != 3 {
		t.Fatalf("args = %q, want exactly three arguments", args)
	}
	if args[2] != message {
		t.Errorf("message argument = %q, want %q", args[2], message)
	}
}

func TestClientCommitReportsGitFailure(t *testing.T) {
	runner := newScriptedRunner().respond("commit -m x", process.Result{
		ExitCode: 1,
		Stderr:   "fatal: nothing to commit, working tree clean\n",
	})

	err := NewClient(runner).Commit(context.Background(), testRoot, "x")

	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Commit() error = %v, want *CommandError", err)
	}
	if !strings.Contains(commandErr.Stderr, "nothing to commit") {
		t.Errorf("Stderr = %q, want it to keep git's diagnostic", commandErr.Stderr)
	}
}

// A commit is never forced through: no --allow-empty and no --no-verify, so a
// clean tree and a failing hook both still stop it.
func TestClientCommitDoesNotBypassGitsChecks(t *testing.T) {
	runner := newScriptedRunner().respond("commit -m x", process.Result{})

	if err := NewClient(runner).Commit(context.Background(), testRoot, "x"); err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}

	for _, arg := range runner.calls[0].Args {
		switch arg {
		case "--allow-empty", "--no-verify", "-n", "--amend", "--no-gpg-sign":
			t.Errorf("Commit() passed %q, want git's own behaviour preserved", arg)
		}
	}
}
