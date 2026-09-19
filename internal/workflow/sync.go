package workflow

import (
	"context"
	"fmt"

	"github.com/roshandroids/g/internal/git"
)

// SyncResult describes a completed synchronisation.
type SyncResult struct {
	// Branch is the branch that was synchronised.
	Branch string
	// Base is the base branch used as the integration target.
	Base string
	// Ahead counts commits on Branch that Base lacks.
	Ahead int
	// Behind counts commits on Base that Branch lacks.
	Behind int
	// OnBase reports that Branch is the base branch itself.
	OnBase bool
	// Fetched reports that a remote was fetched.
	Fetched bool
	// Remote is the remote that was fetched, when Fetched is true.
	Remote string
	// BaseUpdated reports that the local base moved forward from the remote.
	BaseUpdated bool
	// Rebased reports that the feature branch was rebased onto Base.
	Rebased bool
	// HistoryRewritten reports that rebase changed Branch's commit history.
	HistoryRewritten bool
	// Preserved reports that local changes were stashed before mutating work.
	Preserved bool
	// Restored reports that preserved changes were restored afterwards.
	Restored bool
}

// Sync updates the current branch from its base.
//
// State machine (every step has a failure path that stops the workflow):
//
//	Inspect → Validate → Determine base → Determine working state →
//	Preserve (if needed) → Fetch → Update base → Rebase feature →
//	Restore → Verify → Report
//
// Sync never pushes, never force-pushes, never aborts an existing operation,
// and never discards local changes. Dry-run is left for a later phase.
func (s *Service) Sync(ctx context.Context, dir string) (SyncResult, error) {
	// Inspect + Validate.
	state, err := s.requireIdle(ctx, dir)
	if err != nil {
		return SyncResult{}, err
	}

	local, err := s.repo.LocalBranches(ctx, dir)
	if err != nil {
		return SyncResult{}, err
	}

	base, err := s.syncBaseBranch(ctx, dir, local)
	if err != nil {
		return SyncResult{}, err
	}

	result := SyncResult{
		Branch: state.Branch,
		Base:   base,
		OnBase: state.Branch == base,
	}

	remotes, err := s.repo.Remotes(ctx, dir)
	if err != nil {
		return SyncResult{}, err
	}

	dirty := !state.Clean()
	needsRebase := false
	if !result.OnBase {
		ahead, behind, err := s.repo.AheadBehind(ctx, dir, base, state.Branch)
		if err != nil {
			return SyncResult{}, err
		}
		result.Ahead, result.Behind = ahead, behind
		needsRebase = behind > 0
	}

	// Preserve when a later step needs a clean tree: rebasing the feature, or
	// fast-forwarding the base while it is checked out.
	mayUpdateBase := len(remotes) > 0
	if dirty && (needsRebase || (result.OnBase && mayUpdateBase)) {
		preserved, err := s.repo.PreserveChanges(ctx, dir, "g sync: temporary")
		if err != nil {
			return SyncResult{}, fmt.Errorf("preserving local changes: %w", err)
		}
		result.Preserved = preserved.Created
	}

	if len(remotes) > 0 {
		remote, err := chooseRemote(remotes)
		if err != nil {
			return s.abortPreserve(ctx, dir, result, err)
		}

		if err := s.repo.Fetch(ctx, dir, remote); err != nil {
			return s.abortPreserve(ctx, dir, result, fmt.Errorf("fetching %s: %w", remote, err))
		}
		result.Fetched = true
		result.Remote = remote

		updated, err := s.updateBaseFromRemote(ctx, dir, state.Branch, base, remote)
		if err != nil {
			return s.abortPreserve(ctx, dir, result, err)
		}
		result.BaseUpdated = updated
	}

	if result.OnBase {
		return s.finishSync(ctx, dir, result)
	}

	ahead, behind, err := s.repo.AheadBehind(ctx, dir, base, state.Branch)
	if err != nil {
		return s.abortPreserve(ctx, dir, result, err)
	}
	result.Ahead, result.Behind = ahead, behind
	needsRebase = behind > 0

	// Base may have moved since the first check; preserve now if rebase is newly required.
	if needsRebase && dirty && !result.Preserved {
		after, err := s.repo.Status(ctx, dir)
		if err != nil {
			return SyncResult{}, err
		}
		if !after.Clean() {
			preserved, err := s.repo.PreserveChanges(ctx, dir, "g sync: temporary")
			if err != nil {
				return SyncResult{}, fmt.Errorf("preserving local changes: %w", err)
			}
			result.Preserved = preserved.Created
		}
	}

	if needsRebase {
		before, err := s.repo.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return s.abortPreserve(ctx, dir, result, err)
		}

		if err := s.repo.Rebase(ctx, dir, base); err != nil {
			if result.Preserved {
				return SyncResult{}, fmt.Errorf("%w\n\nLocal changes are still preserved in the stash.\nAfter the rebase finishes, restore them with:\n  git stash pop", err)
			}
			return SyncResult{}, err
		}

		after, err := s.repo.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return s.abortPreserve(ctx, dir, result, err)
		}

		result.Rebased = true
		result.HistoryRewritten = before != after
	}

	return s.finishSync(ctx, dir, result)
}

