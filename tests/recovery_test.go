package tests

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/process"
	"github.com/roshandroids/g/internal/workflow"
)

func recoveryService() *workflow.Service {
	return workflow.NewService(git.NewClient(process.ExecRunner{}), workflow.Options{})
}

func TestContinueNoOperation(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")

	_, err := recoveryService().Continue(context.Background(), dir)
	if !errors.Is(err, workflow.ErrNoOperation) {
		t.Fatalf("Continue() error = %v, want ErrNoOperation", err)
	}
}

func TestContinueAfterResolvingRebase(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	startConflictingRebase(t, dir)

	writeFile(t, dir, "README.md", "resolved\n")
	runGit(t, dir, "add", "README.md")

	result, err := recoveryService().Continue(context.Background(), dir)
	if err != nil {
		t.Fatalf("Continue() error = %v, want nil", err)
	}
	if result.Operation != git.OperationRebase {
		t.Errorf("Operation = %q, want rebase", result.Operation)
	}
	if statusOf(t, dir).Operation != git.OperationNone {
		t.Error("operation still in progress after continue")
	}
}

func TestContinueFailurePreservesState(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	startConflictingRebase(t, dir)

	_, err := recoveryService().Continue(context.Background(), dir)
	if err == nil {
		t.Fatal("Continue() error = nil, want conflict failure")
	}
	if statusOf(t, dir).Operation != git.OperationRebase {
		t.Error("rebase should still be in progress after failed continue")
	}
}

func TestAbortRebase(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	startConflictingRebase(t, dir)

	result, err := recoveryService().Abort(context.Background(), dir)
	if err != nil {
		t.Fatalf("Abort() error = %v, want nil", err)
	}
	if result.Operation != git.OperationRebase {
		t.Errorf("Operation = %q, want rebase", result.Operation)
	}
	if statusOf(t, dir).Operation != git.OperationNone {
		t.Error("operation still in progress after abort")
	}
}

func TestAbortMerge(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	startConflictingMerge(t, dir)

	if _, err := recoveryService().Abort(context.Background(), dir); err != nil {
		t.Fatalf("Abort() error = %v, want nil", err)
	}
	if statusOf(t, dir).Operation != git.OperationNone {
		t.Error("merge still in progress after abort")
	}
}

func TestAbortCherryPick(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "README.md", "feature\n")
	runGit(t, dir, "commit", "-qam", "feature change")
	runGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "README.md", "main\n")
	runGit(t, dir, "commit", "-qam", "main change")
	if out, err := tryGit(dir, "cherry-pick", "feature"); err == nil {
		t.Fatalf("cherry-pick unexpectedly succeeded:\n%s", out)
	}

	if _, err := recoveryService().Abort(context.Background(), dir); err != nil {
		t.Fatalf("Abort() error = %v, want nil", err)
	}
}

func TestUndoOneCommit(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	writeFile(t, dir, "README.md", "second\n")
	runGit(t, dir, "commit", "-qam", "second")

	result, err := recoveryService().Undo(context.Background(), dir, 1)
	if err != nil {
		t.Fatalf("Undo() error = %v, want nil", err)
	}
	if result.Count != 1 || result.Commits[0].Subject != "second" {
		t.Fatalf("Undo() = %+v", result)
	}

	status := statusOf(t, dir)
	if status.Clean() {
		t.Fatal("working tree clean after undo; changes should be staged")
	}
	if len(status.Staged) == 0 {
		t.Fatal("expected staged changes after soft undo")
	}
	log := runGit(t, dir, "log", "--oneline")
	if strings.Contains(log, "second") {
		t.Errorf("commit still present:\n%s", log)
	}
}

func TestUndoMultipleCommits(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	writeFile(t, dir, "a.txt", "a\n")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-qm", "a")
	writeFile(t, dir, "b.txt", "b\n")
	runGit(t, dir, "add", "b.txt")
	runGit(t, dir, "commit", "-qm", "b")

	result, err := recoveryService().Undo(context.Background(), dir, 2)
	if err != nil {
		t.Fatalf("Undo() error = %v, want nil", err)
	}
	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2", result.Count)
	}
	status := statusOf(t, dir)
	if len(status.Staged) < 2 {
		t.Fatalf("Staged = %+v, want both files preserved", status.Staged)
	}
}

func TestUndoNothingToUndo(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")

	_, err := recoveryService().Undo(context.Background(), dir, 1)
	if !errors.Is(err, workflow.ErrNothingToUndo) {
		t.Fatalf("Undo() error = %v, want ErrNothingToUndo", err)
	}
}

