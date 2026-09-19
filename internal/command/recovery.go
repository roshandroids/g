package command

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/roshandroids/g/internal/output"
	"github.com/roshandroids/g/internal/workflow"
)

var (
	errNotAborted = errors.New("nothing was aborted")
	errNotCleaned = errors.New("nothing was deleted")
)

func runContinue(ctx context.Context, env *Env, args []string) error {
	if len(args) > 0 {
		return usageErrorf("continue takes no arguments")
	}

	status, err := env.Service.Status(ctx, env.Dir)
	if err != nil {
		return err
	}
	if status.Operation == "" {
		return fmt.Errorf("%w", workflow.ErrNoOperation)
	}

	fmt.Fprintf(env.Out, "%s in progress.\n", output.OperationLabel(status.Operation))
	if status.Branch != "" {
		fmt.Fprintf(env.Out, "Branch: %s\n", status.Branch)
	} else if status.Detached {
		fmt.Fprintf(env.Out, "HEAD: detached at %s\n", status.Head)
	}
	if len(status.Conflicted) > 0 {
		fmt.Fprintln(env.Out, "Conflicted:")
		for _, path := range status.Conflicted {
			fmt.Fprintf(env.Out, "  %s\n", path)
		}
	}
	fmt.Fprintln(env.Out)

	result, err := env.Service.Continue(ctx, env.Dir)
	if err != nil {
		return err
	}
	return output.RenderContinue(env.Out, result)
}

func runAbort(ctx context.Context, env *Env, args []string) error {
	if len(args) > 0 {
		return usageErrorf("abort takes no arguments")
	}

	op, err := env.Service.CurrentOperation(ctx, env.Dir)
	if err != nil {
		return err
	}

	question := fmt.Sprintf(
		"A %s is currently in progress.\nAborting will discard the current %s state.\n\nContinue?",
		op, op,
	)
	confirmed, err := env.Confirm(ctx, question)
	if err != nil {
		return err
	}
	if !confirmed {
		return errNotAborted
	}

	result, err := env.Service.Abort(ctx, env.Dir)
	if err != nil {
		return err
	}
	return output.RenderAbort(env.Out, result)
}

func runUndo(ctx context.Context, env *Env, args []string) error {
	count, err := parseUndoArgs(args)
	if err != nil {
		return err
	}

	result, err := env.Service.Undo(ctx, env.Dir, count)
	if err != nil {
		return err
	}
	return output.RenderUndo(env.Out, result)
}

func parseUndoArgs(args []string) (int, error) {
	switch len(args) {
	case 0:
		return 1, nil
	case 1:
		count, err := strconv.Atoi(args[0])
		if err != nil || count < 1 {
			return 0, usageErrorf("undo count must be a positive integer, got %q", args[0])
		}
		return count, nil
	default:
		return 0, usageErrorf("undo takes at most one count argument")
	}
}

func runClean(ctx context.Context, env *Env, args []string) error {
	apply, err := parseCleanArgs(args)
	if err != nil {
		return err
	}

	if !apply {
		result, err := env.Service.Clean(ctx, env.Dir, workflow.CleanRequest{})
		if err != nil {
			return err
		}
		return output.RenderClean(env.Out, result)
	}

	preview, err := env.Service.Clean(ctx, env.Dir, workflow.CleanRequest{})
	if err != nil {
		return err
	}
	if len(preview.Candidates) == 0 {
		return output.RenderClean(env.Out, preview)
	}

	fmt.Fprintln(env.Out, "Branches that will be deleted:")
	for _, branch := range preview.Candidates {
		if !branch.Safe {
			continue
		}
		fmt.Fprintf(env.Out, "  %s\n", branch.Name)
	}
	fmt.Fprintln(env.Out)

	confirmed, err := env.Confirm(ctx, "Delete these local branches?")
	if err != nil {
		return err
	}
	if !confirmed {
		return errNotCleaned
	}

	result, err := env.Service.Clean(ctx, env.Dir, workflow.CleanRequest{Apply: true})
	if err != nil {
		return err
	}
	return output.RenderClean(env.Out, result)
}

func parseCleanArgs(args []string) (apply bool, err error) {
	for _, arg := range args {
		switch arg {
		case "--apply":
			apply = true
		default:
			return false, usageErrorf("clean takes no arguments other than --apply, got %q", arg)
		}
	}
	return apply, nil
}
