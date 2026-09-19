package command

import (
	"context"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/workflow"
)

func TestRunCommit(t *testing.T) {
	service := &fakeService{commitRes: workflow.CommitResult{
		Message: "fix: resolve applicant history issue",
		Staged:  []workflow.Change{{Path: "lib/foo.dart", Kind: git.Modified}},
	}}
	env, out, errOut := newEnv(service)

	code := Run(context.Background(), env, []string{"commit", "fix", "resolve applicant history issue"})
	if code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if errOut.Len() != 0 {
		t.Errorf("Run() error output = %q, want none", errOut.String())
	}

	want := workflow.CommitRequest{Type: "fix", Message: "resolve applicant history issue"}
	if service.commitReq != want {
		t.Errorf("Commit request = %+v, want %+v", service.commitReq, want)
	}
	if want := []string{testDir}; len(service.dirs) != 1 || service.dirs[0] != want[0] {
		t.Errorf("commit ran in %v, want %v", service.dirs, want)
	}

	got := out.String()
	if !strings.Contains(got, "Committed:") {
		t.Errorf("Run() output = %q, want it to list what was committed", got)
	}
	if !strings.Contains(got, "fix: resolve applicant history issue") {
		t.Errorf("Run() output = %q, want it to show the message", got)
	}
}

func TestRunCommitAcceptsEverySupportedType(t *testing.T) {
	for _, commitType := range []string{"feat", "fix", "ui", "refactor", "test", "docs", "chore", "ci", "perf"} {
		t.Run(commitType, func(t *testing.T) {
			service := &fakeService{}
			env, _, _ := newEnv(service)

			if code := Run(context.Background(), env, []string{"commit", commitType, "a message"}); code != ExitOK {
				t.Errorf("Run(%q) = %d, want %d", commitType, code, ExitOK)
			}
			if service.commitReq.Type != commitType {
				t.Errorf("Type = %q, want %q", service.commitReq.Type, commitType)
			}
		})
	}
}

// The type is normalised at the boundary so the recorded message is always the
// conventional lower case form.
func TestRunCommitNormalizesTheType(t *testing.T) {
	service := &fakeService{}
	env, _, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"commit", "FIX", "a message"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if want := "fix"; service.commitReq.Type != want {
		t.Errorf("Type = %q, want %q", service.commitReq.Type, want)
	}
}

func TestRunCommitJoinsAnUnquotedMessage(t *testing.T) {
	service := &fakeService{}
	env, _, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"commit", "fix", "resolve", "the", "thing"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if want := "resolve the thing"; service.commitReq.Message != want {
		t.Errorf("Message = %q, want %q", service.commitReq.Message, want)
	}
}

func TestRunCommitRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no arguments", args: []string{"commit"}, want: "commit needs a type and a message"},
		{name: "type only", args: []string{"commit", "fix"}, want: "commit needs a type and a message"},
		{name: "unknown type", args: []string{"commit", "bugfix", "a message"}, want: "invalid commit type"},
		{name: "style is not accepted", args: []string{"commit", "style", "a message"}, want: "invalid commit type"},
		{name: "empty message", args: []string{"commit", "fix", "   "}, want: "the commit message is empty"},
		{name: "a flag", args: []string{"commit", "--amend"}, want: `unexpected flag "--amend"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{}
			env, _, errOut := newEnv(service)

			if code := Run(context.Background(), env, test.args); code != ExitUsage {
				t.Errorf("Run(%v) = %d, want %d", test.args, code, ExitUsage)
			}
			if !strings.Contains(errOut.String(), test.want) {
				t.Errorf("Run(%v) error output = %q, want it to contain %q", test.args, errOut.String(), test.want)
			}
			if service.commitCalls != 0 {
				t.Errorf("Commit calls = %d, want none", service.commitCalls)
			}
		})
	}
}

func TestRunCommitReportsNothingStaged(t *testing.T) {
	service := &fakeService{commitErr: &workflow.NothingStagedError{Unstaged: 2, Untracked: 1}}
	env, out, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"commit", "fix", "a message"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if !strings.Contains(errOut.String(), "nothing is staged") {
		t.Errorf("Run() error output = %q, want it to explain the problem", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("Run() output = %q, want none", out.String())
	}
}

func TestRunCommitReportsWriteFailure(t *testing.T) {
	service := &fakeService{commitRes: workflow.CommitResult{Message: "fix: x"}}
	env, _, errOut := newEnv(service)
	env.Out = failingWriter{}

	if code := Run(context.Background(), env, []string{"commit", "fix", "x"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if !strings.Contains(errOut.String(), "write failed") {
		t.Errorf("Run() error output = %q, want it to report the broken stream", errOut.String())
	}
}
