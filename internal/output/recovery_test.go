package output

import (
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/workflow"
)

func TestRenderContinue(t *testing.T) {
	var b strings.Builder
	if err := RenderContinue(&b, workflow.ContinueResult{Operation: git.OperationRebase}); err != nil {
		t.Fatalf("RenderContinue() error = %v", err)
	}
	if got := b.String(); got != "Rebase continued successfully.\n" {
		t.Errorf("RenderContinue() = %q", got)
	}
}

func TestRenderAbort(t *testing.T) {
	var b strings.Builder
	if err := RenderAbort(&b, workflow.AbortResult{Operation: git.OperationMerge}); err != nil {
		t.Fatalf("RenderAbort() error = %v", err)
	}
	if got := b.String(); got != "Merge aborted.\n" {
		t.Errorf("RenderAbort() = %q", got)
	}
}

func TestRenderUndo(t *testing.T) {
	var b strings.Builder
	err := RenderUndo(&b, workflow.UndoResult{
		Count:   1,
		Commits: []git.CommitSummary{{Hash: "abc1234", Subject: "fix: resolve applicant history issue"}},
	})
	if err != nil {
		t.Fatalf("RenderUndo() error = %v", err)
	}
	got := b.String()
	for _, want := range []string{
		"Undoing last commit:",
		"abc1234 fix: resolve applicant history issue",
		"Commit removed.",
		"Changes preserved and staged.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderUndo() missing %q:\n%s", want, got)
		}
	}
}

func TestRenderUndoWarnsAboutUpstream(t *testing.T) {
	var b strings.Builder
	_ = RenderUndo(&b, workflow.UndoResult{
		Count:       1,
		Commits:     []git.CommitSummary{{Hash: "a", Subject: "x"}},
		HasUpstream: true,
		Upstream:    "origin/feature",
	})
	got := b.String()
	if !strings.Contains(got, "The remote was not changed") {
		t.Errorf("RenderUndo() = %q, want remote warning", got)
	}
}

func TestRenderCleanPreview(t *testing.T) {
	var b strings.Builder
	_ = RenderClean(&b, workflow.CleanResult{
		Candidates: []git.StaleBranch{{Name: "HCM-123-old-feature", Reason: git.StaleMerged, Safe: true}},
	})
	got := b.String()
	for _, want := range []string{"Stale local branches:", "HCM-123-old-feature", "Preview only. No branches deleted."} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderClean() missing %q:\n%s", want, got)
		}
	}
}
