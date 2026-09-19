package output

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/workflow"
)

// RenderContinue writes the outcome of continuing a paused operation.
func RenderContinue(w io.Writer, result workflow.ContinueResult) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s continued successfully.\n", titleOperation(result.Operation))
	return write(w, b.String())
}

// RenderAbort writes the outcome of aborting a paused operation.
func RenderAbort(w io.Writer, result workflow.AbortResult) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s aborted.\n", titleOperation(result.Operation))
	return write(w, b.String())
}

// RenderUndo writes the outcome of undoing recent commits.
func RenderUndo(w io.Writer, result workflow.UndoResult) error {
	var b strings.Builder

	if result.Count == 1 {
		b.WriteString("Undoing last commit:\n")
	} else {
		fmt.Fprintf(&b, "Undoing last %d commits:\n", result.Count)
	}
	for _, commit := range result.Commits {
		fmt.Fprintf(&b, "  %s %s\n", commit.Hash, commit.Subject)
	}

	if result.Count == 1 {
		b.WriteString("\nCommit removed.\n")
	} else {
		b.WriteString("\nCommits removed.\n")
	}
	b.WriteString("Changes preserved and staged.\n")

	if result.HasUpstream {
		b.WriteString("\nNote: local history was rewritten.\n")
		b.WriteString("The remote was not changed.\n")
		if result.Upstream != "" {
			fmt.Fprintf(&b, "Upstream %s still points at the previous history.\n", result.Upstream)
		}
		b.WriteString("A force-with-lease push may be required later.\n")
	}

	return write(w, b.String())
}

// RenderClean writes the stale-branch preview or deletion result.
func RenderClean(w io.Writer, result workflow.CleanResult) error {
	var b strings.Builder

	if len(result.Candidates) == 0 {
		b.WriteString("No stale local branches found.\n")
		return write(w, b.String())
	}

	b.WriteString("Stale local branches:\n\n")
	for _, branch := range result.Candidates {
		fmt.Fprintf(&b, "  %s (%s)\n", branch.Name, branch.Reason)
	}

	if !result.Applied {
		b.WriteString("\nPreview only. No branches deleted.\n")
		b.WriteString("Run `g clean --apply` to delete them after confirmation.\n")
		return write(w, b.String())
	}

	if len(result.Deleted) == 0 {
		b.WriteString("\nNo branches deleted.\n")
		return write(w, b.String())
	}

	b.WriteString("\nDeleted:\n")
	for _, name := range result.Deleted {
		fmt.Fprintf(&b, "  %s\n", name)
	}
	return write(w, b.String())
}

// OperationLabel returns the human-facing name for a paused operation.
func OperationLabel(op git.Operation) string {
	return titleOperation(op)
}

func titleOperation(op git.Operation) string {
	name := string(op)
	if name == "" {
		return "Operation"
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
