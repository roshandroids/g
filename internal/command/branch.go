package command

import (
	"context"
	"strings"

	"github.com/roshandroids/g/internal/output"
	"github.com/roshandroids/g/internal/workflow"
)

// runNew implements `g new`, creating a branch for a piece of work.
func runNew(ctx context.Context, env *Env, args []string) error {
	req, err := newBranchRequest(args)
	if err != nil {
		return err
	}

	result, err := env.Service.NewBranch(ctx, env.Dir, req)
	if err != nil {
		return err
	}

	return output.RenderNewBranch(env.Out, result)
}

// newBranchRequest interprets the arguments of `g new`.
//
// The documented usage quotes the description, but joining whatever follows the
// issue key means an unquoted description behaves the same way rather than
// failing on an unexpected third argument.
func newBranchRequest(args []string) (workflow.NewBranchRequest, error) {
	if len(args) == 0 {
		return workflow.NewBranchRequest{}, usageErrorf(
			"new needs an issue key, for example `g new HCM-37538 \"applicant id update\"`")
	}
	if strings.HasPrefix(args[0], "-") {
		return workflow.NewBranchRequest{}, usageErrorf("unexpected flag %q", args[0])
	}

	return workflow.NewBranchRequest{
		Ticket:      args[0],
		Description: strings.Join(args[1:], " "),
	}, nil
}

// runSwitch implements `g switch`, checking out an existing branch.
func runSwitch(ctx context.Context, env *Env, args []string) error {
	if len(args) == 0 {
		return usageErrorf("switch needs the name of a branch")
	}
	if len(args) > 1 {
		return usageErrorf("switch takes one branch name, got %d arguments", len(args))
	}
	if strings.HasPrefix(args[0], "-") {
		return usageErrorf("unexpected flag %q", args[0])
	}

	result, err := env.Service.Switch(ctx, env.Dir, workflow.SwitchRequest{Branch: args[0]})
	if err != nil {
		return err
	}

	return output.RenderSwitch(env.Out, result)
}
