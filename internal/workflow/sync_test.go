package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
)

func TestSyncRebasesFeatureOntoUpdatedBase(t *testing.T) {
	repo := &fakeRepository{
		status:         git.Status{Root: "/repo", Branch: "feature"},
		branches:       []string{"feature", "main"},
		remotes:        []string{"origin"},
		remoteBranches: map[string]bool{"origin/main": true},
		aheadBehind:    aheadBehindResult{ahead: 2, behind: 1},
		revParses:      []string{"aaa", "bbb"},
		preserveResult: git.PreserveResult{},
	}
	// First AheadBehind (pre-fetch): behind>0 so rebase planned.
	// updateBase AheadBehind(tracking, base): local behind remote.
	// Second feature AheadBehind after update: still behind.
	// Final AheadBehind in finish: ahead only.
	repo.aheadBehindSeq = []aheadBehindResult{
		{ahead: 2, behind: 1}, // feature vs base (initial)
		{ahead: 0, behind: 1}, // base vs remote (local behind)
		{ahead: 2, behind: 1}, // feature vs base (after update)
		{ahead: 2, behind: 0}, // feature vs base (final)
	}

	result, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}

	if !result.Fetched || result.Remote != "origin" {
		t.Errorf("Fetched/Remote = %v/%q, want true/origin", result.Fetched, result.Remote)
	}
	if !result.BaseUpdated {
		t.Error("BaseUpdated = false, want true")
	}
	if !result.Rebased || !result.HistoryRewritten {
		t.Errorf("Rebased/HistoryRewritten = %v/%v, want true/true", result.Rebased, result.HistoryRewritten)
	}
	if got := repo.rebaseCalls; len(got) != 1 || got[0] != "main" {
		t.Errorf("rebaseCalls = %v, want [main]", got)
	}
	if got := repo.ffBranchCalls; len(got) != 1 || got[0].branch != "main" {
		t.Errorf("ffBranchCalls = %+v, want main fast-forwarded", got)
	}
}

func TestSyncOnBaseFastForwardsWithoutRebase(t *testing.T) {
	repo := &fakeRepository{
		status:         git.Status{Root: "/repo", Branch: "main"},
		branches:       []string{"main"},
		remotes:        []string{"origin"},
		remoteBranches: map[string]bool{"origin/main": true},
		aheadBehindSeq: []aheadBehindResult{
			{ahead: 0, behind: 2}, // local base behind remote
		},
	}

	result, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}

	if !result.OnBase || !result.BaseUpdated {
		t.Errorf("OnBase/BaseUpdated = %v/%v, want true/true", result.OnBase, result.BaseUpdated)
	}
	if len(repo.rebaseCalls) != 0 {
		t.Errorf("rebaseCalls = %v, want none on the base branch", repo.rebaseCalls)
	}
	if got := repo.mergeFFCalls; len(got) != 1 || got[0] != "origin/main" {
		t.Errorf("mergeFFCalls = %v, want [origin/main]", got)
	}
}

func TestSyncRejectsOperationInProgress(t *testing.T) {
	repo := &fakeRepository{
		status: git.Status{Root: "/repo", Branch: "feature", Operation: git.OperationRebase},
	}

	_, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("Sync() error = %v, want ErrOperationInProgress", err)
	}
}

func TestSyncRejectsDetachedHead(t *testing.T) {
	repo := &fakeRepository{
		status: git.Status{Root: "/repo", Detached: true, Head: "abc1234"},
	}

	_, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if !errors.Is(err, ErrDetachedHead) {
		t.Fatalf("Sync() error = %v, want ErrDetachedHead", err)
	}
}