// finishSync restores preserved changes and reports final ahead/behind counts.
func (s *Service) finishSync(ctx context.Context, dir string, result SyncResult) (SyncResult, error) {
	if result.Preserved {
		if err := s.repo.RestoreChanges(ctx, dir); err != nil {
			return SyncResult{}, err
		}
		result.Restored = true
	}

	if result.OnBase {
		result.Ahead, result.Behind = 0, 0
		return result, nil
	}

	ahead, behind, err := s.repo.AheadBehind(ctx, dir, result.Base, result.Branch)
	if err != nil {
		return SyncResult{}, err
	}
	result.Ahead, result.Behind = ahead, behind
	return result, nil
}

// abortPreserve restores a stash created before a later step failed, so a
// failed fetch or base update does not leave the user's work hidden.
func (s *Service) abortPreserve(ctx context.Context, dir string, result SyncResult, cause error) (SyncResult, error) {
	if !result.Preserved {
		return SyncResult{}, cause
	}
	if err := s.repo.RestoreChanges(ctx, dir); err != nil {
		return SyncResult{}, fmt.Errorf("%v\n\nalso failed to restore preserved changes: %w", cause, err)
	}
	return SyncResult{}, cause
}

// updateBaseFromRemote fast-forwards the local base to the remote-tracking tip
// when that is safe. Divergent history stops the workflow. A missing remote
// branch is skipped (the local base is left as-is).
func (s *Service) updateBaseFromRemote(ctx context.Context, dir, current, base, remote string) (bool, error) {
	exists, err := s.repo.RemoteBranchExists(ctx, dir, remote, base)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	tracking := git.RemoteTrackingRef(remote, base)

	// AheadBehind(tracking, base): ahead = local-only commits, behind = remote-only.
	localAhead, localBehind, err := s.repo.AheadBehind(ctx, dir, tracking, base)
	if err != nil {
		return false, err
	}

	switch {
	case localAhead == 0 && localBehind == 0:
		return false, nil
	case localAhead == 0 && localBehind > 0:
		if current == base {
			if err := s.repo.MergeFastForward(ctx, dir, tracking); err != nil {
				return false, fmt.Errorf("updating %s: %w", base, err)
			}
		} else {
			if err := s.repo.FastForwardBranch(ctx, dir, base, tracking); err != nil {
				return false, fmt.Errorf("updating %s: %w", base, err)
			}
		}
		return true, nil
	case localAhead > 0 && localBehind == 0:
		return false, nil
	default:
		return false, &DivergentBaseError{
			Base:   base,
			Remote: remote,
			Ahead:  localAhead,
			Behind: localBehind,
		}
	}
}

// syncBaseBranch picks the base branch for synchronisation.
//
// Priority: configured default → remote default (when detectable) → main → master.
func (s *Service) syncBaseBranch(ctx context.Context, dir string, local []string) (string, error) {
	candidates := s.syncBaseCandidates(ctx, dir)
	for _, candidate := range candidates {
		if contains(local, candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot find a base branch: none of %s exist", list(candidates))
}

// syncBaseCandidates lists base names to try, in priority order.
func (s *Service) syncBaseCandidates(ctx context.Context, dir string) []string {
	var candidates []string
	add := func(name string) {
		if name != "" && !contains(candidates, name) {
			candidates = append(candidates, name)
		}
	}

	add(s.opts.DefaultBaseBranch)

	remotes, err := s.repo.Remotes(ctx, dir)
	if err == nil && len(remotes) > 0 {
		if remote, err := chooseRemote(remotes); err == nil {
			if def, err := s.repo.RemoteDefaultBranch(ctx, dir, remote); err == nil {
				add(def)
			}
		}
	}

	add("main")
	add("master")
	return candidates
}
