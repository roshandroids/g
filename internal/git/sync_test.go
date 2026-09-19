package git

import (
	"context"
	"errors"
	"testing"

	"github.com/roshandroids/g/internal/process"
)

func TestClientFetch(t *testing.T) {
	runner := newScriptedRunner().respond("fetch --prune origin", process.Result{})
	if err := NewClient(runner).Fetch(context.Background(), testRoot, "origin"); err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
}

func TestClientRebaseConflict(t *testing.T) {
	runner := newScriptedRunner().
		respond("rebase main", process.Result{ExitCode: 1, Stderr: "conflict\n"}).
		respond("rev-parse -q --verify REBASE_HEAD", process.Result{Stdout: "abc\n"}).
		respond("rev-parse -q --verify MERGE_HEAD", process.Result{ExitCode: 1}).
		respond("rev-parse -q --verify CHERRY_PICK_HEAD", process.Result{ExitCode: 1}).
		respond("rev-parse -q --verify REVERT_HEAD", process.Result{ExitCode: 1})

	err := NewClient(runner).Rebase(context.Background(), testRoot, "main")
	var conflict *RebaseConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Rebase() error = %v, want RebaseConflictError", err)
	}
}

func TestClientAheadBehind(t *testing.T) {
	runner := newScriptedRunner().
		respond("rev-list --left-right --count main...feature", process.Result{Stdout: "2\t3\n"})

	ahead, behind, err := NewClient(runner).AheadBehind(context.Background(), testRoot, "main", "feature")
	if err != nil {
		t.Fatalf("AheadBehind() error = %v, want nil", err)
	}
	if ahead != 3 || behind != 2 {
		t.Errorf("AheadBehind() = %d/%d, want 3/2", ahead, behind)
	}
}

func TestClientRemoteDefaultBranch(t *testing.T) {
	runner := newScriptedRunner().
		respond("symbolic-ref --short refs/remotes/origin/HEAD", process.Result{Stdout: "origin/main\n"})

	got, err := NewClient(runner).RemoteDefaultBranch(context.Background(), testRoot, "origin")
	if err != nil {
		t.Fatalf("RemoteDefaultBranch() error = %v, want nil", err)
	}
	if got != "main" {
		t.Errorf("RemoteDefaultBranch() = %q, want main", got)
	}
}