func TestSyncStopsOnDivergentBase(t *testing.T) {
	repo := &fakeRepository{
		status:         git.Status{Root: "/repo", Branch: "feature"},
		branches:       []string{"feature", "main"},
		remotes:        []string{"origin"},
		remoteBranches: map[string]bool{"origin/main": true},
		aheadBehindSeq: []aheadBehindResult{
			{ahead: 1, behind: 0}, // feature vs base
			{ahead: 1, behind: 1}, // divergent base
		},
	}

	_, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if !errors.Is(err, ErrDivergentBase) {
		t.Fatalf("Sync() error = %v, want ErrDivergentBase", err)
	}
	if len(repo.rebaseCalls) != 0 {
		t.Error("rebase must not run when the base cannot be updated")
	}
}

func TestSyncFetchFailureRestoresPreservedChanges(t *testing.T) {
	repo := &fakeRepository{
		status: git.Status{
			Root:   "/repo",
			Branch: "feature",
			Files:  []git.FileChange{{Path: "a.txt", Worktree: git.Modified}},
		},
		branches:       []string{"feature", "main"},
		remotes:        []string{"origin"},
		aheadBehindSeq: []aheadBehindResult{{ahead: 1, behind: 1}},
		preserveResult: git.PreserveResult{Created: true, Ref: "stash@{0}"},
		fetchErr:       errors.New("network down"),
	}

	_, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("Sync() error = %v, want fetch failure", err)
	}
	if repo.preserveCalls != 1 || repo.restoreCalls != 1 {
		t.Errorf("preserve/restore = %d/%d, want 1/1 so work is not left in the stash", repo.preserveCalls, repo.restoreCalls)
	}
}

func TestSyncWithoutRemoteStillRebasesLocally(t *testing.T) {
	repo := &fakeRepository{
		status:   git.Status{Root: "/repo", Branch: "feature"},
		branches: []string{"feature", "main"},
		aheadBehindSeq: []aheadBehindResult{
			{ahead: 1, behind: 1},
			{ahead: 1, behind: 1},
			{ahead: 1, behind: 0},
		},
		revParses: []string{"aaa", "bbb"},
	}

	result, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if result.Fetched {
		t.Error("Fetched = true, want false with no remotes")
	}
	if !result.Rebased {
		t.Error("Rebased = false, want true against the local base")
	}
}

func TestSyncPreservesDirtyTreeAcrossRebase(t *testing.T) {
	repo := &fakeRepository{
		status: git.Status{
			Root:   "/repo",
			Branch: "feature",
			Files:  []git.FileChange{{Path: "a.txt", Worktree: git.Modified}},
		},
		branches:       []string{"feature", "main"},
		aheadBehindSeq: []aheadBehindResult{{ahead: 1, behind: 1}, {ahead: 1, behind: 1}, {ahead: 1, behind: 0}},
		revParses:      []string{"aaa", "bbb"},
		preserveResult: git.PreserveResult{Created: true, Ref: "stash@{0}"},
	}

	result, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if !result.Preserved || !result.Restored {
		t.Errorf("Preserved/Restored = %v/%v, want true/true", result.Preserved, result.Restored)
	}
}

func TestSyncRebaseConflictKeepsStash(t *testing.T) {
	repo := &fakeRepository{
		status: git.Status{
			Root:   "/repo",
			Branch: "feature",
			Files:  []git.FileChange{{Path: "a.txt", Worktree: git.Modified}},
		},
		branches:       []string{"feature", "main"},
		aheadBehindSeq: []aheadBehindResult{{ahead: 1, behind: 1}, {ahead: 1, behind: 1}},
		revParses:      []string{"aaa"},
		preserveResult: git.PreserveResult{Created: true, Ref: "stash@{0}"},
		rebaseErr:      &git.RebaseConflictError{},
	}

	_, err := NewService(repo, Options{}).Sync(context.Background(), "/repo")
	if err == nil {
		t.Fatal("Sync() error = nil, want rebase conflict")
	}
	if !strings.Contains(err.Error(), "git stash pop") {
		t.Errorf("Sync() error = %v, want stash recovery hint", err)
	}
	if repo.restoreCalls != 0 {
		t.Error("restore must not run while a rebase is paused")
	}
}
