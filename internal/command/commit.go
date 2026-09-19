package command

import (
	"context"
	"strings"

	"github.com/roshandroids/g/internal/commit"
	"github.com/roshandroids/g/internal/output"
	"github.com/roshandroids/g/internal/workflow"
)

// runCommit implements `g commit <type> <message>`.
func runCommit(ctx context.Context, env *Env, args []string) error {
	req, err := commitRequest(args)
	if err != nil {
		return err
	}

	result, err := env.Service.Commit(ctx, env.Dir, req)
	if err != nil {
		return err
	}

	return output.RenderCommit(env.Out, result)
}

// commitRequest interprets the arguments of `g commit`.
//
// The type is checked here as well as in the workflow so that a bad type is
// reported as a usage error, which is what it is, rather than as a runtime
// failure.
func commitRequest(args []string) (workflow.CommitRequest, error) {
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return workflow.CommitRequest{}, usageErrorf("unexpected flag %q", args[0])
	}
	if len(args) < 2 {
		return workflow.CommitRequest{}, usageErrorf(
			"commit needs a type and a message, for example `g commit fix \"resolve applicant history issue\"`")
	}

	commitType, err := commit.ParseType(args[0])
	if err != nil {
		return workflow.CommitRequest{}, usageErrorf("%s", err)
	}

	message := strings.Join(args[1:], " ")
	if err := commit.ValidateSubject(message); err != nil {
		return workflow.CommitRequest{}, usageErrorf("%s", err)
	}

	return workflow.CommitRequest{Type: string(commitType), Message: message}, nil
}
