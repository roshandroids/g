package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// CommitSummary is a short description of one commit.
type CommitSummary struct {
	// Hash is the abbreviated object name.
	Hash string
	// Subject is the first line of the commit message.
	Subject string
}

// StaleBranchReason explains why a local branch is a clean candidate.
type StaleBranchReason string

// Reasons a local branch may be listed by clean.
const (
	// StaleGone means the branch's upstream tracking ref was deleted.
	StaleGone StaleBranchReason = "gone"
	// StaleMerged means the branch is already an ancestor of the base branch.
	StaleMerged StaleBranchReason = "merged"
)

// StaleBranch is a local branch that clean may delete.
type StaleBranch struct {
	// Name is the local branch name.
	Name string
	// Reason is why the branch is considered stale.
	Reason StaleBranchReason
	// Safe reports whether `git branch -d` should succeed.
	Safe bool
}

// AmbiguousOperationError reports that more than one paused operation marker is
// present and the combination is not a known Git overlap.
type AmbiguousOperationError struct {
	// Operations lists the markers that were found.
	Operations []Operation
}

func (e *AmbiguousOperationError) Error() string {
	parts := make([]string, 0, len(e.Operations))
	for _, op := range e.Operations {
		parts = append(parts, string(op))
	}
	return fmt.Sprintf("ambiguous Git state: multiple operations in progress (%s); resolve them with git before continuing",
		strings.Join(parts, ", "))
}

// Continue resumes a paused Git operation.
//
// Conflicts and other Git failures are returned as-is; the repository is left
// exactly where Git stopped. An interactive editor is never opened: continue
// keeps the existing commit message via a no-op editor.
func (c *Client) Continue(ctx context.Context, dir string, op Operation) error {
	args, err := continueArgs(op)
	if err != nil {
		return err
	}
	prefixed := append([]string{"-c", "core.editor=true", "-c", "sequence.editor=true"}, args...)
	return c.run(ctx, dir, prefixed...)
}

// Abort cancels a paused Git operation.
func (c *Client) Abort(ctx context.Context, dir string, op Operation) error {
	args, err := abortArgs(op)
	if err != nil {
		return err
	}
	return c.run(ctx, dir, args...)
}

// SoftReset moves HEAD back by count commits while keeping all changes staged.
//
// This is the only reset shape g exposes for undo. Hard resets are never used.
func (c *Client) SoftReset(ctx context.Context, dir string, count int) error {
	if count < 1 {
		return fmt.Errorf("reset count must be at least 1")
	}
	return c.run(ctx, dir, "reset", "--soft", "HEAD~"+strconv.Itoa(count))
}

// RecentCommits returns the most recent count commits reachable from HEAD.
func (c *Client) RecentCommits(ctx context.Context, dir string, count int) ([]CommitSummary, error) {
	if count < 1 {
		return nil, fmt.Errorf("commit count must be at least 1")
	}

	out, err := c.output(ctx, dir, "log", "-n", strconv.Itoa(count), "--format=%h\t%s")
	if err != nil {
		return nil, err
	}

	var commits []CommitSummary
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hash, subject, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("unexpected git log line %q", line)
		}
		commits = append(commits, CommitSummary{Hash: hash, Subject: subject})
	}
	return commits, nil
}

// CommitCount reports how many commits are reachable from HEAD.
func (c *Client) CommitCount(ctx context.Context, dir string) (int, error) {
	out, err := c.line(ctx, dir, "rev-list", "--count", "HEAD")
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("parsing commit count %q: %w", out, err)
	}
	return n, nil
}

// GoneBranches lists local branches whose upstream tracking branch was deleted.
func (c *Client) GoneBranches(ctx context.Context, dir string) ([]string, error) {
	out, err := c.output(ctx, dir, "for-each-ref", "--format=%(refname:short)%00%(upstream:track)", "refs/heads")
	if err != nil {
		return nil, err
	}

	var gone []string
	for _, entry := range strings.Split(out, "\n") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, track, ok := strings.Cut(entry, "\x00")
		if !ok {
			name, track, ok = strings.Cut(entry, " ")
			if !ok {
				continue
			}
		}
		if strings.Contains(track, "[gone]") {
			gone = append(gone, name)
		}
	}
	return gone, nil
}

// MergedBranches lists local branches already merged into base.
//
// base itself is included by git and must be filtered by the caller.
func (c *Client) MergedBranches(ctx context.Context, dir, base string) ([]string, error) {
	out, err := c.output(ctx, dir, "branch", "--format=%(refname:short)", "--merged", base)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// DeleteBranch deletes a local branch.
//
// When force is false, git refuses branches that are not fully merged (`-d`).
// Force uses `-D` and is reserved for an explicit user confirmation path.
func (c *Client) DeleteBranch(ctx context.Context, dir, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return c.run(ctx, dir, "branch", flag, name)
}

func continueArgs(op Operation) ([]string, error) {
	switch op {
	case OperationRebase:
		return []string{"rebase", "--continue"}, nil
	case OperationMerge:
		return []string{"merge", "--continue"}, nil
	case OperationCherryPick:
		return []string{"cherry-pick", "--continue"}, nil
	case OperationRevert:
		return []string{"revert", "--continue"}, nil
	case OperationNone:
		return nil, fmt.Errorf("no Git operation is currently in progress")
	default:
		return nil, fmt.Errorf("cannot continue unknown operation %q", op)
	}
}

func abortArgs(op Operation) ([]string, error) {
	switch op {
	case OperationRebase:
		return []string{"rebase", "--abort"}, nil
	case OperationMerge:
		return []string{"merge", "--abort"}, nil
	case OperationCherryPick:
		return []string{"cherry-pick", "--abort"}, nil
	case OperationRevert:
		return []string{"revert", "--abort"}, nil
	case OperationNone:
		return nil, fmt.Errorf("no Git operation is currently in progress")
	default:
		return nil, fmt.Errorf("cannot abort unknown operation %q", op)
	}
}
