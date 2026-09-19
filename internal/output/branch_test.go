package output

import (
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/workflow"
)

func TestRenderNewBranch(t *testing.T) {
	var b strings.Builder

	err := RenderNewBranch(&b, workflow.NewBranchResult{
		Branch: "HCM-37538-applicant-id-update",
		Base:   "main",
	})
	if err != nil {
		t.Fatalf("RenderNewBranch() error = %v, want nil", err)
	}

	want := "Created branch HCM-37538-applicant-id-update from main\n"
	if got := b.String(); got != want {
		t.Errorf("RenderNewBranch() = %q, want %q", got, want)
	}
}

func TestRenderNewBranchWithNotes(t *testing.T) {
	var b strings.Builder

	err := RenderNewBranch(&b, workflow.NewBranchResult{
		Branch:       "HCM-1-work",
		Base:         "master",
		Dirty:        true,
		RemoteExists: true,
		Remote:       "origin",
	})
	if err != nil {
		t.Fatalf("RenderNewBranch() error = %v, want nil", err)
	}

	got := b.String()
	for _, want := range []string{
		"Created branch HCM-1-work from master\n",
		"uncommitted changes moved with you",
		"origin already has a branch called HCM-1-work",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderNewBranch() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRenderNewBranchOmitsAbsentNotes(t *testing.T) {
	var b strings.Builder

	if err := RenderNewBranch(&b, workflow.NewBranchResult{Branch: "HCM-1", Base: "main"}); err != nil {
		t.Fatalf("RenderNewBranch() error = %v, want nil", err)
	}

	got := b.String()
	for _, unwanted := range []string{"note:", "warning:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("RenderNewBranch() = %q, want no %q line", got, unwanted)
		}
	}
}

func TestRenderNewBranchReportsWriteFailure(t *testing.T) {
	if err := RenderNewBranch(failingWriter{}, workflow.NewBranchResult{Branch: "HCM-1"}); err == nil {
		t.Fatal("RenderNewBranch() error = nil, want the write failure")
	}
}

func TestRenderSwitch(t *testing.T) {
	tests := []struct {
		name   string
		result workflow.SwitchResult
		want   string
	}{
		{
			name:   "from another branch",
			result: workflow.SwitchResult{Branch: "main", Previous: "feature/x"},
			want:   "Switched to branch main (from feature/x)\n",
		},
		{
			name:   "with no previous branch",
			result: workflow.SwitchResult{Branch: "main"},
			want:   "Switched to branch main\n",
		},
		{
			name:   "already on the branch",
			result: workflow.SwitchResult{Branch: "main", AlreadyOn: true},
			want:   "Already on branch main\n",
		},
		{
			name:   "dirty tree",
			result: workflow.SwitchResult{Branch: "main", Previous: "dev", Dirty: true},
			want:   "Switched to branch main (from dev)\nnote: uncommitted changes moved with you\n",
		},
		{
			name:   "already on a dirty branch keeps quiet",
			result: workflow.SwitchResult{Branch: "main", AlreadyOn: true, Dirty: true},
			want:   "Already on branch main\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var b strings.Builder
			if err := RenderSwitch(&b, test.result); err != nil {
				t.Fatalf("RenderSwitch() error = %v, want nil", err)
			}
			if got := b.String(); got != test.want {
				t.Errorf("RenderSwitch() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRenderSwitchReportsWriteFailure(t *testing.T) {
	if err := RenderSwitch(failingWriter{}, workflow.SwitchResult{Branch: "main"}); err == nil {
		t.Fatal("RenderSwitch() error = nil, want the write failure")
	}
}
