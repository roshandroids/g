package git

import (
	"fmt"
	"strings"
)

// ChangeKind classifies how a path differs from the index or the working tree.
type ChangeKind string

// The kinds git reports for a path. An empty kind means the path is unchanged
// on that side.
const (
	Modified    ChangeKind = "modified"
	Added       ChangeKind = "added"
	Deleted     ChangeKind = "deleted"
	Renamed     ChangeKind = "renamed"
	Copied      ChangeKind = "copied"
	TypeChanged ChangeKind = "typechanged"
	Unmerged    ChangeKind = "unmerged"
)

// FileChange is one path reported by `git status --porcelain`.
//
// Index describes the staged state and Worktree the unstaged state; either is
// empty when the path is unchanged on that side. Any other combination git
// considers a conflict sets Unmerged and is reported through neither side.
type FileChange struct {
	// Path is the current path, relative to the repository root.
	Path string
	// OriginalPath is the previous path of a rename or copy.
	OriginalPath string
	// Index is the staged change.
	Index ChangeKind
	// Worktree is the unstaged change.
	Worktree ChangeKind
	// Untracked marks a path git does not track.
	Untracked bool
	// Unmerged marks a path with an unresolved merge conflict.
	Unmerged bool
}

// Status is the parsed state of a repository, as reported by git itself.
type Status struct {
	// Root is the absolute path of the work tree.
	Root string
	// Branch is the current branch; empty when HEAD is detached.
	Branch string
	// Detached is set when HEAD points at a commit instead of a branch.
	Detached bool
	// Head is the abbreviated commit HEAD points at when detached.
	Head string
	// Upstream is the tracking branch of the current branch; empty when the
	// branch tracks nothing.
	Upstream string
	// Ahead counts commits the local branch has that the upstream lacks.
	Ahead int
	// Behind counts commits the upstream has that the local branch lacks.
	Behind int
	// Files are the paths git reports as changed.
	Files []FileChange
}

// HasUpstream reports whether the current branch tracks another branch.
func (s Status) HasUpstream() bool { return s.Upstream != "" }

// unmergedPairs are the two-letter codes git uses for unresolved conflicts.
//
// git-status(1) documents DD, AU, UD, UA, DU, AA and UU as unmerged; the
// remaining codes always involve at least one clean side.
var unmergedPairs = map[string]bool{
	"DD": true,
	"AU": true,
	"UD": true,
	"UA": true,
	"DU": true,
	"AA": true,
	"UU": true,
}

// parseStatus parses NUL-separated `git status --porcelain -z` output.
//
// Each field is "XY PATH", where X is the index status and Y the working tree
// status. Rename and copy entries carry the original path in the field that
// follows, which is why the output must be read as NUL-separated rather than by
// line.
func parseStatus(output string) ([]FileChange, error) {
	fields := strings.Split(output, "\x00")

	var changes []FileChange
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if entry == "" {
			continue
		}
		if len(entry) < 4 {
			return nil, fmt.Errorf("malformed status entry %q", entry)
		}

		code, path := entry[:2], entry[3:]
		change := FileChange{Path: path}

		switch {
		case code == "??":
			change.Untracked = true
		case code == "!!":
			// Ignored paths are not part of the requested output.
			continue
		case unmergedPairs[code]:
			change.Unmerged = true
			change.Index, change.Worktree = Unmerged, Unmerged
		default:
			change.Index = kindOf(code[0])
			change.Worktree = kindOf(code[1])
			if code[0] == 'R' || code[0] == 'C' {
				i++
				if i >= len(fields) || fields[i] == "" {
					return nil, fmt.Errorf("malformed status entry %q: missing original path", entry)
				}
				change.OriginalPath = fields[i]
			}
		}

		changes = append(changes, change)
	}

	return changes, nil
}

// kindOf maps a single porcelain status code to a change kind.
func kindOf(code byte) ChangeKind {
	switch code {
	case 'M':
		return Modified
	case 'A':
		return Added
	case 'D':
		return Deleted
	case 'R':
		return Renamed
	case 'C':
		return Copied
	case 'T':
		return TypeChanged
	case 'U':
		return Unmerged
	default:
		return ""
	}
}
