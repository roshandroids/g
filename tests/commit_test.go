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

func TestCommitRecordsTheStagedChange(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	writeFile(t, dir, "feature.txt", "new file\n")
	runGit(t, dir, "add", "feature.txt")

	result, err := newService(t, workflow.Options{}).Commit(context.Background(), dir, workflow.CommitRequest{
		Type:    "feat",
		Message: "add the feature file",
	})
	if err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}

	if want := "feat: add the feature file"; result.Message != want {
		t.Errorf("Message = %q, want %q", result.Message, want)
	}
	if got := strings.TrimSpace(runGit(t, dir, "log", "-1", "--format=%s")); got != "feat: add the feature file" {
		t.Errorf("committed subject = %q, want the conventional message", got)
	}
	if status := statusOf(t, dir); !status.Clean() {
		t.Errorf("Clean() = false after committing, want true: %+v", status)
	}
}

// The message is the author's own. g adds the type and nothing else.
func TestCommitDoesNotRewriteTheMessage(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	writeFile(t, dir, "a.txt", "a\n")
	runGit(t, dir, "add", "a.txt")

	const message = "Do NOT reword THIS or re-case it"

	if _, err := newService(t, workflow.Options{}).Commit(context.Background(), dir, workflow.CommitRequest{
		Type:    "docs",
		Message: message,
	}); err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}

	if got := strings.TrimSpace(runGit(t, dir, "log", "-1", "--format=%s")); got != "docs: "+message {
		t.Errorf("committed subject = %q, want %q", got, "docs: "+message)
	}
}

// Only the index is committed. Everything else in the working tree is left
// exactly where it was.
func TestCommitLeavesUnstagedChangesAlone(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md", "other.txt")
	writeFile(t, dir, "staged.txt", "staged\n")
	runGit(t, dir, "add", "staged.txt")
	writeFile(t, dir, "other.txt", "unstaged change\n")

	result, err := newService(t, workflow.Options{}).Commit(context.Background(), dir, workflow.CommitRequest{
		Type:    "fix",
		Message: "only the staged file",
	})
	if err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}

	if len(result.Staged) != 1 || result.Staged[0].Path != "staged.txt" {
		t.Errorf("Staged = %+v, want only staged.txt", result.Staged)
	}

	files := strings.TrimSpace(runGit(t, dir, "show", "--name-only", "--format=", "HEAD"))
	if files != "staged.txt" {
		t.Errorf("committed files = %q, want only staged.txt", files)
	}

	status := statusOf(t, dir)
	if len(status.Unstaged) != 1 || status.Unstaged[0].Path != "other.txt" {
		t.Errorf("Unstaged = %+v, want other.txt untouched", status.Unstaged)
	}
}

func TestCommitRefusesWhenNothingIsStaged(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	writeFile(t, dir, "untracked.txt", "scratch\n")

	_, err := newService(t, workflow.Options{}).Commit(context.Background(), dir, workflow.CommitRequest{
		Type:    "fix",
		Message: "nothing staged",
	})
	if !errors.Is(err, workflow.ErrNothingStaged) {
		t.Fatalf("Commit() error = %v, want ErrNothingStaged", err)
	}
	if !strings.Contains(err.Error(), "git add") {
		t.Errorf("Commit() error = %v, want it to say how to stage", err)
	}

	// A stray commit here would be an empty one, which is exactly what this
	// guard exists to prevent.
	if got := strings.TrimSpace(runGit(t, dir, "log", "--oneline")); strings.Count(got, "\n") != 0 {
		t.Errorf("log = %q, want no commit to have been created", got)
	}
}

func TestCommitRefusesOnACleanTree(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")

	_, err := newService(t, workflow.Options{}).Commit(context.Background(), dir, workflow.CommitRequest{
		Type:    "fix",
		Message: "nothing to do",
	})
	if !errors.Is(err, workflow.ErrNothingStaged) {
		t.Fatalf("Commit() error = %v, want ErrNothingStaged", err)
	}
	if !strings.Contains(err.Error(), "working tree is clean") {
		t.Errorf("Commit() error = %v, want it to distinguish a clean tree", err)
	}
}

// Finishing a merge is exactly what committing is for, so a paused merge must
// not block the command the way it blocks the branch workflows.
func TestCommitResolvesAPausedMerge(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	startConflictingMerge(t, dir)

	writeFile(t, dir, "README.md", "resolved\n")
	runGit(t, dir, "add", "README.md")

	if _, err := newService(t, workflow.Options{}).Commit(context.Background(), dir, workflow.CommitRequest{
		Type:    "chore",
		Message: "merge the feature branch",
	}); err != nil {
		t.Fatalf("Commit() error = %v, want nil while a merge is paused", err)
	}

	// The merge has to be genuinely finished, not merely reported as such.
	raw, err := git.NewClient(process.ExecRunner{}).Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}
	if raw.Operation != git.OperationNone {
		t.Errorf("Operation = %q, want the merge to be complete", raw.Operation)
	}
}

// A rejected commit has to leave the index alone: the user's staging decisions
// are not g's to undo.
func TestCommitKeepsTheIndexWhenGitsHooksReject(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	writeFile(t, dir, ".git/hooks/pre-commit", "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(hook, 0o700); err != nil {
		t.Fatalf("making the hook executable: %v", err)
	}

	writeFile(t, dir, "a.txt", "a\n")
	runGit(t, dir, "add", "a.txt")

	_, err := newService(t, workflow.Options{}).Commit(context.Background(), dir, workflow.CommitRequest{
		Type:    "feat",
		Message: "should be rejected",
	})
	if err == nil {
		t.Fatal("Commit() error = nil, want the hook to reject the commit")
	}

	status := statusOf(t, dir)
	if len(status.Staged) != 1 {
		t.Errorf("Staged = %+v, want the index left intact after a rejected commit", status.Staged)
	}
}

func TestCommitOutsideARepository(t *testing.T) {
	requireGit(t)

	_, err := newService(t, workflow.Options{}).Commit(context.Background(), t.TempDir(), workflow.CommitRequest{
		Type:    "fix",
		Message: "anything",
	})
	if !errors.Is(err, workflow.ErrNotARepository) {
		t.Fatalf("Commit() error = %v, want ErrNotARepository", err)
	}
}
