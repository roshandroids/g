package git

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// operationMarkers are the refs git creates while an operation is paused.
//
// They are ordinary refs, so asking git to resolve them keeps g out of the
// .git directory for merge, cherry-pick and revert. Rebase is detected
// separately: REBASE_HEAD can linger after a successful rebase finishes, so
// the rebase-merge / rebase-apply directories are the reliable signal.
var operationMarkers = []struct {
	ref       string
	operation Operation
}{
	{"MERGE_HEAD", OperationMerge},
	{"CHERRY_PICK_HEAD", OperationCherryPick},
	{"REVERT_HEAD", OperationRevert},
}

// operation reports the single Git operation currently paused in the
// repository, if any.
//
// It consults every marker so unexpected overlaps are not silently ignored.
// Rebase of a merge commit legitimately leaves both a rebase state and
// MERGE_HEAD; that combination resolves as a rebase. Any other combination is
// reported as AmbiguousOperationError.
func (c *Client) operation(ctx context.Context, dir string) (Operation, error) {
	ops, err := c.PausedOperations(ctx, dir)
	if err != nil {
		return OperationNone, err
	}
	return ResolveOperation(ops)
}

// PausedOperations lists every paused Git operation marker present in dir.
//
// status, continue, abort and sync all share this detection so they cannot
// disagree about what is in progress.
func (c *Client) PausedOperations(ctx context.Context, dir string) ([]Operation, error) {
	var found []Operation

	rebase, err := c.rebaseInProgress(ctx, dir)
	if err != nil {
		return nil, err
	}
	if rebase {
		found = append(found, OperationRebase)
	}

	for _, marker := range operationMarkers {
		result, err := c.exec(ctx, dir, "rev-parse", "-q", "--verify", marker.ref)
		if err != nil {
			return nil, err
		}
		if !result.Failed() {
			found = append(found, marker.operation)
		}
	}
	return found, nil
}

// rebaseInProgress reports whether a rebase is paused in dir.
//
// git names the state directories through `rev-parse --git-path`; existence of
// either directory is the same signal git itself uses. REBASE_HEAD alone is
// not enough — it can remain as a stale file after a successful continue.
func (c *Client) rebaseInProgress(ctx context.Context, dir string) (bool, error) {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		path, err := c.line(ctx, dir, "rev-parse", "--git-path", name)
		if err != nil {
			return false, err
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, err
		}
		if info.IsDir() {
			return true, nil
		}
	}
	return false, nil
}

// ResolveOperation turns the set of paused markers into a single operation.
//
// An empty set means the repository is idle. Rebase+merge is the known overlap
// Git creates while rebasing a merge commit. Anything else is ambiguous and
// must not be guessed at by continue or abort.
func ResolveOperation(ops []Operation) (Operation, error) {
	switch len(ops) {
	case 0:
		return OperationNone, nil
	case 1:
		return ops[0], nil
	case 2:
		if hasOperation(ops, OperationRebase) && hasOperation(ops, OperationMerge) {
			return OperationRebase, nil
		}
		fallthrough
	default:
		return OperationNone, &AmbiguousOperationError{Operations: append([]Operation(nil), ops...)}
	}
}

func hasOperation(ops []Operation, want Operation) bool {
	for _, op := range ops {
		if op == want {
			return true
		}
	}
	return false
}

// LocalBranches lists the names of the repository's local branches.
//
// for-each-ref is used rather than `branch --list` because its output is
// documented as stable and it does not decorate or paginate.
func (c *Client) LocalBranches(ctx context.Context, dir string) ([]string, error) {
	out, err := c.output(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// Remotes lists the names of the configured remotes.
func (c *Client) Remotes(ctx context.Context, dir string) ([]string, error) {
	out, err := c.output(ctx, dir, "remote")
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// RemoteBranchExists reports whether remote already has a branch called name.
//
// A missing remote is reported as false rather than as an error: the caller is
// asking about a branch, and an unconfigured remote simply has none.
func (c *Client) RemoteBranchExists(ctx context.Context, dir, remote, name string) (bool, error) {
	out, err := c.output(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/remotes/"+remote+"/"+name)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// CreateBranch creates name pointing at base and switches to it.
//
// `switch -c` carries uncommitted changes across to the new branch and never
// discards them, which is why the caller is expected to have warned about them
// rather than to have blocked on them.
func (c *Client) CreateBranch(ctx context.Context, dir, name, base string) error {
	return c.run(ctx, dir, "switch", "-c", name, base)
}

// SwitchBranch switches to an existing branch.
//
// git itself refuses to switch when that would overwrite local changes, so a
// dirty tree fails loudly instead of losing work.
func (c *Client) SwitchBranch(ctx context.Context, dir, name string) error {
	return c.run(ctx, dir, "switch", name)
}

// run executes git for its effect, reporting any non-zero status as an error.
func (c *Client) run(ctx context.Context, dir string, args ...string) error {
	_, err := c.output(ctx, dir, args...)
	return err
}

// splitLines splits git's line-oriented output into non-empty entries.
//
// The result is sorted so callers get a stable order regardless of how git
// happens to emit refs.
func splitLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	sort.Strings(lines)
	return lines
}
