package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/commit"
	"github.com/roshandroids/g/internal/git"
)

// stagedRepository is a repository with one change already in the index.
func stagedRepository() *fakeRepository {
	return &fakeRepository{
		status: git.Status{
			Root:   branchTestDir,
			Branch: "main",
			Files: []git.FileChange{
				{Path: "lib/foo.dart", Index: git.Modified},
				{Path: "lib/bar.dart", Index: git.Added},
			},
		},
	}
}

func TestServiceCommit(t *testing.T) {
	repo := stagedRepository()

	result, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, CommitRequest{
		Type:    "fix",
		Message: "resolve applicant history issue",
	})
	if err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}

	want := "fix: resolve applicant history issue"
	if result.Message != want {
		t.Errorf("Message = %q, want %q", result.Message, want)
	}
	if result.Type != commit.Fix {
		t.Errorf("Type = %q, want %q", result.Type, commit.Fix)
	}
	if len(result.Staged) != 2 {
		t.Errorf("Staged = %+v, want the two staged changes", result.Staged)
	}

	if len(repo.commitCalls) != 1 || repo.commitCalls[0] != want {
		t.Errorf("Commit calls = %v, want exactly [%q]", repo.commitCalls, want)
	}
}

// Only the index is committed. Staging everything is a separate decision the
// user has to make, because a commit that swept up a stray file cannot be
// un-made without rewriting history.
func TestServiceCommitDoesNotStageAnything(t *testing.T) {
	repo := &fakeRepository{
		status: git.Status{
			Root:   branchTestDir,
			Branch: "main",
			Files: []git.FileChange{
				{Path: "staged.txt", Index: git.Modified},
				{Path: "unstaged.txt", Worktree: git.Modified},
				{Path: "untracked.txt", Untracked: true},
			},
		},
	}

	result, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, CommitRequest{
		Type:    "feat",
		Message: "add the thing",
	})
	if err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}

	if len(result.Staged) != 1 || result.Staged[0].Path != "staged.txt" {
		t.Errorf("Staged = %+v, want only the staged path", result.Staged)
	}
}

func TestServiceCommitRejectsNothingStaged(t *testing.T) {
	tests := []struct {
		name  string
		files []git.FileChange
		clean bool
		want  string
	}{
		{
			name:  "unstaged and untracked work exists",
			files: []git.FileChange{{Path: "a.txt", Worktree: git.Modified}, {Path: "b.txt", Untracked: true}},
			want:  "git add",
		},
		{
			name:  "nothing to commit at all",
			files: nil,
			clean: true,
			want:  "working tree is clean",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeRepository{status: git.Status{Root: branchTestDir, Branch: "main", Files: test.files}}

			_, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, CommitRequest{
				Type:    "fix",
				Message: "something",
			})
			if !errors.Is(err, ErrNothingStaged) {
				t.Fatalf("Commit() error = %v, want ErrNothingStaged", err)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("Commit() error = %v, want it to contain %q", err, test.want)
			}
			if len(repo.commitCalls) != 0 {
				t.Errorf("Commit calls = %v, want none", repo.commitCalls)
			}
		})
	}
}

// Refusing to commit during a merge would block the user from finishing the
// very state they are in, so unlike the branch workflows this one does not
// require an idle repository.
func TestServiceCommitWorksDuringAPausedOperation(t *testing.T) {
	repo := stagedRepository()
	repo.status.Operation = git.OperationMerge

	if _, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, CommitRequest{
		Type:    "chore",
		Message: "resolve the merge",
	}); err != nil {
		t.Fatalf("Commit() error = %v, want nil while a merge is paused", err)
	}
	if len(repo.commitCalls) != 1 {
		t.Errorf("Commit calls = %v, want the commit to be recorded", repo.commitCalls)
	}
}

// Committing on a detached HEAD is legal git and sometimes what the user wants,
// so it is reported rather than blocked.
func TestServiceCommitReportsADetachedHead(t *testing.T) {
	repo := stagedRepository()
	repo.status.Detached = true
	repo.status.Branch = ""
	repo.status.Head = "a1b2c3d"

	result, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, CommitRequest{
		Type:    "fix",
		Message: "hotfix",
	})
	if err != nil {
		t.Fatalf("Commit() error = %v, want nil", err)
	}
	if !result.Detached {
		t.Error("Detached = false, want true")
	}
}

func TestServiceCommitValidatesItsInput(t *testing.T) {
	tests := []struct {
		name    string
		request CommitRequest
		want    string
	}{
		{
			name:    "unknown type",
			request: CommitRequest{Type: "bugfix", Message: "something"},
			want:    "invalid commit type",
		},
		{
			name:    "empty type",
			request: CommitRequest{Type: "", Message: "something"},
			want:    "invalid commit type",
		},
		{
			name:    "empty message",
			request: CommitRequest{Type: "fix", Message: ""},
			want:    "empty",
		},
		{
			name:    "whitespace message",
			request: CommitRequest{Type: "fix", Message: "   "},
			want:    "empty",
		},
		{
			name:    "multi-line message",
			request: CommitRequest{Type: "fix", Message: "one\ntwo"},
			want:    "single line",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := stagedRepository()

			_, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, test.request)
			if err == nil {
				t.Fatal("Commit() error = nil, want a validation error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("Commit() error = %v, want it to contain %q", err, test.want)
			}
			if len(repo.commitCalls) != 0 {
				t.Errorf("Commit calls = %v, want none", repo.commitCalls)
			}
		})
	}
}

func TestServiceCommitPropagatesFailures(t *testing.T) {
	t.Run("git refuses the commit", func(t *testing.T) {
		repo := stagedRepository()
		repo.commitErr = errors.New("fatal: a pre-commit hook rejected this")

		_, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, CommitRequest{
			Type:    "fix",
			Message: "something",
		})
		if err == nil {
			t.Fatal("Commit() error = nil, want git's failure")
		}
		if !strings.Contains(err.Error(), "pre-commit hook") {
			t.Errorf("Commit() error = %v, want it to keep git's diagnostic", err)
		}
	})

	t.Run("status is unavailable", func(t *testing.T) {
		repo := &fakeRepository{statusErr: errors.New("boom")}

		_, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, CommitRequest{
			Type:    "fix",
			Message: "something",
		})
		if err == nil {
			t.Fatal("Commit() error = nil, want the status failure")
		}
	})

	t.Run("outside a repository", func(t *testing.T) {
		repo := &fakeRepository{statusErr: git.ErrNotARepository}

		_, err := NewService(repo, Options{}).Commit(context.Background(), branchTestDir, CommitRequest{
			Type:    "fix",
			Message: "something",
		})
		if !errors.Is(err, ErrNotARepository) {
			t.Fatalf("Commit() error = %v, want ErrNotARepository", err)
		}
	})
}
