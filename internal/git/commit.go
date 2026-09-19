package git

import "context"

// Commit records the staged changes with message.
//
// Only what is already in the index is committed: g never stages files on the
// user's behalf. Everything else about the commit is git's own behaviour, so
// hooks run and a commit that completes a merge is recorded as a merge.
func (c *Client) Commit(ctx context.Context, dir, message string) error {
	return c.run(ctx, dir, "commit", "-m", message)
}
