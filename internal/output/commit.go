package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/roshandroids/g/internal/workflow"
)

// RenderCommit writes the outcome of recording a commit.
//
// What was committed is listed before the message, because the message is what
// the user typed and the file list is what they might have got wrong.
func RenderCommit(w io.Writer, result workflow.CommitResult) error {
	var b strings.Builder

	b.WriteString("Committed:\n")
	writeChangeList(&b, result.Staged)
	fmt.Fprintf(&b, "\n%s\n", result.Message)

	if result.Detached {
		b.WriteString("\nwarning: HEAD is not on a branch, so this commit is reachable only from HEAD\n")
	}

	return write(w, b.String())
}
