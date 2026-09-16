package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/roshandroids/g/internal/output"
)

// Run interprets args (the arguments after the program name) and returns the
// exit code the process should exit with.
func Run(ctx context.Context, env *Env, args []string) int {
	commands := Commands()

	if len(args) == 0 {
		return render(env, func(w io.Writer) error {
			return output.RenderShortHelp(w, summaries(commands))
		})
	}

	name := args[0]
	if name == "--help" || name == "-h" {
		name = helpName
	}

	command, found := Find(name)
	if !found {
		fmt.Fprintf(env.Err, "g: unknown %s %q\n\n", subject(name), name)
		writeCommandHelp(env.Err, commands)
		return ExitUsage
	}

	if !command.Available() {
		fmt.Fprintf(env.Err, "g: %s is not implemented yet (planned for %s)\n", command.Name, command.Roadmap)
		return ExitError
	}

	if err := command.Run(ctx, env, args[1:]); err != nil {
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			fmt.Fprintf(env.Err, "g: %s\nusage: %s\n", usageErr, command.Usage)
			return ExitUsage
		}
		fmt.Fprintf(env.Err, "g: %v\n", err)
		return ExitError
	}

	return ExitOK
}

// render writes help to the output stream, reporting a broken stream as a
// failure rather than silently producing nothing.
func render(env *Env, help func(io.Writer) error) int {
	if err := help(env.Out); err != nil {
		fmt.Fprintf(env.Err, "g: %v\n", err)
		return ExitError
	}
	return ExitOK
}

// writeCommandHelp prints the command list after a usage failure. Error output
// is best effort, so a write failure here is not reported again.
func writeCommandHelp(w io.Writer, commands []Command) {
	_ = output.RenderShortHelp(w, summaries(commands))
}

// subject names what could not be resolved, so flags are not reported as
// commands.
func subject(name string) string {
	if strings.HasPrefix(name, "-") {
		return "flag"
	}
	return "command"
}
