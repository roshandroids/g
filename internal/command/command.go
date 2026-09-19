// Package command defines the g command surface and dispatches invocations to
// it.
//
// Commands translate arguments into workflow calls and hand the results to the
// output package; they hold no Git logic themselves, which keeps the same
// workflows reachable from a future GUI.
package command

import (
	"context"
	"io"

	"github.com/roshandroids/g/internal/config"
	"github.com/roshandroids/g/internal/github"
	"github.com/roshandroids/g/internal/prompt"
	"github.com/roshandroids/g/internal/workflow"
)

// Exit codes returned by Run.
const (
	// ExitOK reports success.
	ExitOK = 0
	// ExitError reports a runtime failure, including running outside a Git
	// repository.
	ExitError = 1
	// ExitUsage reports invalid command line input.
	ExitUsage = 2
)

// Service is the workflow surface the command layer drives.
type Service interface {
	// Status summarises the repository containing dir.
	Status(ctx context.Context, dir string) (workflow.Status, error)
	// NewBranch creates a branch for a piece of work and switches to it.
	NewBranch(ctx context.Context, dir string, req workflow.NewBranchRequest) (workflow.NewBranchResult, error)
	// Switch checks out an existing local branch.
	Switch(ctx context.Context, dir string, req workflow.SwitchRequest) (workflow.SwitchResult, error)
	// Commit records the staged changes.
	Commit(ctx context.Context, dir string, req workflow.CommitRequest) (workflow.CommitResult, error)
	// Push sends the current branch to its remote.
	Push(ctx context.Context, dir string, req workflow.PushRequest) (workflow.PushResult, error)
	// Sync updates the current branch from its base.
	Sync(ctx context.Context, dir string) (workflow.SyncResult, error)
}

// Env carries the dependencies and streams a command needs.
type Env struct {
	// Service performs repository workflows.
	Service Service
	// GitHub is the GitHub boundary; commands check Available before using it.
	GitHub github.Client
	// Prompt asks the user to confirm an action. A nil Prompt refuses every
	// confirmation, so a command that needs one fails closed rather than
	// assuming consent.
	Prompt prompt.Prompter
	// Config is the user configuration.
	Config config.Config
	// Dir is the directory commands operate on.
	Dir string
	// Out receives normal output.
	Out io.Writer
	// Err receives error output.
	Err io.Writer
}

// Confirm asks the user to confirm an action.
func (e *Env) Confirm(ctx context.Context, question string) (bool, error) {
	if e.Prompt == nil {
		return false, nil
	}
	return e.Prompt.Confirm(ctx, question)
}

// Command is one entry of the g command surface.
type Command struct {
	// Name is the command word, for example "status".
	Name string
	// Usage is the complete invocation, starting with "g ".
	Usage string
	// Summary is the one-line description shown in help.
	Summary string
	// Roadmap is the milestone that delivers the command when Run is nil.
	Roadmap string
	// Run executes the command. A nil Run marks a planned command.
	Run func(ctx context.Context, env *Env, args []string) error
}

// Available reports whether the command can be executed.
func (c Command) Available() bool { return c.Run != nil }
