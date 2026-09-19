package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/roshandroids/g/internal/workflow"
)

// RenderNewBranch writes the outcome of creating a branch.
//
// The two notes are conditions the user should know about but which are not
// failures: the new branch carries uncommitted changes with it, and a branch of
// the same name already on the remote will have to be dealt with at push time.
func RenderNewBranch(w io.Writer, result workflow.NewBranchResult) error {
	var b strings.Builder

	fmt.Fprintf(&b, "Created branch %s from %s\n", result.Branch, result.Base)
	if result.Dirty {
		b.WriteString("note: uncommitted changes moved with you to the new branch\n")
	}
	if result.RemoteExists {
		fmt.Fprintf(&b, "warning: %s already has a branch called %s\n", result.Remote, result.Branch)
	}

	return write(w, b.String())
}

// RenderSwitch writes the outcome of switching branches.
func RenderSwitch(w io.Writer, result workflow.SwitchResult) error {
	var b strings.Builder

	switch {
	case result.AlreadyOn:
		fmt.Fprintf(&b, "Already on branch %s\n", result.Branch)
	case result.Previous == "":
		fmt.Fprintf(&b, "Switched to branch %s\n", result.Branch)
	default:
		fmt.Fprintf(&b, "Switched to branch %s (from %s)\n", result.Branch, result.Previous)
	}

	if result.Dirty && !result.AlreadyOn {
		b.WriteString("note: uncommitted changes moved with you\n")
	}

	return write(w, b.String())
}
