package workflow

import (
	"context"

	"github.com/roshandroids/g/internal/git"
)

// PushRequest describes how to push the current branch.
type PushRequest struct {
	// Force replaces the remote branch using --force-with-lease.
	//
	// Setting it is an assertion that the user was asked in so many words. The
	// workflow cannot verify that, so it never sets the flag itself and never
	// escalates to a plain --force.
	Force bool
}

// PushResult describes the push that happened.
type PushResult struct {
	// Branch is the branch that was pushed.
	Branch string
	// Upstream is the remote-tracking branch it now tracks.
	Upstream string
	// SetUpstream reports that this push created the upstream.
	SetUpstream bool
	// Forced reports that --force-with-lease was used.
	Forced bool
}

// Push sends the current branch to its remote.
//
// A branch that already tracks a remote is pushed to it. A branch that does not
// is published with `git push -u <remote> HEAD`, choosing "origin" when it
// exists and the sole remote otherwise, so the user does not have to type the
// remote and branch name.
//
// Force pushes are never automatic and never plain: the strongest thing
// available is --force-with-lease, which still refuses to overwrite a remote
// branch that has moved since the last fetch. A force push with no upstream is
// refused, because there would be nothing to compare against.
func (s *Service) Push(ctx context.Context, dir string, req PushRequest) (PushResult, error) {
	state, err := s.requireIdle(ctx, dir)
	if err != nil {
		return PushResult{}, err
	}

	remotes, err := s.repo.Remotes(ctx, dir)
	if err != nil {
		return PushResult{}, err
	}
	if len(remotes) == 0 {
		return PushResult{}, &NoRemoteError{}
	}

	if state.HasUpstream() {
		return s.pushToUpstream(ctx, dir, state, req)
	}

	// A force push has no remote branch to compare against, so it could only be
	// a plain overwrite. Stop without pushing anything.
	if req.Force {
		return PushResult{}, &NoUpstreamError{Branch: state.Branch}
	}

	return s.pushNewUpstream(ctx, dir, state, remotes)
}

// pushToUpstream pushes to the branch's existing upstream.
func (s *Service) pushToUpstream(ctx context.Context, dir string, state git.Status, req PushRequest) (PushResult, error) {
	// Nothing is named on the command line: git already knows which remote and
	// which branch this is, and repeating it could only get it wrong.
	opts := git.PushOptions{ForceWithLease: req.Force}
	if err := s.repo.Push(ctx, dir, opts); err != nil {
		return PushResult{}, err
	}

	return PushResult{
		Branch:   state.Branch,
		Upstream: state.Upstream,
		Forced:   req.Force,
	}, nil
}

// pushNewUpstream publishes the current branch and records the upstream.
func (s *Service) pushNewUpstream(ctx context.Context, dir string, state git.Status, remotes []string) (PushResult, error) {
	remote, err := chooseRemote(remotes)
	if err != nil {
		return PushResult{}, err
	}

	opts := git.PushOptions{Remote: remote, Ref: "HEAD", SetUpstream: true}
	if err := s.repo.Push(ctx, dir, opts); err != nil {
		return PushResult{}, err
	}

	return PushResult{
		Branch:      state.Branch,
		Upstream:    remote + "/" + state.Branch,
		SetUpstream: true,
	}, nil
}

// chooseRemote picks the remote to publish a branch to when git has not been
// told which one to use.
//
// "origin" wins because it is overwhelmingly the one meant, and a sole remote
// is unambiguous. Anything else has more than one right answer, so the user is
// asked to say which rather than being guessed at.
func chooseRemote(remotes []string) (string, error) {
	for _, remote := range remotes {
		if remote == "origin" {
			return remote, nil
		}
	}
	if len(remotes) == 1 {
		return remotes[0], nil
	}
	return "", &AmbiguousRemoteError{Available: remotes}
}
