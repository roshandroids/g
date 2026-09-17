package output

import (
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/workflow"
)

func TestRenderPush(t *testing.T) {
	tests := []struct {
		name   string
		result workflow.PushResult
		want   string
	}{
		{
			name:   "an ordinary push",
			result: workflow.PushResult{Branch: "HCM-1-work", Upstream: "origin/HCM-1-work"},
			want:   "Pushed HCM-1-work to origin/HCM-1-work\n",
		},
		{
			name:   "a push that created the upstream",
			result: workflow.PushResult{Branch: "HCM-1-work", Upstream: "origin/HCM-1-work", SetUpstream: true},
			want:   "Pushed HCM-1-work and set upstream to origin/HCM-1-work\n",
		},
		{
			name:   "a forced push names the flag it used",
			result: workflow.PushResult{Branch: "HCM-1-work", Upstream: "origin/HCM-1-work", Forced: true},
			want:   "Force-pushed HCM-1-work to origin/HCM-1-work (--force-with-lease)\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var b strings.Builder
			if err := RenderPush(&b, test.result); err != nil {
				t.Fatalf("RenderPush() error = %v, want nil", err)
			}
			if got := b.String(); got != test.want {
				t.Errorf("RenderPush() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRenderPushReportsWriteFailure(t *testing.T) {
	if err := RenderPush(failingWriter{}, workflow.PushResult{Branch: "work"}); err == nil {
		t.Fatal("RenderPush() error = nil, want the write failure")
	}
}
