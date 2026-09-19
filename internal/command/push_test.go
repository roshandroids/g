package command

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/workflow"
)

func TestRunPush(t *testing.T) {
	service := &fakeService{pushRes: workflow.PushResult{Branch: "HCM-1-work", Upstream: "origin/HCM-1-work"}}
	env, out, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"push"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if errOut.Len() != 0 {
		t.Errorf("Run() error output = %q, want none", errOut.String())
	}
	if service.pushCalls != 1 {
		t.Errorf("Push calls = %d, want 1", service.pushCalls)
	}
	if service.pushReq.Force {
		t.Error("Force = true, want a plain push by default")
	}
	if !strings.Contains(out.String(), "Pushed HCM-1-work to origin/HCM-1-work") {
		t.Errorf("Run() output = %q, want it to report the push", out.String())
	}
}

func TestRunPushRejectsUnexpectedArguments(t *testing.T) {
	for _, arg := range []string{"origin", "main", "-f", "--frobnicate"} {
		t.Run(arg, func(t *testing.T) {
			service := &fakeService{}
			env, _, errOut := newEnv(service)

			if code := Run(context.Background(), env, []string{"push", arg}); code != ExitUsage {
				t.Errorf("Run(push %s) = %d, want %d", arg, code, ExitUsage)
			}
			if !strings.Contains(errOut.String(), "push takes no arguments other than --force") {
				t.Errorf("Run(push %s) error output = %q, want it to explain the flags", arg, errOut.String())
			}
			if service.pushCalls != 0 {
				t.Errorf("Push calls = %d, want none", service.pushCalls)
			}
		})
	}
}

func TestRunPushReportsANewUpstream(t *testing.T) {
	service := &fakeService{pushRes: workflow.PushResult{
		Branch:      "HCM-1-work",
		Upstream:    "origin/HCM-1-work",
		SetUpstream: true,
	}}
	env, out, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"push"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if service.pushCalls != 1 {
		t.Errorf("Push calls = %d, want 1", service.pushCalls)
	}
	if !strings.Contains(out.String(), "set upstream to origin/HCM-1-work") {
		t.Errorf("Run() output = %q, want it to report the new upstream", out.String())
	}
}

func TestRunPushForceRequiresConfirmation(t *testing.T) {
	service := &fakeService{
		status:  workflow.Status{Branch: "work", Upstream: &workflow.Upstream{Name: "origin/work"}},
		pushRes: workflow.PushResult{Branch: "work", Upstream: "origin/work", Forced: true},
	}

	prompter := &fakePrompter{answers: []bool{true}}
	env, out, _ := newEnv(service)
	env.Prompt = prompter

	if code := Run(context.Background(), env, []string{"push", "--force"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if len(prompter.questions) != 1 {
		t.Fatalf("asked %d questions, want 1", len(prompter.questions))
	}
	if !strings.Contains(prompter.questions[0], "--force-with-lease") {
		t.Errorf("question = %q, want it to name the flag being used", prompter.questions[0])
	}
	if !service.pushReq.Force {
		t.Error("Force = false, want the confirmed force to reach the workflow")
	}
	if !strings.Contains(out.String(), "--force-with-lease") {
		t.Errorf("Run() output = %q, want it to say how the push was made", out.String())
	}
}

func TestRunPushForceDeclinedDoesNotPush(t *testing.T) {
	service := &fakeService{
		status:  workflow.Status{Branch: "work", Upstream: &workflow.Upstream{Name: "origin/work"}},
		pushRes: workflow.PushResult{Branch: "work", Upstream: "origin/work"},
	}

	prompter := &fakePrompter{answers: []bool{false}}
	env, _, errOut := newEnv(service)
	env.Prompt = prompter

	if code := Run(context.Background(), env, []string{"push", "--force"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if !strings.Contains(errOut.String(), "nothing was pushed") {
		t.Errorf("Run() error output = %q, want it to refuse", errOut.String())
	}
	if service.pushCalls != 0 {
		t.Errorf("Push calls = %d, want none after declining", service.pushCalls)
	}
}

// Asking about a force push that cannot happen would be nonsense, so the
// missing upstream is reported instead of a question.
func TestRunPushForceWithoutAnUpstreamDoesNotAsk(t *testing.T) {
	service := &fakeService{status: workflow.Status{Branch: "work"}}

	prompter := &fakePrompter{answers: []bool{true}}
	env, _, errOut := newEnv(service)
	env.Prompt = prompter

	if code := Run(context.Background(), env, []string{"push", "--force"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if len(prompter.questions) != 0 {
		t.Errorf("asked %q, want no question when there is nothing to force", prompter.questions)
	}
	if !strings.Contains(errOut.String(), "no upstream") {
		t.Errorf("Run() error output = %q, want it to explain the missing upstream", errOut.String())
	}
	if service.pushCalls != 0 {
		t.Errorf("Push calls = %d, want none", service.pushCalls)
	}
}

func TestRunPushForceWithoutAPrompterDoesNotPush(t *testing.T) {
	service := &fakeService{status: workflow.Status{Branch: "work", Upstream: &workflow.Upstream{Name: "origin/work"}}}

	env, _, errOut := newEnv(service)
	env.Prompt = nil

	if code := Run(context.Background(), env, []string{"push", "--force"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if !strings.Contains(errOut.String(), "nothing was pushed") {
		t.Errorf("Run() error output = %q, want it to refuse", errOut.String())
	}
	if service.pushCalls != 0 {
		t.Errorf("Push calls = %d, want none", service.pushCalls)
	}
}

func TestRunPushReportsFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "no remote", err: &workflow.NoRemoteError{}, want: "no remote is configured"},
		{name: "detached head", err: errors.New("HEAD is detached at a1b2c3d"), want: "detached"},
		{name: "rejected", err: errors.New("! [rejected] non-fast-forward"), want: "non-fast-forward"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{pushErr: test.err}
			env, _, errOut := newEnv(service)

			if code := Run(context.Background(), env, []string{"push"}); code != ExitError {
				t.Errorf("Run() = %d, want %d", code, ExitError)
			}
			if !strings.Contains(errOut.String(), test.want) {
				t.Errorf("Run() error output = %q, want it to contain %q", errOut.String(), test.want)
			}
		})
	}
}

func TestRunPushReportsWriteFailure(t *testing.T) {
	service := &fakeService{pushRes: workflow.PushResult{Branch: "work", Upstream: "origin/work"}}
	env, _, errOut := newEnv(service)
	env.Out = failingWriter{}

	if code := Run(context.Background(), env, []string{"push"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if !strings.Contains(errOut.String(), "write failed") {
		t.Errorf("Run() error output = %q, want it to report the broken stream", errOut.String())
	}
}

// fakePrompter answers each question from a script and records what it was
// asked, so the confirmation flow is testable without a terminal.
type fakePrompter struct {
	answers   []bool
	questions []string
	err       error
}

func (f *fakePrompter) Confirm(_ context.Context, question string) (bool, error) {
	f.questions = append(f.questions, question)
	if f.err != nil {
		return false, f.err
	}
	if len(f.questions) > len(f.answers) {
		return false, nil
	}
	return f.answers[len(f.questions)-1], nil
}
