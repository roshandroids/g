package workflow

import (
	"context"
	"fmt"

	"github.com/roshandroids/g/internal/git"
)

// ContinueResult is the outcome of finishing a paused Git operation.
type ContinueResult struct {
	// Operation is the operation that was continued.
	Operation git.Operation
	// Branch is the current branch after continuing, if any.
	Branch string
	// Detached reports whether HEAD remained detached.
	Detached bool
	// Conflicted lists paths that are still unresolved after a failed continue
	// attempt is not used; success leaves this empty. Reserved for reporting
	// pre-continue state.
	Conflicted []string
	// Dirty reports whether the working tree had local changes before continue.
	Dirty bool
}

// AbortResult is the outcome of cancelling a paused Git operation.
type AbortResult struct {
	// Operation is the operation that was aborted.
	Operation git.Operation
}

// Continue resumes the paused Git operation in dir.
//
// The operation is detected from the same markers status and sync use. Nothing
// is guessed when no operation is present or when markers conflict.
func (s *Service) Continue(ctx context.Context, dir string) (ContinueResult, error) {
	raw, err := s.status(ctx, dir)
	if err != nil {
		return ContinueResult{}, err
	}

	op, err := s.requirePaused(ctx, dir, raw)
	if err != nil {
		return ContinueResult{}, err
	}

	result := ContinueResult{
		Operation:  op,
		Branch:     raw.Branch,
		Detached:   raw.Detached,
		Dirty:      !raw.Clean(),
		Conflicted: conflictedPaths(raw),
	}

	if err := s.repo.Continue(ctx, dir, op); err != nil {
		return ContinueResult{}, err
	}

	after, err := s.status(ctx, dir)
	if err != nil {
		return result, nil
	}
	result.Branch = after.Branch
	result.Detached = after.Detached
	return result, nil
}

// Abort cancels the paused Git operation in dir.
//
// Confirmation belongs at the command edge; the workflow only performs the
// abort once that edge has already agreed.
func (s *Service) Abort(ctx context.Context, dir string) (AbortResult, error) {
	raw, err := s.status(ctx, dir)
	if err != nil {
		return AbortResult{}, err
	}

	op, err := s.requirePaused(ctx, dir, raw)
	if err != nil {
		return AbortResult{}, err
	}

	if err := s.repo.Abort(ctx, dir, op); err != nil {
		return AbortResult{}, err
	}
	return AbortResult{Operation: op}, nil
}

// CurrentOperation reports the paused operation in dir, if any.
//
// Used by the abort command to describe what would be discarded before asking
// for confirmation.
func (s *Service) CurrentOperation(ctx context.Context, dir string) (git.Operation, error) {
	raw, err := s.status(ctx, dir)
	if err != nil {
		return git.OperationNone, err
	}
	if raw.Operation == git.OperationNone {
		return git.OperationNone, fmt.Errorf("%w", ErrNoOperation)
	}
	return raw.Operation, nil
}

func (s *Service) requirePaused(ctx context.Context, dir string, raw git.Status) (git.Operation, error) {
	ops, err := s.repo.PausedOperations(ctx, dir)
	if err != nil {
		return git.OperationNone, err
	}

	op, err := git.ResolveOperation(ops)
	if err != nil {
		return git.OperationNone, err
	}
	if op == git.OperationNone {
		return git.OperationNone, fmt.Errorf("%w", ErrNoOperation)
	}

	// Prefer the shared resolution over status's cached field so continue and
	// abort cannot act on a stale or differently resolved view.
	_ = raw
	return op, nil
}

func conflictedPaths(raw git.Status) []string {
	var paths []string
	for _, file := range raw.Files {
		if file.Unmerged {
			paths = append(paths, file.Path)
		}
	}
	return paths
}
