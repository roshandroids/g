package git

import "context"

// PushOptions describes how a branch is pushed.
//
// There is deliberately no way to ask for a plain --force. A force push can
// only be expressed as --force-with-lease, which refuses to overwrite a remote
// branch that has moved since the last fetch, so the dangerous form is not
// reachable from this API at all.
type PushOptions struct {
	// Remote is the remote to push to. Empty pushes to the branch's configured
	// upstream.
	Remote string
	// Ref is the revision to push, for example "HEAD". Empty pushes the
	// current branch.
	Ref string
	// SetUpstream records the pushed branch as the current branch's upstream.
	SetUpstream bool
	// ForceWithLease replaces the remote branch only if it still matches the
	// remote-tracking ref.
	ForceWithLease bool
}

// args renders the git arguments for these options.
func (o PushOptions) args() []string {
	args := []string{"push"}
	if o.ForceWithLease {
		args = append(args, "--force-with-lease")
	}
	if o.SetUpstream {
		args = append(args, "--set-upstream")
	}
	if o.Remote != "" {
		args = append(args, o.Remote)
	}
	if o.Ref != "" {
		args = append(args, o.Ref)
	}
	return args
}

// Push sends the current branch to its remote.
//
// A rejected push is reported as a git failure, not worked around: when the
// remote has moved on, the answer is to fetch and integrate, and g does not get
// to decide that on the user's behalf.
func (c *Client) Push(ctx context.Context, dir string, opts PushOptions) error {
	return c.run(ctx, dir, opts.args()...)
}
