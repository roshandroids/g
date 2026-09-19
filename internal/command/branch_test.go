package command

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/workflow"
)

func TestRunNew(t *testing.T) {
	service := &fakeService{newResult: workflow.NewBranchResult{
		Branch: "HCM-37538-applicant-id-update",
		Base:   "main",
	}}
	env, out, errOut := newEnv(service)

	code := Run(context.Background(), env, []string{"new", "HCM-37538", "applicant id update"})
	if code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if errOut.Len() != 0 {
		t.Errorf("Run() error output = %q, want none", errOut.String())
	}

	want := workflow.NewBranchRequest{Ticket: "HCM-37538", Description: "applicant id update"}
	if service.newRequest != want {
		t.Errorf("NewBranch request = %+v, want %+v", service.newRequest, want)
	}
	if want := []string{testDir}; len(service.dirs) != 1 || service.dirs[0] != want[0] {
		t.Errorf("new ran in %v, want %v", service.dirs, want)
	}
	if !strings.Contains(out.String(), "Created branch HCM-37538-applicant-id-update from main") {
		t.Errorf("Run() output = %q, want it to report the created branch", out.String())
	}
}

// The usage quotes the description, but an unquoted one has to behave the same
// way rather than failing as an unexpected argument.
func TestRunNewJoinsAnUnquotedDescription(t *testing.T) {
	service := &fakeService{}
	env, _, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"new", "HCM-1", "applicant", "id", "update"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if want := "applicant id update"; service.newRequest.Description != want {
		t.Errorf("Description = %q, want %q", service.newRequest.Description, want)
	}
}

func TestRunNewWithoutADescription(t *testing.T) {
	service := &fakeService{}
	env, _, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"new", "HCM-1"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if service.newRequest.Description != "" {
		t.Errorf("Description = %q, want empty", service.newRequest.Description)
	}
}

func TestRunNewRejectsMissingArguments(t *testing.T) {
	service := &fakeService{}
	env, _, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"new"}); code != ExitUsage {
		t.Errorf("Run() = %d, want %d", code, ExitUsage)
	}

	got := errOut.String()
	if !strings.Contains(got, "new needs an issue key") {
		t.Errorf("Run() error output = %q, want it to explain the problem", got)
	}
	if !strings.Contains(got, "usage: g new <ticket> [description]") {
		t.Errorf("Run() error output = %q, want it to show the usage", got)
	}
	if service.newCalls != 0 {
		t.Errorf("NewBranch calls = %d, want none", service.newCalls)
	}
}

func TestRunNewRejectsAFlag(t *testing.T) {
	service := &fakeService{}
	env, _, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"new", "--base"}); code != ExitUsage {
		t.Errorf("Run() = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(errOut.String(), `unexpected flag "--base"`) {
		t.Errorf("Run() error output = %q, want it to name the flag", errOut.String())
	}
	if service.newCalls != 0 {
		t.Errorf("NewBranch calls = %d, want none", service.newCalls)
	}
}

func TestRunNewReportsWorkflowFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "branch exists",
			err:  &workflow.BranchExistsError{Name: "HCM-1-work"},
			want: "already exists",
		},
		{
			name: "detached head",
			err:  errors.New("HEAD is detached at a1b2c3d"),
			want: "detached",
		},
		{
			name: "rebase in progress",
			err:  &workflow.OperationError{},
			want: "in progress",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{newErr: test.err}
			env, out, errOut := newEnv(service)

			if code := Run(context.Background(), env, []string{"new", "HCM-1"}); code != ExitError {
				t.Errorf("Run() = %d, want %d", code, ExitError)
			}
			if !strings.Contains(errOut.String(), test.want) {
				t.Errorf("Run() error output = %q, want it to contain %q", errOut.String(), test.want)
			}
			if out.Len() != 0 {
				t.Errorf("Run() output = %q, want none", out.String())
			}
		})
	}
}

func TestRunSwitch(t *testing.T) {
	service := &fakeService{switchRes: workflow.SwitchResult{Branch: "main", Previous: "feature/x"}}
	env, out, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"switch", "main"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if errOut.Len() != 0 {
		t.Errorf("Run() error output = %q, want none", errOut.String())
	}
	if want := (workflow.SwitchRequest{Branch: "main"}); service.switchReq != want {
		t.Errorf("Switch request = %+v, want %+v", service.switchReq, want)
	}
	if !strings.Contains(out.String(), "Switched to branch main (from feature/x)") {
		t.Errorf("Run() output = %q, want it to report the switch", out.String())
	}
}

func TestRunSwitchAcceptsABranchNameContainingASlash(t *testing.T) {
	service := &fakeService{}
	env, _, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"switch", "feature/branch-workflows"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if want := "feature/branch-workflows"; service.switchReq.Branch != want {
		t.Errorf("Branch = %q, want %q", service.switchReq.Branch, want)
	}
}

func TestRunSwitchRejectsMissingOrExtraArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no branch", args: []string{"switch"}, want: "switch needs the name of a branch"},
		{name: "two branches", args: []string{"switch", "main", "dev"}, want: "takes one branch name"},
		{name: "a flag", args: []string{"switch", "--all"}, want: `unexpected flag "--all"`},
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
			if !strings.Contains(errOut.String(), "usage: g switch <branch>") {
				t.Errorf("Run(%v) error output = %q, want it to show the usage", test.args, errOut.String())
			}
			if service.switchCalls != 0 {
				t.Errorf("Switch calls = %d, want none", service.switchCalls)
			}
		})
	}
}

func TestRunSwitchReportsAnUnknownBranch(t *testing.T) {
	service := &fakeService{switchErr: &workflow.BranchNotFoundError{
		Name:      "nope",
		Available: []string{"main", "dev"},
	}}
	env, _, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"switch", "nope"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}

	got := errOut.String()
	if !strings.Contains(got, `branch "nope" does not exist`) {
		t.Errorf("Run() error output = %q, want it to name the branch", got)
	}
	if !strings.Contains(got, "main, dev") {
		t.Errorf("Run() error output = %q, want it to list the branches that exist", got)
	}
}

func TestRunNewReportsWriteFailure(t *testing.T) {
	service := &fakeService{newResult: workflow.NewBranchResult{Branch: "HCM-1", Base: "main"}}
	env, _, errOut := newEnv(service)
	env.Out = failingWriter{}

	if code := Run(context.Background(), env, []string{"new", "HCM-1"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if !strings.Contains(errOut.String(), "write failed") {
		t.Errorf("Run() error output = %q, want it to report the broken stream", errOut.String())
	}
}

func TestRunSwitchReportsWriteFailure(t *testing.T) {
	service := &fakeService{switchRes: workflow.SwitchResult{Branch: "main"}}
	env, _, errOut := newEnv(service)
	env.Out = failingWriter{}

	if code := Run(context.Background(), env, []string{"switch", "main"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if !strings.Contains(errOut.String(), "write failed") {
		t.Errorf("Run() error output = %q, want it to report the broken stream", errOut.String())
	}
}