func TestUndoDirtyTreePreservesExistingChanges(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	writeFile(t, dir, "README.md", "second\n")
	runGit(t, dir, "commit", "-qam", "second")
	writeFile(t, dir, "extra.txt", "extra\n")

	if _, err := recoveryService().Undo(context.Background(), dir, 1); err != nil {
		t.Fatalf("Undo() error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "extra.txt")); err != nil {
		t.Fatalf("extra.txt missing after undo: %v", err)
	}
}

func TestUndoDetachedHead(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	writeFile(t, dir, "README.md", "second\n")
	runGit(t, dir, "commit", "-qam", "second")
	runGit(t, dir, "checkout", "-q", "--detach", "HEAD")

	_, err := recoveryService().Undo(context.Background(), dir, 1)
	if !errors.Is(err, workflow.ErrDetachedHead) {
		t.Fatalf("Undo() error = %v, want ErrDetachedHead", err)
	}
}

func TestUndoWithUpstreamWarnsInResult(t *testing.T) {
	requireGit(t)
	upstream := initBareRepository(t)
	dir := initRepository(t, "README.md")
	runGit(t, dir, "remote", "add", "origin", upstream)
	runGit(t, dir, "push", "-u", "-q", "origin", "main")
	writeFile(t, dir, "README.md", "second\n")
	runGit(t, dir, "commit", "-qam", "second")

	result, err := recoveryService().Undo(context.Background(), dir, 1)
	if err != nil {
		t.Fatalf("Undo() error = %v, want nil", err)
	}
	if !result.HasUpstream {
		t.Error("HasUpstream = false, want true")
	}
}

func TestCleanMergedPreview(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "done")
	writeFile(t, dir, "done.txt", "done\n")
	runGit(t, dir, "add", "done.txt")
	runGit(t, dir, "commit", "-qm", "done")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "-q", "--no-ff", "done", "-m", "merge done")

	result, err := recoveryService().Clean(context.Background(), dir, workflow.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean() error = %v, want nil", err)
	}
	found := false
	for _, c := range result.Candidates {
		if c.Name == "done" {
			found = true
		}
		if c.Name == "main" {
			t.Fatal("main must never be a clean candidate")
		}
	}
	if !found {
		t.Fatalf("Candidates = %+v, want done", result.Candidates)
	}
	if result.Applied || len(result.Deleted) != 0 {
		t.Fatal("preview must not delete")
	}
}

func TestCleanApplyDeletesMerged(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "done")
	writeFile(t, dir, "done.txt", "done\n")
	runGit(t, dir, "add", "done.txt")
	runGit(t, dir, "commit", "-qm", "done")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "-q", "--no-ff", "done", "-m", "merge done")

	result, err := recoveryService().Clean(context.Background(), dir, workflow.CleanRequest{Apply: true})
	if err != nil {
		t.Fatalf("Clean() error = %v, want nil", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "done" {
		t.Fatalf("Deleted = %v, want [done]", result.Deleted)
	}
	branches := runGit(t, dir, "branch", "--list")
	if strings.Contains(branches, "done") {
		t.Errorf("done still listed:\n%s", branches)
	}
}

func TestCleanGoneRemoteTracking(t *testing.T) {
	requireGit(t)
	upstream := initBareRepository(t)
	runGit(t, upstream, "symbolic-ref", "HEAD", "refs/heads/main")

	dir := initRepository(t, "README.md")
	runGit(t, dir, "remote", "add", "origin", upstream)
	runGit(t, dir, "push", "-u", "-q", "origin", "main")
	runGit(t, dir, "checkout", "-q", "-b", "stale")
	writeFile(t, dir, "stale.txt", "stale\n")
	runGit(t, dir, "add", "stale.txt")
	runGit(t, dir, "commit", "-qm", "stale")
	runGit(t, dir, "push", "-u", "-q", "origin", "stale")
	runGit(t, dir, "checkout", "-q", "main")

	// Delete the remote branch and prune so the local tracking ref is gone.
	runGit(t, dir, "push", "-q", "origin", "--delete", "stale")
	runGit(t, dir, "fetch", "-q", "--prune", "origin")

	result, err := recoveryService().Clean(context.Background(), dir, workflow.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean() error = %v, want nil", err)
	}
	found := false
	for _, c := range result.Candidates {
		if c.Name == "stale" && c.Reason == git.StaleGone {
			found = true
		}
	}
	if !found {
		t.Fatalf("Candidates = %+v, want stale (gone)", result.Candidates)
	}
}

func TestCleanSkipsCurrentBranch(t *testing.T) {
	requireGit(t)
	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "f.txt", "f\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-qm", "f")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "-q", "--ff-only", "feature")
	runGit(t, dir, "checkout", "-q", "feature")

	result, err := recoveryService().Clean(context.Background(), dir, workflow.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean() error = %v, want nil", err)
	}
	for _, c := range result.Candidates {
		if c.Name == "feature" {
			t.Fatal("current branch must not be a candidate")
		}
	}
}

func TestCleanProtectsMaster(t *testing.T) {
	requireGit(t)
	dir := initRepositoryOn(t, "master", "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "done")
	writeFile(t, dir, "done.txt", "done\n")
	runGit(t, dir, "add", "done.txt")
	runGit(t, dir, "commit", "-qm", "done")
	runGit(t, dir, "checkout", "-q", "master")
	runGit(t, dir, "merge", "-q", "--no-ff", "done", "-m", "merge done")

	result, err := recoveryService().Clean(context.Background(), dir, workflow.CleanRequest{})
	if err != nil {
		t.Fatalf("Clean() error = %v, want nil", err)
	}
	for _, c := range result.Candidates {
		if c.Name == "master" {
			t.Fatal("master must never be a clean candidate")
		}
	}
}
