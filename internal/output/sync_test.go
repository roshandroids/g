package output

import (
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/workflow"
)

func TestRenderSync(t *testing.T) {
	var b strings.Builder
	err := RenderSync(&b, workflow.SyncResult{
		Branch:           "feature",
		Base:             "main",
		Ahead:            3,
		Behind:           0,
		Fetched:          true,
		Remote:           "origin",
		BaseUpdated:      true,
		Rebased:          true,
		HistoryRewritten: true,
		Preserved:        true,
		Restored:         true,
	})
	if err != nil {
		t.Fatalf("RenderSync() error = %v, want nil", err)
	}

	got := b.String()
	for _, want := range []string{
		"Syncing feature",
		"Base: main",
		"Fetching origin...",
		"Updating main...",
		"Rebasing feature onto main...",
		"Local changes preserved and restored.",
		"Sync complete.",
		"Ahead:  3",
		"force-with-lease",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderSync() missing %q:\n%s", want, got)
		}
	}
}
