// Package git is a thin, testable wrapper around the installed git executable.
//
// It never reads or writes .git internals: every fact it reports comes from
// running git and parsing its documented, machine-readable output.
package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/roshandroids/g/internal/process"
)

// Client reads repository state by executing git.
type Client struct {
	runner process.Runner
}

// NewClient returns a Client that runs git through runner.
func NewClient(runner process.Runner) *Client {
	return &Client{runner: runner}
}

// Status inspects the repository containing dir.
//
// It returns ErrNotARepository when dir is not inside a work tree, a
// *CommandError when git fails, and the wrapped process error when git cannot
// be executed at all.
func (c *Client) Status(ctx context.Context, dir string) (Status, error) {
	root, err := c.line(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		if errors.Is(err, ErrNotARepository) {
			return Status{}, fmt.Errorf("%w: %s", ErrNotARepository, dir)
		}
		return Status{}, err
	}

	status := Status{Root: root}

	if status.Branch, status.Detached, status.Head, err = c.head(ctx, root); err != nil {
		return Status{}, err
	}
	if status.Upstream, err = c.upstream(ctx, root); err != nil {
		return Status{}, err
	}
	if status.HasUpstream() {
		if status.Ahead, status.Behind, err = c.aheadBehind(ctx, root); err != nil {
			return Status{}, err
		}
	}
	if status.Files, err = c.files(ctx, root); err != nil {
		return Status{}, err
	}

	return status, nil
}

// head reports the branch HEAD points at. When HEAD is detached it reports an
// empty branch, sets detached, and resolves the abbreviated commit.
//
// git exits non-zero for `symbolic-ref` exactly when HEAD is not a symbolic
// reference, which is the detached case.
func (c *Client) head(ctx context.Context, dir string) (branch string, detached bool, head string, err error) {
	result, err := c.exec(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return "", false, "", err
	}
	if !result.Failed() {
		return strings.TrimSpace(result.Stdout), false, "", nil
	}

	commit, err := c.line(ctx, dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", false, "", err
	}
	return "", true, commit, nil
}

// upstream reports the branch the current branch tracks.
//
// git fails both when no upstream is configured and when HEAD is detached;
// neither is an error for status, so both are reported as "no upstream".
func (c *Client) upstream(ctx context.Context, dir string) (string, error) {
	result, err := c.exec(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return "", err
	}
	if result.Failed() {
		return "", nil
	}
	return strings.TrimSpace(result.Stdout), nil
}

// aheadBehind counts the commits the local branch and its upstream differ by.
func (c *Client) aheadBehind(ctx context.Context, dir string) (ahead, behind int, err error) {
	out, err := c.output(ctx, dir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
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

// files lists the paths git reports as changed.
func (c *Client) files(ctx context.Context, dir string) ([]FileChange, error) {
	out, err := c.output(ctx, dir, "status", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parseStatus(out)
}

// exec runs git and returns the captured result. An error means git could not
// be executed; a non-zero status is reported through the result.
func (c *Client) exec(ctx context.Context, dir string, args ...string) (process.Result, error) {
	spec := process.Spec{Dir: dir, Name: "git", Args: args}
	result, err := c.runner.Run(ctx, spec)
	if err != nil {
		return result, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return result, nil
}

// output runs git and returns its stdout untouched, translating a non-zero
// status into an error.
//
// The output is deliberately not trimmed: porcelain status output begins with a
// space whenever the change is unstaged, so trimming would corrupt every entry.
func (c *Client) output(ctx context.Context, dir string, args ...string) (string, error) {
	result, err := c.exec(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	if result.Failed() {
		if isNotARepository(result.Stderr) {
			return "", ErrNotARepository
		}
		return "", &CommandError{Args: args, ExitCode: result.ExitCode, Stderr: result.Stderr}
	}
	return result.Stdout, nil
}

// line runs git and returns its single-value stdout with surrounding whitespace
// removed.
func (c *Client) line(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := c.output(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// isNotARepository reports whether git refused to run outside a work tree.
//
// Detection is textual because git does not expose a stable exit code for this
// condition; the message has been stable since git 1.7.
func isNotARepository(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "not a git repository")
}
