package workflow

import (
	"context"
	"fmt"

	"github.com/roshandroids/g/internal/git"
)

// UndoResult is the outcome of undoing recent commits while keeping their
// changes staged.
type UndoResult struct {
	// Branch is the branch the commits were removed from.
	Branch string
	// Count is how many commits were undone.
	Count int
	// Commits are the commits that were removed, newest first.
	Commits []git.CommitSummary
	// HasUpstream reports that the branch tracks a remote.
	HasUpstream bool
	// Upstream is the tracking branch name when HasUpstream is set.
	Upstream string
	// Ahead was the ahead count before the undo.
	Ahead int
}

// Undo removes the most recent count commits while preserving their changes in
// the index.
//
// Soft reset is the only shape used. Hard reset is never offered here.
func (s *Service) Undo(ctx context.Context, dir string, count int) (UndoResult, error) {
	if count < 1 {
		return UndoResult{}, fmt.Errorf("%w: count must be at least 1", ErrInvalidUndoCount)
	}

	state, err := s.requireIdle(ctx, dir)
	if err != nil {
		return UndoResult{}, err
	}

	available, err := s.repo.CommitCount(ctx, dir)
	if err != nil {
		return UndoResult{}, err
	}
	// Leaving the repository with no commits is not a useful undo target.
	if count >= available {
		return UndoResult{}, fmt.Errorf("%w: only %d commit(s) available to undo", ErrNothingToUndo, available-1)
	}

	commits, err := s.repo.RecentCommits(ctx, dir, count)
	if err != nil {
		return UndoResult{}, err
	}
	if len(commits) < count {
		return UndoResult{}, fmt.Errorf("%w: only %d commit(s) available to undo", ErrNothingToUndo, len(commits))
	}

	result := UndoResult{
		Branch:      state.Branch,
		Count:       count,
		Commits:     commits,
		HasUpstream: state.HasUpstream(),
		Upstream:    state.Upstream,
		Ahead:       state.Ahead,
	}

	if err := s.repo.SoftReset(ctx, dir, count); err != nil {
		return UndoResult{}, err
	}
	return result, nil
}
