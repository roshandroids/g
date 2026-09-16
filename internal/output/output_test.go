package output

import (
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/workflow"
)

func TestRenderStatusCleanRepository(t *testing.T) {
	status := workflow.Status{
		Name:     "nexus_applicant",
		Branch:   "HCM-37538-applicant-id-update",
		Upstream: &workflow.Upstream{Name: "origin/HCM-37538-applicant-id-update"},
	}

	want := "Repository: nexus_applicant\n" +
		"Branch:     HCM-37538-applicant-id-update\n" +
		"Upstream:   origin/HCM-37538-applicant-id-update\n" +
		"\n" +
		"Working tree: clean\n" +
		"\n" +
		"Ahead:  0\n" +
		"Behind: 0\n"

	if got := renderStatus(t, status); got != want {
		t.Errorf("RenderStatus() =\n%q\nwant\n%q", got, want)
	}
}

// A clean tree that is ahead of its upstream is the state `g push` acts on, so
// the divergence must still be visible.
func TestRenderStatusCleanButAhead(t *testing.T) {
	status := workflow.Status{
		Name:     "app",
		Branch:   "main",
		Upstream: &workflow.Upstream{Name: "origin/main", Ahead: 2},
	}

	want := "Repository: app\n" +
		"Branch:     main\n" +
		"Upstream:   origin/main\n" +
		"\n" +
		"Working tree: clean\n" +
		"\n" +
		"Ahead:  2\n" +
		"Behind: 0\n"

	if got := renderStatus(t, status); got != want {
		t.Errorf("RenderStatus() =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderStatusWithoutUpstream(t *testing.T) {
	status := workflow.Status{Name: "demo", Branch: "main"}

	want := "Repository: demo\n" +
		"Branch:     main\n" +
		"Upstream:   none\n" +
		"\n" +
		"Working tree: clean\n"

	if got := renderStatus(t, status); got != want {
		t.Errorf("RenderStatus() =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderStatusDetachedHead(t *testing.T) {
	status := workflow.Status{Name: "demo", Detached: true, Head: "a1b2c3d"}

	want := "Repository: demo\n" +
		"Branch:     detached at a1b2c3d\n" +
		"Upstream:   none\n" +
		"\n" +
		"Working tree: clean\n"

	if got := renderStatus(t, status); got != want {
		t.Errorf("RenderStatus() =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderStatusDirtyRepository(t *testing.T) {
	status := workflow.Status{
		Name:     "demo",
		Branch:   "main",
		Upstream: &workflow.Upstream{Name: "origin/main", Ahead: 2, Behind: 1},
		Staged: []workflow.Change{
			{Path: "lib/foo.dart", Kind: git.Modified},
			{Path: "lib/baz.dart", OriginalPath: "lib/bar.dart", Kind: git.Renamed},
		},
		Unstaged:   []workflow.Change{{Path: "lib/foo.dart", Kind: git.Modified}},
		Untracked:  []string{"test/foo_test.dart"},
		Conflicted: []string{"lib/conflict.dart"},
	}

	want := "Repository: demo\n" +
		"Branch:     main\n" +
		"Upstream:   origin/main\n" +
		"\n" +
		"Staged:\n" +
		"  modified     lib/foo.dart\n" +
		"  renamed      lib/bar.dart -> lib/baz.dart\n" +
		"\n" +
		"Unstaged:\n" +
		"  modified     lib/foo.dart\n" +
		"\n" +
		"Untracked:\n" +
		"  test/foo_test.dart\n" +
		"\n" +
		"Conflicted:\n" +
		"  lib/conflict.dart\n" +
		"\n" +
		"Ahead:  2\n" +
		"Behind: 1\n"

	if got := renderStatus(t, status); got != want {
		t.Errorf("RenderStatus() =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderStatusWithoutUpstreamOmitsAheadAndBehind(t *testing.T) {
	status := workflow.Status{
		Name:      "demo",
		Branch:    "main",
		Untracked: []string{"notes.txt"},
	}

	got := renderStatus(t, status)
	for _, unwanted := range []string{"Ahead:", "Behind:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("RenderStatus() = %q, want no %q line without an upstream", got, unwanted)
		}
	}
}

func TestRenderStatusReportsWriteFailure(t *testing.T) {
	err := RenderStatus(failingWriter{}, workflow.Status{Name: "demo"})
	if err == nil {
		t.Fatal("RenderStatus() error = nil, want the write failure")
	}
}

func TestRenderShortHelp(t *testing.T) {
	commands := []CommandSummary{
		{Usage: "g status", Summary: "Show a concise summary of the repository state", Available: true},
		{Usage: "g help", Summary: "Show the full command reference", Available: true},
		{Usage: "g sync", Summary: "Update the current branch", Available: false},
	}

	want := "g - personal Git and GitHub workflow assistant\n" +
		"\n" +
		"Usage:\n" +
		"  g <command> [arguments]\n" +
		"\n" +
		"Commands: status, help\n" +
		"Planned:  sync\n" +
		"\n" +
		"Run `g --help` for details.\n"

	var b strings.Builder
	if err := RenderShortHelp(&b, commands); err != nil {
		t.Fatalf("RenderShortHelp() error = %v, want nil", err)
	}
	if got := b.String(); got != want {
		t.Errorf("RenderShortHelp() =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderHelp(t *testing.T) {
	commands := []CommandSummary{
		{Usage: "g status", Summary: "Show a concise summary of the repository state", Available: true},
		{Usage: "g sync now", Summary: "Update the current branch", Available: false},
	}

	want := "g - personal Git and GitHub workflow assistant\n" +
		"\n" +
		"Usage:\n" +
		"  g <command> [arguments]\n" +
		"  g --help\n" +
		"\n" +
		"Commands:\n" +
		"  g status    Show a concise summary of the repository state\n" +
		"\n" +
		"Planned (not implemented yet):\n" +
		"  g sync now  Update the current branch\n" +
		"\n" +
		"Exit codes:\n" +
		"  0 success\n" +
		"  1 error\n" +
		"  2 usage error\n"

	var b strings.Builder
	if err := RenderHelp(&b, commands); err != nil {
		t.Fatalf("RenderHelp() error = %v, want nil", err)
	}
	if got := b.String(); got != want {
		t.Errorf("RenderHelp() =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderHelpReportsWriteFailure(t *testing.T) {
	if err := RenderHelp(failingWriter{}, nil); err == nil {
		t.Fatal("RenderHelp() error = nil, want the write failure")
	}
}

func TestRenderShortHelpReportsWriteFailure(t *testing.T) {
	if err := RenderShortHelp(failingWriter{}, nil); err == nil {
		t.Fatal("RenderShortHelp() error = nil, want the write failure")
	}
}

// renderStatus renders a status and returns the text, failing the test if
// rendering itself fails.
func renderStatus(t *testing.T, status workflow.Status) string {
	t.Helper()

	var b strings.Builder
	if err := RenderStatus(&b, status); err != nil {
		t.Fatalf("RenderStatus() error = %v, want nil", err)
	}
	return b.String()
}

// failingWriter reports every write as a failure.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
