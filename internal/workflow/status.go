package workflow

import (
	"path/filepath"

	"github.com/roshandroids/g/internal/git"
)

// Change is a path that differs from the index or the working tree.
type Change struct {
	// Path is the current path, relative to the repository root.
	Path string
	// OriginalPath is the previous path of a rename or copy.
	OriginalPath string
	// Kind classifies the difference.
	Kind git.ChangeKind
}

// Upstream is the branch the current branch tracks.
type Upstream struct {
	// Name is the tracking branch, for example "origin/main".
	Name string
	// Ahead counts local commits the upstream lacks.
	Ahead int
	// Behind counts upstream commits the local branch lacks.
	Behind int
}

// Status is the application view of a repository, ready to present.
type Status struct {
	// Name is the repository directory name, used for display.
	Name string
	// Root is the absolute path of the work tree.
	Root string
	// Branch is the current branch; empty when HEAD is detached.
	Branch string
	// Detached is set when HEAD points at a commit instead of a branch.
	Detached bool
	// Head is the abbreviated commit HEAD points at when detached.
	Head string
	// Operation is the Git operation currently paused in the repository, if
	// any.
	//
	// A rebase, cherry-pick or revert leaves HEAD detached, so this is what
	// tells "you are looking at a commit" apart from "you are part way through
	// something".
	Operation git.Operation
	// Upstream is nil when the current branch tracks no other branch.
	Upstream *Upstream
	// Staged holds index changes.
	Staged []Change
	// Unstaged holds working tree changes to tracked files.
	Unstaged []Change
	// Untracked holds paths git does not track.
	Untracked []string
	// Conflicted holds paths with unresolved merge conflicts.
	Conflicted []string
}

// Clean reports whether nothing is staged, unstaged, untracked or conflicted.
func (s Status) Clean() bool {
	return len(s.Staged) == 0 && len(s.Unstaged) == 0 && len(s.Untracked) == 0 && len(s.Conflicted) == 0
}

// summarize turns parsed git state into the application view of a repository,
// classifying each reported path for presentation.
func summarize(raw git.Status) Status {
	status := Status{
		Name:      filepath.Base(filepath.Clean(raw.Root)),
		Root:      raw.Root,
		Branch:    raw.Branch,
		Detached:  raw.Detached,
		Head:      raw.Head,
		Operation: raw.Operation,
	}

	if raw.HasUpstream() {
		status.Upstream = &Upstream{Name: raw.Upstream, Ahead: raw.Ahead, Behind: raw.Behind}
	}

	for _, file := range raw.Files {
		switch {
		case file.Untracked:
			status.Untracked = append(status.Untracked, file.Path)
		case file.Unmerged:
			status.Conflicted = append(status.Conflicted, file.Path)
		default:
			if file.Index != "" {
				status.Staged = append(status.Staged, Change{
					Path:         file.Path,
					OriginalPath: file.OriginalPath,
					Kind:         file.Index,
				})
			}
			if file.Worktree != "" {
				status.Unstaged = append(status.Unstaged, Change{
					Path:         file.Path,
					OriginalPath: file.OriginalPath,
					Kind:         file.Worktree,
				})
			}
		}
	}

	return status
}
