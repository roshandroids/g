package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/roshandroids/g/internal/workflow"
)

// RenderSync writes the outcome of a synchronisation.
func RenderSync(w io.Writer, result workflow.SyncResult) error {
	var b strings.Builder

	fmt.Fprintf(&b, "Syncing %s\n\n", result.Branch)
	fmt.Fprintf(&b, "Base: %s\n", result.Base)

	if result.Fetched {
		fmt.Fprintf(&b, "Fetching %s...\n", result.Remote)
	} else {
		b.WriteString("No remote configured; using local base only.\n")
	}

	switch {
	case result.BaseUpdated:
		fmt.Fprintf(&b, "Updating %s...\n", result.Base)
	case result.Fetched:
		fmt.Fprintf(&b, "%s already up to date with remote.\n", result.Base)
	}

	if result.OnBase {
		b.WriteString("Already on base branch; nothing to rebase.\n")
	} else if result.Rebased {
		fmt.Fprintf(&b, "Rebasing %s onto %s...\n", result.Branch, result.Base)
	} else {
		fmt.Fprintf(&b, "%s already based on %s.\n", result.Branch, result.Base)
	}

	if result.Preserved && result.Restored {
		b.WriteString("Local changes preserved and restored.\n")
	}

	b.WriteString("\nSync complete.\n\n")
	fmt.Fprintf(&b, "Branch: %s\n", result.Branch)
	fmt.Fprintf(&b, "Base:   %s\n", result.Base)
	if !result.OnBase {
		fmt.Fprintf(&b, "Ahead:  %d\n", result.Ahead)
		fmt.Fprintf(&b, "Behind: %d\n", result.Behind)
	}

	if result.HistoryRewritten {
		b.WriteString("\nNote: branch history was rewritten.\n")
		b.WriteString("A force-with-lease push may be required.\n")
		b.WriteString("Review changes before pushing with `g push`.\n")
	}

	return write(w, b.String())
}
