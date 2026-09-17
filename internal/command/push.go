package command

import (
	"context"
	"errors"

	"github.com/roshandroids/g/internal/output"
	"github.com/roshandroids/g/internal/workflow"
)

// errNotPushed reports that the user was asked and said no.
//
// It is an error rather than a quiet success because the thing that was asked
// for did not happen, and a script checking the exit code should not be told
// otherwise.
var errNotPushed = errors.New("nothing was pushed")

// runPush implements `g push`, and `g push --force`.
//
// A push that needs an upstream is discovered without side effects first, so
// the confirmation happens before anything leaves the machine rather than
// halfway through.
func runPush(ctx context.Context, env *Env, args []string) error {
	force, err := parsePushArgs(args)
	if err != nil {
		return err
	}

	if force {
		if err := confirmForce(ctx, env); err != nil {
			return err
		}
	}

	result, err := env.Service.Push(ctx, env.Dir, workflow.PushRequest{Force: force})
	if err == nil {
		return output.RenderPush(env.Out, result)
	}

	// A branch with no upstream is a question rather than a failure: nothing
	// has been pushed, and creating an upstream is a visible change to the
	// remote, so g offers it instead of assuming it.
	if force || !errors.Is(err, workflow.ErrNoUpstream) {
		return err
	}

	confirmed, err := env.Confirm(ctx, "No upstream is set for this branch. Push it and set one?")
	if err != nil {
		return err
	}
	if !confirmed {
		return errNotPushed
	}

	result, err = env.Service.Push(ctx, env.Dir, workflow.PushRequest{CreateUpstream: true})
	if err != nil {
		return err
	}

	return output.RenderPush(env.Out, result)
}

// parsePushArgs interprets the arguments of `g push`.
//
// --force is the only flag, and it is not a shorthand for "push harder": the
// workflow turns it into --force-with-lease, and only after this has asked.
func parsePushArgs(args []string) (bool, error) {
	var force bool

	for _, arg := range args {
		switch arg {
		case "--force":
			force = true
		default:
			return false, usageErrorf("push takes no arguments other than --force, got %q", arg)
		}
	}

	return force, nil
}

// confirmForce asks before replacing a remote branch, and reports the branch
// that has nothing to replace as the error it is.
//
// The question is asked every time this flag is used, deliberately.
// --force-with-lease narrows the window for clobbering somebody else's commits
// but does not close it, and there is no undo for history that has already been
// replaced. The confirmDestructive setting is not consulted here: it governs
// operations that discard local work, and this is not one of those.
func confirmForce(ctx context.Context, env *Env) error {
	status, err := env.Service.Status(ctx, env.Dir)
	if err != nil {
		return err
	}

	// Asking about a force push that cannot happen would be nonsense. A
	// detached HEAD is left to the workflow, which reports it properly.
	if !status.Detached && status.Upstream == nil {
		return &workflow.NoUpstreamError{Branch: status.Branch}
	}

	confirmed, err := env.Confirm(ctx, "Force-push with --force-with-lease, replacing the remote branch if it has moved?")
	if err != nil {
		return err
	}
	if !confirmed {
		return errNotPushed
	}

	return nil
}
