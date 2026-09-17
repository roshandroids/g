package output

import (
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/commit"
	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/workflow"
)

func TestRenderCommit(t *testing.T) {
	var b strings.Builder

	err := RenderCommit(&b, workflow.CommitResult{
		Message: "fix: resolve applicant history issue",
		Type:    commit.Fix,
		Staged: []workflow.Change{
			{Path: "lib/foo.dart", Kind: git.Modified},
			{Path: "lib/baz.dart", OriginalPath: "lib/bar.dart", Kind: git.Renamed},
		},
	})
	if err != nil {
		t.Fatalf("RenderCommit() error = %v, want nil", err)
	}

	want := "Committed:\n" +
		"  modified     lib/foo.dart\n" +
		"  renamed      lib/bar.dart -> lib/baz.dart\n" +
		"\n" +
		"fix: resolve applicant history issue\n"

	if got := b.String(); got != want {
		t.Errorf("RenderCommit() =\n%q\nwant\n%q", got, want)
	}
}

// A commit made on a detached HEAD is legal but easy to lose, so it is
// impossible to see the output of this command without being told.
func TestRenderCommitWarnsAboutADetachedHead(t *testing.T) {
	var b strings.Builder

	err := RenderCommit(&b, workflow.CommitResult{
		Message:  "fix: hotfix",
		Staged:   []workflow.Change{{Path: "a.txt", Kind: git.Modified}},
		Detached: true,
	})
	if err != nil {
		t.Fatalf("RenderCommit() error = %v, want nil", err)
	}

	if got := b.String(); !strings.Contains(got, "HEAD is not on a branch") {
		t.Errorf("RenderCommit() = %q, want it to warn about the detached HEAD", got)
	}
}

func TestRenderCommitOmitsTheWarningOnABranch(t *testing.T) {
	var b strings.Builder

	if err := RenderCommit(&b, workflow.CommitResult{Message: "fix: x", Staged: []workflow.Change{{Path: "a.txt"}}}); err != nil {
		t.Fatalf("RenderCommit() error = %v, want nil", err)
	}

	if got := b.String(); strings.Contains(got, "warning:") {
		t.Errorf("RenderCommit() = %q, want no warning", got)
	}
}

func TestRenderCommitReportsWriteFailure(t *testing.T) {
	if err := RenderCommit(failingWriter{}, workflow.CommitResult{Message: "fix: x"}); err == nil {
		t.Fatal("RenderCommit() error = nil, want the write failure")
	}
}
