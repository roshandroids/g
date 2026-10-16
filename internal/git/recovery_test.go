package git

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/process"
)

func TestClientContinue(t *testing.T) {
	tests := []struct {
		op   Operation
		args string
	}{
		{OperationRebase, "rebase --continue"},
		{OperationMerge, "merge --continue"},
		{OperationCherryPick, "cherry-pick --continue"},
		{OperationRevert, "revert --continue"},
	}

	for _, test := range tests {
		t.Run(string(test.op), func(t *testing.T) {
			runner := newScriptedRunner().respond(
				"-c core.editor=true -c sequence.editor=true "+test.args,
				process.Result{},
			)
			if err := NewClient(runner).Continue(context.Background(), testRoot, test.op); err != nil {
				t.Fatalf("Continue() error = %v, want nil", err)
			}
		})
	}
}

func TestClientAbort(t *testing.T) {
	runner := newScriptedRunner().respond("rebase --abort", process.Result{})
	if err := NewClient(runner).Abort(context.Background(), testRoot, OperationRebase); err != nil {
		t.Fatalf("Abort() error = %v, want nil", err)
	}
}

func TestClientSoftReset(t *testing.T) {
	runner := newScriptedRunner().respond("reset --soft HEAD~2", process.Result{})
	if err := NewClient(runner).SoftReset(context.Background(), testRoot, 2); err != nil {
		t.Fatalf("SoftReset() error = %v, want nil", err)
	}
}

func TestClientSoftResetRejectsZero(t *testing.T) {
	err := NewClient(newScriptedRunner()).SoftReset(context.Background(), testRoot, 0)
	if err == nil {
		t.Fatal("SoftReset(0) error = nil, want rejection")
	}
}

func TestClientRecentCommits(t *testing.T) {
	runner := newScriptedRunner().respond("log -n 2 --format=%h\t%s", process.Result{
		Stdout: "abc1234\tfix: one\ndef5678\tfeat: two\n",
	})

	got, err := NewClient(runner).RecentCommits(context.Background(), testRoot, 2)
	if err != nil {
		t.Fatalf("RecentCommits() error = %v, want nil", err)
	}
	if len(got) != 2 || got[0].Hash != "abc1234" || got[0].Subject != "fix: one" {
		t.Fatalf("RecentCommits() = %+v, want two commits", got)
	}
}

func TestClientGoneBranches(t *testing.T) {
	runner := newScriptedRunner().respond(
		"for-each-ref --format=%(refname:short)%00%(upstream:track) refs/heads",
		process.Result{Stdout: "main\x00\nfeature\x00[gone]\nstale\x00[gone]\n"},
	)

	got, err := NewClient(runner).GoneBranches(context.Background(), testRoot)
	if err != nil {
		t.Fatalf("GoneBranches() error = %v, want nil", err)
	}
	if strings.Join(got, ",") != "feature,stale" {
		t.Errorf("GoneBranches() = %v, want feature,stale", got)
	}
}

func TestClientMergedBranches(t *testing.T) {
	runner := newScriptedRunner().respond(
		"branch --format=%(refname:short) --merged main",
		process.Result{Stdout: "main\ndone\n"},
	)

	got, err := NewClient(runner).MergedBranches(context.Background(), testRoot, "main")
	if err != nil {
		t.Fatalf("MergedBranches() error = %v, want nil", err)
	}
	if strings.Join(got, ",") != "done,main" {
		t.Errorf("MergedBranches() = %v, want done,main", got)
	}
}

func TestClientDeleteBranch(t *testing.T) {
	runner := newScriptedRunner().respond("branch -d stale", process.Result{})
	if err := NewClient(runner).DeleteBranch(context.Background(), testRoot, "stale", false); err != nil {
		t.Fatalf("DeleteBranch() error = %v, want nil", err)
	}
}

func TestClientDeleteBranchForce(t *testing.T) {
	runner := newScriptedRunner().respond("branch -D stale", process.Result{})
	if err := NewClient(runner).DeleteBranch(context.Background(), testRoot, "stale", true); err != nil {
		t.Fatalf("DeleteBranch(force) error = %v, want nil", err)
	}
}

func TestClientContinueReportsGitFailure(t *testing.T) {
	runner := newScriptedRunner().respond(
		"-c core.editor=true -c sequence.editor=true rebase --continue",
		process.Result{
			ExitCode: 1,
			Stderr:   "error: you have unmerged paths\n",
		},
	)

	err := NewClient(runner).Continue(context.Background(), testRoot, OperationRebase)
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("Continue() error = %v, want CommandError", err)
	}
}
