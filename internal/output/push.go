package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/roshandroids/g/internal/workflow"
)

// RenderPush writes the outcome of a push.
func RenderPush(w io.Writer, result workflow.PushResult) error {
	var b strings.Builder

	switch {
	case result.Forced:
		fmt.Fprintf(&b, "Force-pushed %s to %s (--force-with-lease)\n", result.Branch, result.Upstream)
	case result.SetUpstream:
		fmt.Fprintf(&b, "Pushed %s and set upstream to %s\n", result.Branch, result.Upstream)
	default:
		fmt.Fprintf(&b, "Pushed %s to %s\n", result.Branch, result.Upstream)
	}

	return write(w, b.String())
}
