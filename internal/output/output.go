// Package output renders the text g writes to a terminal.
//
// Rendering lives here so that commands only decide what to show, and so a
// future GUI can present the same workflow results through a different
// presenter.
package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/roshandroids/g/internal/workflow"
)

// fieldWidth aligns the labels in the repository summary block.
const fieldWidth = 12

// CommandSummary is the presentation view of one command.
type CommandSummary struct {
	// Usage is the complete invocation, for example
	// "g new <ticket> [description]".
	Usage string
	// Summary is the one-line description.
	Summary string
	// Available is false for commands that are still planned.
	Available bool
}

// RenderStatus writes a concise summary of s.
func RenderStatus(w io.Writer, s workflow.Status) error {
	var b strings.Builder

	field(&b, "Repository:", s.Name)
	field(&b, "Branch:", branchLabel(s))
	if s.Upstream != nil {
		field(&b, "Upstream:", s.Upstream.Name)
	} else {
		field(&b, "Upstream:", "none")
	}

	if s.Clean() {
		b.WriteString("\nWorking tree: clean\n")
	} else {
		writeChanges(&b, "Staged", s.Staged)
		writeChanges(&b, "Unstaged", s.Unstaged)
		writePaths(&b, "Untracked", s.Untracked)
		writePaths(&b, "Conflicted", s.Conflicted)
	}

	// Divergence is reported even for a clean tree, because being ahead of the
	// upstream with nothing left to change is the state `g push` acts on.
	if s.Upstream != nil {
		fmt.Fprintf(&b, "\nAhead:  %d\nBehind: %d\n", s.Upstream.Ahead, s.Upstream.Behind)
	}

	return write(w, b.String())
}

// RenderShortHelp writes the help shown when g runs without arguments.
func RenderShortHelp(w io.Writer, commands []CommandSummary) error {
	available, planned := split(commands)

	var b strings.Builder
	b.WriteString("g - personal Git and GitHub workflow assistant\n\n")
	b.WriteString("Usage:\n  g <command> [arguments]\n\n")
	if len(available) > 0 {
		fmt.Fprintf(&b, "Commands: %s\n", strings.Join(names(available), ", "))
	}
	if len(planned) > 0 {
		fmt.Fprintf(&b, "Planned:  %s\n", strings.Join(names(planned), ", "))
	}
	b.WriteString("\nRun `g --help` for details.\n")

	return write(w, b.String())
}

// RenderHelp writes the full command reference.
func RenderHelp(w io.Writer, commands []CommandSummary) error {
	available, planned := split(commands)

	width := 0
	for _, c := range commands {
		width = max(width, len(c.Usage))
	}

	var b strings.Builder
	b.WriteString("g - personal Git and GitHub workflow assistant\n\n")
	b.WriteString("Usage:\n  g <command> [arguments]\n  g --help\n\n")
	writeCommandList(&b, "Commands:", available, width)
	writeCommandList(&b, "Planned (not implemented yet):", planned, width)
	b.WriteString("Exit codes:\n  0 success\n  1 error\n  2 usage error\n")

	return write(w, b.String())
}

// field writes one "Label:      value" line of the summary block.
func field(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "%-*s%s\n", fieldWidth, label, value)
}

// branchLabel describes where HEAD points.
func branchLabel(s workflow.Status) string {
	switch {
	case s.Detached:
		return "detached at " + s.Head
	case s.Branch == "":
		return "unknown"
	default:
		return s.Branch
	}
}

// writeChanges writes one titled block of changed paths, kept apart from the
// block above it by a blank line.
func writeChanges(b *strings.Builder, title string, changes []workflow.Change) {
	if len(changes) == 0 {
		return
	}

	fmt.Fprintf(b, "\n%s:\n", title)
	for _, change := range changes {
		fmt.Fprintf(b, "  %-12s %s\n", change.Kind, changeLabel(change))
	}
}

// writePaths writes one titled block of plain paths, kept apart from the block
// above it by a blank line.
func writePaths(b *strings.Builder, title string, paths []string) {
	if len(paths) == 0 {
		return
	}

	fmt.Fprintf(b, "\n%s:\n", title)
	for _, path := range paths {
		fmt.Fprintf(b, "  %s\n", path)
	}
}

// changeLabel renders a change, showing both sides of a rename or copy.
func changeLabel(change workflow.Change) string {
	if change.OriginalPath != "" {
		return change.OriginalPath + " -> " + change.Path
	}
	return change.Path
}

// writeCommandList writes one titled column of commands.
func writeCommandList(b *strings.Builder, title string, commands []CommandSummary, width int) {
	if len(commands) == 0 {
		return
	}

	fmt.Fprintf(b, "%s\n", title)
	for _, c := range commands {
		fmt.Fprintf(b, "  %-*s  %s\n", width, c.Usage, c.Summary)
	}
	b.WriteString("\n")
}

// split separates the command surface into what runs today and what is planned.
func split(commands []CommandSummary) (available, planned []CommandSummary) {
	for _, c := range commands {
		if c.Available {
			available = append(available, c)
			continue
		}
		planned = append(planned, c)
	}
	return available, planned
}

// names extracts the command word from each usage line.
func names(commands []CommandSummary) []string {
	result := make([]string, 0, len(commands))
	for _, c := range commands {
		fields := strings.Fields(c.Usage)
		if len(fields) < 2 {
			result = append(result, c.Usage)
			continue
		}
		result = append(result, fields[1])
	}
	return result
}

// write sends the rendered text to the output stream.
func write(w io.Writer, text string) error {
	_, err := io.WriteString(w, text)
	return err
}
