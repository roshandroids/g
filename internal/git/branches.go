package git

import (
	"context"
	"sort"
	"strings"
)

// operationMarkers are the refs git creates while an operation is paused.
//
// They are ordinary refs, so asking git to resolve them keeps g out of the
// .git directory entirely. The order is the precedence used when more than one
// is present, which happens while a rebase replays commits onto a merge.
var operationMarkers = []struct {
	ref       string
	operation Operation
}{
	{"REBASE_HEAD", OperationRebase},
	{"MERGE_HEAD", OperationMerge},
	{"CHERRY_PICK_HEAD", OperationCherryPick},
	{"REVERT_HEAD", OperationRevert},
}

// operation reports the Git operation currently paused in the repository.
//
// `rev-parse -q --verify` exits non-zero and stays silent when the ref does not
// exist, which is precisely the "no operation in progress" case.
func (c *Client) operation(ctx context.Context, dir string) (Operation, error) {
	for _, marker := range operationMarkers {
		result, err := c.exec(ctx, dir, "rev-parse", "-q", "--verify", marker.ref)
		if err != nil {
			return OperationNone, err
		}
		if !result.Failed() {
			return marker.operation, nil
		}
	}
	return OperationNone, nil
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
