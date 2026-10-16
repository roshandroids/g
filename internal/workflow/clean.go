package workflow

import (
	"context"
	"fmt"
	"sort"

	"github.com/roshandroids/g/internal/git"
)

// CleanRequest configures a clean preview or apply.
type CleanRequest struct {
	// Apply deletes the stale branches after confirmation at the command edge.
	Apply bool
	// Force allows `git branch -D` for branches that are not safely deletable.
	// It is unused in v0.5; force deletion is reserved for a later deliberate
	// confirmation path.
	Force bool
}

// CleanResult lists stale local branches and, when Apply is set, which ones were
// deleted.
type CleanResult struct {
	// Base is the base branch used for merged detection.
	Base string
	// Current is the current branch, which is never deleted.
	Current string
	// Candidates are the stale local branches.
	Candidates []git.StaleBranch
	// Deleted are the branches removed when Apply was set.
	Deleted []string
	// Applied reports whether deletion was requested.
	Applied bool
}

// Clean identifies stale local branches and optionally deletes them.
//
// Preview is the default. Deletion never targets main, master, the configured
// base, or the current branch. Remote branches are never deleted.
func (s *Service) Clean(ctx context.Context, dir string, req CleanRequest) (CleanResult, error) {
	state, err := s.requireIdle(ctx, dir)
	if err != nil {
		return CleanResult{}, err
	}

	local, err := s.repo.LocalBranches(ctx, dir)
	if err != nil {
		return CleanResult{}, err
	}

	base, err := s.baseBranch(local)
	if err != nil {
		return CleanResult{}, err
	}

	result := CleanResult{
		Base:    base,
		Current: state.Branch,
		Applied: req.Apply,
	}

	candidates, err := s.staleBranches(ctx, dir, state.Branch, base, local)
	if err != nil {
		return CleanResult{}, err
	}
	result.Candidates = candidates

	if !req.Apply {
		return result, nil
	}

	for _, candidate := range candidates {
		if !candidate.Safe && !req.Force {
			continue
		}
		if err := s.repo.DeleteBranch(ctx, dir, candidate.Name, req.Force && !candidate.Safe); err != nil {
			return result, fmt.Errorf("deleting %s: %w", candidate.Name, err)
		}
		result.Deleted = append(result.Deleted, candidate.Name)
	}

	return result, nil
}

func (s *Service) staleBranches(ctx context.Context, dir, current, base string, local []string) ([]git.StaleBranch, error) {
	protected := s.protectedBranches()
	seen := map[string]git.StaleBranch{}

	gone, err := s.repo.GoneBranches(ctx, dir)
	if err != nil {
		return nil, err
	}
	for _, name := range gone {
		if isProtectedBranch(name, current, protected) {
			continue
		}
		seen[name] = git.StaleBranch{Name: name, Reason: git.StaleGone, Safe: true}
	}

	merged, err := s.repo.MergedBranches(ctx, dir, base)
	if err != nil {
		return nil, err
	}
	for _, name := range merged {
		if isProtectedBranch(name, current, protected) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		// A branch listed by --merged is safe to delete with -d.
		seen[name] = git.StaleBranch{Name: name, Reason: git.StaleMerged, Safe: true}
	}

	_ = local

	candidates := make([]git.StaleBranch, 0, len(seen))
	for _, branch := range seen {
		candidates = append(candidates, branch)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})
	return candidates, nil
}

func (s *Service) protectedBranches() []string {
	var names []string
	for _, name := range []string{s.opts.DefaultBaseBranch, "main", "master"} {
		if name != "" && !contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

func isProtectedBranch(name, current string, protected []string) bool {
	if name == current {
		return true
	}
	return contains(protected, name)
}
