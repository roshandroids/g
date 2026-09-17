package workflow

import (
	"context"
	"fmt"

	"github.com/roshandroids/g/internal/commit"
)

// CommitRequest describes the commit to record.
type CommitRequest struct {
	// Type is the conventional commit type, for example "fix".
	Type string
	// Message is the author's own description of the change.
	Message string
}

// CommitResult describes the commit that was recorded.
type CommitResult struct {
	// Message is the full message that was committed, including the type.
	Message string
	// Type is the conventional commit type that was used.
	Type commit.Type
	// Staged lists the changes that were committed.
	Staged []Change
	// Detached reports that HEAD was not on a branch, so the new commit is not
	// reachable from any branch and can be lost.
	Detached bool
}

// Commit records the staged changes.
//
// Nothing is staged on the user's behalf. "Commit everything" is convenient
// right up to the moment it sweeps up a stray file, and there is no undo for a
// commit that was never meant to exist, so the index is treated as the user's
// explicit decision about what belongs in this commit.
//
// Unlike the branch workflows this does not refuse to run mid-operation.
// Committing is how a paused merge is finished, so refusing would block the
// user from resolving the very state g is complaining about.
func (s *Service) Commit(ctx context.Context, dir string, req CommitRequest) (CommitResult, error) {
	// The command layer validates these to report a usage error, and they are
	// checked again here so any other caller gets the same guarantees.
	commitType, err := commit.ParseType(req.Type)
	if err != nil {
		return CommitResult{}, err
	}
	if err := commit.ValidateSubject(req.Message); err != nil {
		return CommitResult{}, err
	}

	status, err := s.Status(ctx, dir)
	if err != nil {
		return CommitResult{}, err
	}

	if len(status.Staged) == 0 {
		return CommitResult{}, &NothingStagedError{
			Unstaged:  len(status.Unstaged),
			Untracked: len(status.Untracked),
			Clean:     status.Clean(),
		}
	}

	message := commit.Message(commitType, req.Message)
	if err := s.repo.Commit(ctx, dir, message); err != nil {
		return CommitResult{}, err
	}

	return CommitResult{
		Message:  message,
		Type:     commitType,
		Staged:   status.Staged,
		Detached: status.Detached,
	}, nil
}

// NothingStagedError reports that there is nothing in the index to commit.
//
// It carries enough detail to tell "you forgot to stage" apart from "there is
// nothing to commit at all", which are very different problems.
type NothingStagedError struct {
	// Unstaged counts the working tree changes that could be staged.
	Unstaged int
	// Untracked counts the paths git does not track.
	Untracked int
	// Clean reports that the working tree has no changes at all.
	Clean bool
}

func (e *NothingStagedError) Error() string {
	if e.Clean {
		return "nothing to commit: the working tree is clean"
	}

	detail := fmt.Sprintf("%d unstaged, %d untracked", e.Unstaged, e.Untracked)
	return fmt.Sprintf("nothing is staged, and g does not stage files for you (%s); "+
		"run `git add <path>` for what belongs in this commit", detail)
}

// Is reports the error as ErrNothingStaged.
func (e *NothingStagedError) Is(target error) bool { return target == ErrNothingStaged }
