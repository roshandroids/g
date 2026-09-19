// Package workflow holds the UI-independent operations g performs on a
// repository.
//
// Everything here takes and returns plain values, so the CLI and any future GUI
// drive exactly the same logic. The package never executes anything itself: it
// depends on the Repository interface, which the git package implements and
// tests replace with a fake.
package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/roshandroids/g/internal/branch"
	"github.com/roshandroids/g/internal/git"
)

// Repository is the Git capability the workflow layer needs.
type Repository interface {
	// Status reports the state of the repository containing dir.
	Status(ctx context.Context, dir string) (git.Status, error)
	// LocalBranches lists the repository's local branch names.
	LocalBranches(ctx context.Context, dir string) ([]string, error)
	// Remotes lists the configured remote names.
	Remotes(ctx context.Context, dir string) ([]string, error)
	// RemoteBranchExists reports whether a remote has a branch called name.
	RemoteBranchExists(ctx context.Context, dir, remote, name string) (bool, error)
	// CreateBranch creates name from base and switches to it.
	CreateBranch(ctx context.Context, dir, name, base string) error
	// SwitchBranch switches to an existing branch.
	SwitchBranch(ctx context.Context, dir, name string) error
	// Commit records the staged changes with message.
	Commit(ctx context.Context, dir, message string) error
	// Push sends the current branch to its remote.
	Push(ctx context.Context, dir string, opts git.PushOptions) error
}

// Options configures a Service.
//
// The zero value is usable: NewService fills in the defaults for anything left
// unset, so callers only supply what they want to change.
type Options struct {
	// DefaultBaseBranch is the branch new work is started from. It is tried
	// first, before the conventional fallbacks.
	DefaultBaseBranch string
	// BranchSeparator joins the issue key and the description in a branch
	// name.
	BranchSeparator string
	// BranchMaxLength caps generated branch names; zero means no limit.
	BranchMaxLength int
}

// DefaultOptions returns the built-in Service configuration.
func DefaultOptions() Options {
	return Options{
		DefaultBaseBranch: "main",
		BranchSeparator:   branch.DefaultSeparator,
	}
}

// Service performs repository workflows.
type Service struct {
	repo   Repository
	opts   Options
	naming branch.Namer
}

// NewService returns a Service backed by repo.
func NewService(repo Repository, opts Options) *Service {
	if opts.DefaultBaseBranch == "" {
		opts.DefaultBaseBranch = DefaultOptions().DefaultBaseBranch
	}
	if opts.BranchSeparator == "" {
		opts.BranchSeparator = branch.DefaultSeparator
	}

	return &Service{
		repo:   repo,
		opts:   opts,
		naming: branch.Policy{Separator: opts.BranchSeparator, MaxLength: opts.BranchMaxLength},
	}
}

// Status summarises the repository containing dir.
func (s *Service) Status(ctx context.Context, dir string) (Status, error) {
	raw, err := s.status(ctx, dir)
	if err != nil {
		return Status{}, err
	}
	return summarize(raw), nil
}

// status reads repository state, translating git's not-a-repository signal into
// the workflow's own error.
func (s *Service) status(ctx context.Context, dir string) (git.Status, error) {
	raw, err := s.repo.Status(ctx, dir)
	if err != nil {
		if errors.Is(err, git.ErrNotARepository) {
			return git.Status{}, fmt.Errorf("%w: %s", ErrNotARepository, dir)
		}
		return git.Status{}, err
	}
	return raw, nil
}

// requireIdle reads repository state and refuses to continue unless the
// repository is on a branch with no other operation paused.
//
// Both conditions would otherwise surface later as a confusing git failure, or
// worse, as a branch created on top of a half-finished rebase.
func (s *Service) requireIdle(ctx context.Context, dir string) (git.Status, error) {
	raw, err := s.status(ctx, dir)
	if err != nil {
		return git.Status{}, err
	}

	if raw.Operation != git.OperationNone {
		return git.Status{}, &OperationError{Operation: raw.Operation}
	}
	if raw.Detached {
		return git.Status{}, fmt.Errorf("%w at %s; check out a branch first", ErrDetachedHead, raw.Head)
	}

	return raw, nil
}

// baseBranch picks the branch new work is created from.
//
// The configured default is tried first, then the two conventional names, so a
// repository works out of the box whether it calls its trunk main or master
// without either name being hard-coded into the command layer.
func (s *Service) baseBranch(local []string) (string, error) {
	for _, candidate := range s.baseCandidates() {
		if contains(local, candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("cannot find a base branch: none of %s exist", list(s.baseCandidates()))
}

// baseCandidates lists the base branches to try, in order and without repeats.
func (s *Service) baseCandidates() []string {
	var candidates []string
	for _, name := range []string{s.opts.DefaultBaseBranch, "main", "master"} {
		if name != "" && !contains(candidates, name) {
			candidates = append(candidates, name)
		}
	}
	return candidates
}

// existingRemoteBranch reports the remote that already has a branch called
// name, if any.
func (s *Service) existingRemoteBranch(ctx context.Context, dir, name string) (string, bool, error) {
	remotes, err := s.repo.Remotes(ctx, dir)
	if err != nil {
		return "", false, err
	}

	for _, remote := range remotes {
		exists, err := s.repo.RemoteBranchExists(ctx, dir, remote, name)
		if err != nil {
			return "", false, err
		}
		if exists {
			return remote, true, nil
		}
	}

	return "", false, nil
}

// contains reports whether values holds want.
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
