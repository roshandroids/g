package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/roshandroids/g/internal/git"
)

func TestContinueNoOperation(t *testing.T) {
	repo := &fakeRepository{status: git.Status{Branch: "main"}}
	_, err := NewService(repo, Options{}).Continue(context.Background(), "/repo")
	if !errors.Is(err, ErrNoOperation) {
		t.Fatalf("Continue() error = %v, want ErrNoOperation", err)
	}
}

func TestContinueRebase(t *testing.T) {
	repo := &fakeRepository{
		status:    git.Status{Branch: "", Detached: true, Head: "abc", Operation: git.OperationRebase},
		pausedOps: []git.Operation{git.OperationRebase},
	}
	result, err := NewService(repo, Options{}).Continue(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Continue() error = %v, want nil", err)
	}
	if result.Operation != git.OperationRebase {
		t.Errorf("Operation = %q, want rebase", result.Operation)
	}
	if len(repo.continueCalls) != 1 {
		t.Errorf("continueCalls = %d, want 1", len(repo.continueCalls))
	}
}

func TestAbortMerge(t *testing.T) {
	repo := &fakeRepository{
		status:    git.Status{Branch: "main", Operation: git.OperationMerge},
		pausedOps: []git.Operation{git.OperationMerge},
	}
	result, err := NewService(repo, Options{}).Abort(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Abort() error = %v, want nil", err)
	}
	if result.Operation != git.OperationMerge {
		t.Errorf("Operation = %q, want merge", result.Operation)
	}
}

func TestContinueAmbiguous(t *testing.T) {
	repo := &fakeRepository{
		status:    git.Status{Branch: "main", Operation: git.OperationMerge},
		pausedOps: []git.Operation{git.OperationMerge, git.OperationCherryPick},
	}
	_, err := NewService(repo, Options{}).Continue(context.Background(), "/repo")
	var ambiguous *git.AmbiguousOperationError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("Continue() error = %v, want AmbiguousOperationError", err)
	}
}

func TestUndoSoftReset(t *testing.T) {
	repo := &fakeRepository{
		status:        git.Status{Branch: "feature"},
		commitCount:   3,
		recentCommits: []git.CommitSummary{{Hash: "abc", Subject: "fix: x"}},
	}
	result, err := NewService(repo, Options{}).Undo(context.Background(), "/repo", 1)
	if err != nil {
		t.Fatalf("Undo() error = %v, want nil", err)
	}
	if result.Count != 1 || len(repo.softResetCalls) != 1 || repo.softResetCalls[0] != 1 {
		t.Fatalf("Undo result/calls = %+v / %v", result, repo.softResetCalls)
	}
}

func TestUndoRejectsDetachedHead(t *testing.T) {
	repo := &fakeRepository{status: git.Status{Detached: true, Head: "abc"}}
	_, err := NewService(repo, Options{}).Undo(context.Background(), "/repo", 1)
	if !errors.Is(err, ErrDetachedHead) {
		t.Fatalf("Undo() error = %v, want ErrDetachedHead", err)
	}
}

func TestUndoRejectsTooMany(t *testing.T) {
	repo := &fakeRepository{
		status:      git.Status{Branch: "main"},
		commitCount: 2,
	}
	_, err := NewService(repo, Options{}).Undo(context.Background(), "/repo", 2)
	if !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("Undo() error = %v, want ErrNothingToUndo", err)
	}
}

func TestCleanPreview(t *testing.T) {
	repo := &fakeRepository{
		status:         git.Status{Branch: "main"},
		branches:       []string{"main", "old", "gone"},
		mergedBranches: []string{"main", "old"},
		goneBranches:   []string{"gone"},
	}
	result, err := NewService(repo, Options{}).Clean(context.Background(), "/repo", CleanRequest{})
	if err != nil {
		t.Fatalf("Clean() error = %v, want nil", err)
	}
	if result.Applied {
		t.Error("Applied = true, want preview")
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("Candidates = %+v, want old and gone", result.Candidates)
	}
	if len(repo.deleteCalls) != 0 {
		t.Errorf("deleteCalls = %d, want 0", len(repo.deleteCalls))
	}
}

func TestCleanApply(t *testing.T) {
	repo := &fakeRepository{
		status:         git.Status{Branch: "main"},
		branches:       []string{"main", "old"},
		mergedBranches: []string{"main", "old"},
	}
	result, err := NewService(repo, Options{}).Clean(context.Background(), "/repo", CleanRequest{Apply: true})
	if err != nil {
		t.Fatalf("Clean() error = %v, want nil", err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "old" {
		t.Fatalf("Deleted = %v, want [old]", result.Deleted)
	}
}

func TestCleanProtectsCurrentAndBase(t *testing.T) {
	repo := &fakeRepository{
		status:         git.Status{Branch: "feature"},
		branches:       []string{"main", "master", "feature", "old"},
		mergedBranches: []string{"main", "master", "feature", "old"},
	}
	result, err := NewService(repo, Options{}).Clean(context.Background(), "/repo", CleanRequest{})
	if err != nil {
		t.Fatalf("Clean() error = %v, want nil", err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Name != "old" {
		t.Fatalf("Candidates = %+v, want only old", result.Candidates)
	}
}
