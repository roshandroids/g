package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// StashRef is the name of a stash entry, for example "stash@{0}".
type StashRef string

// PreserveResult describes a temporary stash created to free the working tree.
type PreserveResult struct {
	// Created reports that a stash entry was written.
	Created bool
	// Ref names the stash entry when Created is true.
	Ref StashRef
}

// Fetch updates remote-tracking refs from remote.
//
// A network, authentication or unreachable-remote failure is reported as an
// error. Callers must not pretend the remote was consulted when Fetch fails.
func (c *Client) Fetch(ctx context.Context, dir, remote string) error {
	return c.run(ctx, dir, "fetch", "--prune", remote)
}

// RemoteDefaultBranch reports the branch origin/HEAD (or remote/HEAD) points at.
//
// The empty string means the remote default could not be determined, which is
// normal before the first fetch or when the remote never advertised HEAD.
func (c *Client) RemoteDefaultBranch(ctx context.Context, dir, remote string) (string, error) {
	out, err := c.line(ctx, dir, "symbolic-ref", "--short", "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		var cmdErr *CommandError
		if errors.As(err, &cmdErr) {
			return "", nil
		}
		return "", err
	}

	prefix := remote + "/"
	if !strings.HasPrefix(out, prefix) {
		return "", nil
	}
	return strings.TrimPrefix(out, prefix), nil
}

// AheadBehind counts commits unique to left and right across left...right.
//
// Behind is commits reachable from left but not right; Ahead is commits
// reachable from right but not left. For a feature branch compared to its base,
// call AheadBehind(dir, base, feature): Behind > 0 means the feature needs a
// rebase onto base.
func (c *Client) AheadBehind(ctx context.Context, dir, left, right string) (ahead, behind int, err error) {
	out, err := c.output(ctx, dir, "rev-list", "--left-right", "--count", left+"..."+right)
	if err != nil {
		return 0, 0, err
	}

	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected git rev-list output %q", strings.TrimSpace(out))
	}

	if behind, err = strconv.Atoi(fields[0]); err != nil {
		return 0, 0, fmt.Errorf("parsing git rev-list output %q: %w", strings.TrimSpace(out), err)
	}
	if ahead, err = strconv.Atoi(fields[1]); err != nil {
		return 0, 0, fmt.Errorf("parsing git rev-list output %q: %w", strings.TrimSpace(out), err)
	}
	return ahead, behind, nil
}

// RevParse resolves rev to a commit object name.
func (c *Client) RevParse(ctx context.Context, dir, rev string) (string, error) {
	return c.line(ctx, dir, "rev-parse", rev)
}

// IsAncestor reports whether ancestor is an ancestor of rev.
func (c *Client) IsAncestor(ctx context.Context, dir, ancestor, rev string) (bool, error) {
	result, err := c.exec(ctx, dir, "merge-base", "--is-ancestor", ancestor, rev)
	if err != nil {
		return false, err
	}
	switch result.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, &CommandError{Args: []string{"merge-base", "--is-ancestor", ancestor, rev}, ExitCode: result.ExitCode, Stderr: result.Stderr}
	}
}

// FastForwardBranch moves branch to tip when the update is a fast-forward.
//
// branch must not be the currently checked-out branch; use MergeFastForward for
// that case so the index and working tree stay consistent with HEAD.
func (c *Client) FastForwardBranch(ctx context.Context, dir, branch, tip string) error {
	ok, err := c.IsAncestor(ctx, dir, branch, tip)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cannot fast-forward %q to %q: not a fast-forward", branch, tip)
	}
	return c.run(ctx, dir, "update-ref", "refs/heads/"+branch, tip)
}

// MergeFastForward fast-forwards the current branch to rev.
func (c *Client) MergeFastForward(ctx context.Context, dir, rev string) error {
	return c.run(ctx, dir, "merge", "--ff-only", rev)
}

// Rebase replays the current branch onto onto.
//
// A conflict leaves the repository paused in a rebase and returns
// *RebaseConflictError. The rebase is not aborted.
func (c *Client) Rebase(ctx context.Context, dir, onto string) error {
	result, err := c.exec(ctx, dir, "rebase", onto)
	if err != nil {
		return err
	}
	if !result.Failed() {
		return nil
	}

	op, opErr := c.operation(ctx, dir)
	if opErr != nil {
		return opErr
	}
	if op == OperationRebase {
		return &RebaseConflictError{}
	}
	return &CommandError{Args: []string{"rebase", onto}, ExitCode: result.ExitCode, Stderr: result.Stderr}
}

// PreserveChanges stashes local modifications so a branch switch or rebase can
// proceed. Untracked files are included when present.
//
// Nothing is discarded: a failure leaves the working tree as it was. An empty
// working tree yields Created=false without running stash.
func (c *Client) PreserveChanges(ctx context.Context, dir, message string) (PreserveResult, error) {
	status, err := c.Status(ctx, dir)
	if err != nil {
		return PreserveResult{}, err
	}
	if status.Clean() {
		return PreserveResult{}, nil
	}

	args := []string{"stash", "push", "-m", message}
	if hasUntracked(status) {
		// -u includes untracked files. -a (all) would also stash ignored files,
		// which surprises people and is not required here.
		args = append(args, "--include-untracked")
	}

	if err := c.run(ctx, dir, args...); err != nil {
		return PreserveResult{}, err
	}

	after, err := c.Status(ctx, dir)
	if err != nil {
		return PreserveResult{}, err
	}
	if !after.Clean() {
		return PreserveResult{}, fmt.Errorf("failed to preserve local changes: working tree still dirty after stash")
	}

	ref, err := c.line(ctx, dir, "rev-parse", "--short", "stash@{0}")
	if err != nil {
		return PreserveResult{}, fmt.Errorf("preserved changes but cannot identify stash entry: %w", err)
	}
	return PreserveResult{Created: true, Ref: StashRef("stash@{0} (" + ref + ")")}, nil
}

// RestoreChanges applies the most recent stash and drops it only on success.
//
// On conflict the stash entry is left intact and *StashRestoreError is returned
// so the user can recover manually.
func (c *Client) RestoreChanges(ctx context.Context, dir string) error {
	result, err := c.exec(ctx, dir, "stash", "pop")
	if err != nil {
		return err
	}
	if !result.Failed() {
		return nil
	}

	// stash pop refuses to drop when the apply has conflicts, so the entry is
	// still there. Surface that explicitly rather than as a generic git error.
	return &StashRestoreError{Detail: strings.TrimSpace(result.Stderr)}
}

// RemoteTrackingRef builds the remote-tracking ref name for a branch.
func RemoteTrackingRef(remote, branch string) string {
	return remote + "/" + branch
}

// RebaseConflictError reports that a rebase stopped with conflicts.
type RebaseConflictError struct{}

func (e *RebaseConflictError) Error() string {
	return strings.TrimSpace(`Rebase stopped because of conflicts.

Resolve the conflicts, then run:
  git add <files>
  git rebase --continue

Or abort with:
  git rebase --abort`)
}

// StashRestoreError reports that restoring preserved changes left conflicts.
//
// The stash entry is kept so nothing is lost.
type StashRestoreError struct {
	// Detail is git's explanation, when available.
	Detail string
}

func (e *StashRestoreError) Error() string {
	message := "restoring preserved local changes caused conflicts; the stash entry was kept"
	if e.Detail != "" {
		message += ": " + e.Detail
	}
	message += "\n\nResolve the conflicts, then drop the stash with:\n  git stash drop"
	return message
}

func hasUntracked(status Status) bool {
	for _, file := range status.Files {
		if file.Untracked {
			return true
		}
	}
	return false
}
