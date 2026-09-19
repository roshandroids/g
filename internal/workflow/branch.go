package workflow

import (
	"context"
	"errors"
	"strings"
)

// NewBranchRequest describes the branch to create.
type NewBranchRequest struct {
	// Ticket is the issue key the work belongs to, for example "HCM-37538".
	Ticket string
	// Description summarises the work and becomes the branch name suffix.
	Description string
}

// NewBranchResult describes the branch that was created.
//
// It reports the conditions the caller may want to mention; it does not phrase
// them, so the same result renders as text in a terminal and as a JSON field in
// a future GUI.
type NewBranchResult struct {
	// Branch is the name of the branch that was created.
	Branch string
	// Base is the branch the new branch points at.
	Base string
	// Dirty reports that the working tree had uncommitted changes, which the
	// new branch carries along rather than discarding.
	Dirty bool
	// RemoteExists reports that a remote already has a branch of this name.
	RemoteExists bool
	// Remote is the remote named by RemoteExists.
	Remote string
}

// SwitchRequest describes the branch to switch to.
type SwitchRequest struct {
	// Branch is the exact name of an existing local branch.
	Branch string
}

// SwitchResult describes the branch that is now checked out.
type SwitchResult struct {
	// Branch is the branch that is now checked out.
	Branch string
	// Previous is the branch that was left; empty when HEAD was detached.
	Previous string
	// AlreadyOn reports that the requested branch was already checked out, so
	// nothing had to change.
	AlreadyOn bool
	// Dirty reports that the working tree has uncommitted changes, which
	// travelled to the new branch rather than being discarded.
	Dirty bool
}

// NewBranch creates a branch for a piece of work and switches to it.
//
// The branch is created from the local base branch as it stands: nothing is
// fetched, pulled or rebased. Updating the base is a separate decision with its
// own safety story, and doing it implicitly here would mean a branch whose
// starting point depends on network state.
func (s *Service) NewBranch(ctx context.Context, dir string, req NewBranchRequest) (NewBranchResult, error) {
	name, err := s.naming.Name(req.Ticket, req.Description)
	if err != nil {
		return NewBranchResult{}, err
	}

	state, err := s.requireIdle(ctx, dir)
	if err != nil {
		return NewBranchResult{}, err
	}

	local, err := s.repo.LocalBranches(ctx, dir)
	if err != nil {
		return NewBranchResult{}, err
	}
	if contains(local, name) {
		return NewBranchResult{}, &BranchExistsError{Name: name}
	}

	base, err := s.baseBranch(local)
	if err != nil {
		return NewBranchResult{}, err
	}

	result := NewBranchResult{Branch: name, Base: base, Dirty: !state.Clean()}

	// A branch of the same name on the remote is not fatal, but pushing later
	// would collide with it, so it is worth knowing before the work starts.
	if remote, ok, err := s.existingRemoteBranch(ctx, dir, name); err != nil {
		return NewBranchResult{}, err
	} else if ok {
		result.RemoteExists = true
		result.Remote = remote
	}

	if err := s.repo.CreateBranch(ctx, dir, name, base); err != nil {
		return NewBranchResult{}, err
	}

	return result, nil
}

// Switch checks out an existing local branch.
//
// The match is exact: g does not guess at a branch that does not exist, because
// creating work on the wrong branch is much harder to notice than an error.
func (s *Service) Switch(ctx context.Context, dir string, req SwitchRequest) (SwitchResult, error) {
	name := strings.TrimSpace(req.Branch)
	if name == "" {
		return SwitchResult{}, errors.New("a branch name is required")
	}

	state, err := s.requireIdle(ctx, dir)
	if err != nil {
		return SwitchResult{}, err
	}

	local, err := s.repo.LocalBranches(ctx, dir)
	if err != nil {
		return SwitchResult{}, err
	}
	if !contains(local, name) {
		return SwitchResult{}, &BranchNotFoundError{Name: name, Available: local}
	}

	result := SwitchResult{Branch: name, Previous: state.Branch, Dirty: !state.Clean()}
	if state.Branch == name {
		result.AlreadyOn = true
		return result, nil
	}

	// git refuses the switch outright when it would overwrite local changes,
	// so uncommitted work cannot be lost here; its refusal is passed through
	// as-is rather than being pre-empted by a weaker guess.
	if err := s.repo.SwitchBranch(ctx, dir, name); err != nil {
		return SwitchResult{}, err
	}

	return result, nil
}
