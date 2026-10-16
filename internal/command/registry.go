package command

import (
	"context"
	"fmt"

	"github.com/roshandroids/g/internal/output"
)

// Commands returns the command surface in help order.
//
// Every command listed here runs today. Invoking an unknown name fails with a
// usage error rather than pretending to work.
func Commands() []Command {
	return []Command{
		statusCommand,
		helpCommand(),
		newCommand,
		switchCommand,
		commitCommand,
		pushCommand,
		syncCommand,
		continueCommand,
		abortCommand,
		undoCommand,
		cleanCommand,
	}
}

// Find returns the command with the given name.
func Find(name string) (Command, bool) {
	for _, command := range Commands() {
		if command.Name == name {
			return command, true
		}
	}
	return Command{}, false
}

var statusCommand = Command{
	Name:    "status",
	Usage:   "g status",
	Summary: "Show a concise summary of the repository state",
	Run:     runStatus,
}

var newCommand = Command{
	Name:    "new",
	Usage:   "g new <ticket> [description]",
	Summary: "Create a branch for a new piece of work",
	Run:     runNew,
}

var switchCommand = Command{
	Name:    "switch",
	Usage:   "g switch <branch>",
	Summary: "Switch to an existing branch",
	Run:     runSwitch,
}

var commitCommand = Command{
	Name:    "commit",
	Usage:   "g commit <type> <message>",
	Summary: "Record the staged changes as a commit",
	Run:     runCommit,
}

var pushCommand = Command{
	Name:    "push",
	Usage:   "g push [--force]",
	Summary: "Push the current branch to its upstream",
	Run:     runPush,
}

var syncCommand = Command{
	Name:    "sync",
	Usage:   "g sync",
	Summary: "Update the current branch from its base branch",
	Run:     runSync,
}

var continueCommand = Command{
	Name:    "continue",
	Usage:   "g continue",
	Summary: "Continue a paused rebase, merge, or cherry-pick",
	Run:     runContinue,
}

var abortCommand = Command{
	Name:    "abort",
	Usage:   "g abort",
	Summary: "Abort a paused rebase, merge, or cherry-pick",
	Run:     runAbort,
}

var undoCommand = Command{
	Name:    "undo",
	Usage:   "g undo [count]",
	Summary: "Undo recent commits while keeping their changes staged",
	Run:     runUndo,
}

var cleanCommand = Command{
	Name:    "clean",
	Usage:   "g clean [--apply]",
	Summary: "Preview or delete stale local branches",
	Run:     runClean,
}

// helpName is the command word for the help command.
const helpName = "help"

// helpCommand returns the help command.
//
// It is a constructor rather than a package variable because its handler
// enumerates the command surface, which would otherwise be an initialization
// cycle.
func helpCommand() Command {
	return Command{
		Name:    helpName,
		Usage:   "g help",
		Summary: "Show the full command reference",
		Run:     runHelp,
	}
}

// runHelp renders the full command reference.
func runHelp(_ context.Context, env *Env, _ []string) error {
	return output.RenderHelp(env.Out, summaries(Commands()))
}

// runStatus renders the state of the repository containing env.Dir.
func runStatus(ctx context.Context, env *Env, args []string) error {
	if len(args) > 0 {
		return usageErrorf("status takes no arguments")
	}

	status, err := env.Service.Status(ctx, env.Dir)
	if err != nil {
		return err
	}
	return output.RenderStatus(env.Out, status)
}

// summaries projects the command surface into its presentation form.
func summaries(commands []Command) []output.CommandSummary {
	result := make([]output.CommandSummary, 0, len(commands))
	for _, command := range commands {
		result = append(result, output.CommandSummary{
			Usage:     command.Usage,
			Summary:   command.Summary,
			Available: command.Available(),
		})
	}
	return result
}

// usageErrorf builds an error that Run reports as a usage failure.
func usageErrorf(format string, args ...any) error {
	return &UsageError{Message: fmt.Sprintf(format, args...)}
}

// UsageError reports invalid arguments for a command.
type UsageError struct {
	// Message describes what was wrong with the arguments.
	Message string
}

func (e *UsageError) Error() string { return e.Message }
