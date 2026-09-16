package git

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotARepository reports that the inspected directory is not inside a Git
// work tree.
var ErrNotARepository = errors.New("not a git repository")

// CommandError reports a git invocation that finished with a non-zero status.
type CommandError struct {
	// Args are the arguments that were passed after the git executable.
	Args []string
	// ExitCode is the status git exited with.
	ExitCode int
	// Stderr is the diagnostic output git produced.
	Stderr string
}

func (e *CommandError) Error() string {
	detail := strings.TrimSpace(e.Stderr)
	if detail == "" {
		detail = "no error output"
	}
	return fmt.Sprintf("git %s: exit status %d: %s", strings.Join(e.Args, " "), e.ExitCode, detail)
}
