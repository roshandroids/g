package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/config"
	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/workflow"
)

const testDir = "/work/demo"

// fakeService records the directories it is asked about and returns a fixed
// result, so commands are tested without a repository.
type fakeService struct {
	status workflow.Status
	err    error
	dirs   []string

	newRequest  workflow.NewBranchRequest
	newResult   workflow.NewBranchResult
	newErr      error
	newCalls    int
	switchReq   workflow.SwitchRequest
	switchRes   workflow.SwitchResult
	switchErr   error
	switchCalls int

	commitReq   workflow.CommitRequest
	commitRes   workflow.CommitResult
	commitErr   error
	commitCalls int

	pushReq   workflow.PushRequest
	pushRes   workflow.PushResult
	pushErr   error
	pushCalls int

	syncRes   workflow.SyncResult
	syncErr   error
	syncCalls int

	continueRes   workflow.ContinueResult
	continueErr   error
	continueCalls int

	currentOp    git.Operation
	currentOpErr error

	abortRes   workflow.AbortResult
	abortErr   error
	abortCalls int

	undoCount int
	undoRes   workflow.UndoResult
	undoErr   error
	undoCalls int

	cleanReq   workflow.CleanRequest
	cleanRes   workflow.CleanResult
	cleanErr   error
	cleanCalls int
}

func (f *fakeService) Status(_ context.Context, dir string) (workflow.Status, error) {
	f.dirs = append(f.dirs, dir)
	if f.err != nil {
		return workflow.Status{}, f.err
	}
	return f.status, nil
}

func (f *fakeService) NewBranch(_ context.Context, dir string, req workflow.NewBranchRequest) (workflow.NewBranchResult, error) {
	f.dirs = append(f.dirs, dir)
	f.newCalls++
	f.newRequest = req
	if f.newErr != nil {
		return workflow.NewBranchResult{}, f.newErr
	}
	return f.newResult, nil
}

func (f *fakeService) Switch(_ context.Context, dir string, req workflow.SwitchRequest) (workflow.SwitchResult, error) {
	f.dirs = append(f.dirs, dir)
	f.switchCalls++
	f.switchReq = req
	if f.switchErr != nil {
		return workflow.SwitchResult{}, f.switchErr
	}
	return f.switchRes, nil
}

func (f *fakeService) Commit(_ context.Context, dir string, req workflow.CommitRequest) (workflow.CommitResult, error) {
	f.dirs = append(f.dirs, dir)
	f.commitCalls++
	f.commitReq = req
	if f.commitErr != nil {
		return workflow.CommitResult{}, f.commitErr
	}
	return f.commitRes, nil
}

func (f *fakeService) Push(_ context.Context, dir string, req workflow.PushRequest) (workflow.PushResult, error) {
	f.dirs = append(f.dirs, dir)
	f.pushCalls++
	f.pushReq = req
	if f.pushErr != nil {
		return workflow.PushResult{}, f.pushErr
	}
	return f.pushRes, nil
}

func (f *fakeService) Sync(_ context.Context, dir string) (workflow.SyncResult, error) {
	f.dirs = append(f.dirs, dir)
	f.syncCalls++
	if f.syncErr != nil {
		return workflow.SyncResult{}, f.syncErr
	}
	return f.syncRes, nil
}

func (f *fakeService) Continue(_ context.Context, dir string) (workflow.ContinueResult, error) {
	f.dirs = append(f.dirs, dir)
	f.continueCalls++
	if f.continueErr != nil {
		return workflow.ContinueResult{}, f.continueErr
	}
	return f.continueRes, nil
}

func (f *fakeService) CurrentOperation(_ context.Context, dir string) (git.Operation, error) {
	f.dirs = append(f.dirs, dir)
	if f.currentOpErr != nil {
		return git.OperationNone, f.currentOpErr
	}
	return f.currentOp, nil
}

func (f *fakeService) Abort(_ context.Context, dir string) (workflow.AbortResult, error) {
	f.dirs = append(f.dirs, dir)
	f.abortCalls++
	if f.abortErr != nil {
		return workflow.AbortResult{}, f.abortErr
	}
	return f.abortRes, nil
}

func (f *fakeService) Undo(_ context.Context, dir string, count int) (workflow.UndoResult, error) {
	f.dirs = append(f.dirs, dir)
	f.undoCalls++
	f.undoCount = count
	if f.undoErr != nil {
		return workflow.UndoResult{}, f.undoErr
	}
	return f.undoRes, nil
}

func (f *fakeService) Clean(_ context.Context, dir string, req workflow.CleanRequest) (workflow.CleanResult, error) {
	f.dirs = append(f.dirs, dir)
	f.cleanCalls++
	f.cleanReq = req
	if f.cleanErr != nil {
		return workflow.CleanResult{}, f.cleanErr
	}
	return f.cleanRes, nil
}

func TestRunWithoutArgumentsShowsShortHelp(t *testing.T) {
	env, out, errOut := newEnv(&fakeService{})

	if code := Run(context.Background(), env, nil); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(out.String(), "Commands: status, help, new, switch, commit, push, sync, continue, abort, undo, clean") {
		t.Errorf("Run() output = %q, want it to list the available commands", out.String())
	}
	if !strings.Contains(out.String(), "g --help") {
		t.Errorf("Run() output = %q, want it to point at `g --help`", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("Run() error output = %q, want none", errOut.String())
	}
}

func TestRunShowsHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			env, out, errOut := newEnv(&fakeService{})

			if code := Run(context.Background(), env, []string{arg}); code != ExitOK {
				t.Errorf("Run(%q) = %d, want %d", arg, code, ExitOK)
			}
			if errOut.Len() != 0 {
				t.Errorf("Run(%q) error output = %q, want none", arg, errOut.String())
			}

			got := out.String()
			for _, want := range []string{"Commands:", "g status", "g sync", "g continue", "g undo", "g clean", "Exit codes:"} {
				if !strings.Contains(got, want) {
					t.Errorf("Run(%q) output missing %q:\n%s", arg, want, got)
				}
			}
			if strings.Contains(got, "Planned (not implemented yet):") {
				t.Errorf("Run(%q) output still lists planned commands:\n%s", arg, got)
			}
		})
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	env, out, errOut := newEnv(&fakeService{})

	if code := Run(context.Background(), env, []string{"frobnicate"}); code != ExitUsage {
		t.Errorf("Run() = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(errOut.String(), `unknown command "frobnicate"`) {
		t.Errorf("Run() error output = %q, want it to name the command", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("Run() output = %q, want help on the error stream instead", out.String())
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	env, _, errOut := newEnv(&fakeService{})

	if code := Run(context.Background(), env, []string{"--frobnicate"}); code != ExitUsage {
		t.Errorf("Run() = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(errOut.String(), `unknown flag "--frobnicate"`) {
		t.Errorf("Run() error output = %q, want it to report a flag", errOut.String())
	}
}

func TestRunUndo(t *testing.T) {
	service := &fakeService{undoRes: workflow.UndoResult{
		Branch:  "feature",
		Count:   1,
		Commits: []git.CommitSummary{{Hash: "abc1234", Subject: "fix: thing"}},
	}}
	env, out, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"undo"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if errOut.Len() != 0 {
		t.Errorf("Run() error output = %q, want none", errOut.String())
	}
	if service.undoCalls != 1 || service.undoCount != 1 {
		t.Errorf("Undo calls/count = %d/%d, want 1/1", service.undoCalls, service.undoCount)
	}
	if !strings.Contains(out.String(), "Changes preserved and staged") {
		t.Errorf("Run() output = %q, want undo summary", out.String())
	}
}

func TestRunUndoWithCount(t *testing.T) {
	service := &fakeService{undoRes: workflow.UndoResult{Count: 2, Commits: []git.CommitSummary{
		{Hash: "a", Subject: "one"},
		{Hash: "b", Subject: "two"},
	}}}
	env, _, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"undo", "2"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if service.undoCount != 2 {
		t.Errorf("undoCount = %d, want 2", service.undoCount)
	}
}

func TestRunUndoRejectsInvalidCount(t *testing.T) {
	env, _, errOut := newEnv(&fakeService{})

	if code := Run(context.Background(), env, []string{"undo", "0"}); code != ExitUsage {
		t.Errorf("Run() = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(errOut.String(), "positive integer") {
		t.Errorf("error = %q, want positive integer message", errOut.String())
	}
}

func TestRunAbortRequiresConfirmation(t *testing.T) {
	service := &fakeService{
		currentOp: git.OperationRebase,
		abortRes:  workflow.AbortResult{Operation: git.OperationRebase},
	}
	prompter := &fakePrompter{answers: []bool{true}}
	env, out, _ := newEnv(service)
	env.Prompt = prompter

	if code := Run(context.Background(), env, []string{"abort"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if service.abortCalls != 1 {
		t.Errorf("abortCalls = %d, want 1", service.abortCalls)
	}
	if !strings.Contains(out.String(), "Rebase aborted") {
		t.Errorf("output = %q, want aborted message", out.String())
	}
}

func TestRunAbortDeclined(t *testing.T) {
	service := &fakeService{currentOp: git.OperationMerge}
	prompter := &fakePrompter{answers: []bool{false}}
	env, _, errOut := newEnv(service)
	env.Prompt = prompter

	if code := Run(context.Background(), env, []string{"abort"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if service.abortCalls != 0 {
		t.Errorf("abortCalls = %d, want 0", service.abortCalls)
	}
	if !strings.Contains(errOut.String(), "nothing was aborted") {
		t.Errorf("error = %q, want declined message", errOut.String())
	}
}

func TestRunCleanPreview(t *testing.T) {
	service := &fakeService{cleanRes: workflow.CleanResult{
		Candidates: []git.StaleBranch{{Name: "old", Reason: git.StaleMerged, Safe: true}},
	}}
	env, out, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"clean"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if service.cleanCalls != 1 || service.cleanReq.Apply {
		t.Errorf("cleanCalls/Apply = %d/%v, want preview", service.cleanCalls, service.cleanReq.Apply)
	}
	if !strings.Contains(out.String(), "Preview only") {
		t.Errorf("output = %q, want preview", out.String())
	}
}

func TestRunContinue(t *testing.T) {
	service := &fakeService{
		status:      workflow.Status{Branch: "main", Operation: git.OperationRebase},
		continueRes: workflow.ContinueResult{Operation: git.OperationRebase, Branch: "main"},
	}
	env, out, _ := newEnv(service)

	if code := Run(context.Background(), env, []string{"continue"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if service.continueCalls != 1 {
		t.Errorf("continueCalls = %d, want 1", service.continueCalls)
	}
	if !strings.Contains(out.String(), "Rebase continued successfully") {
		t.Errorf("output = %q, want success", out.String())
	}
}

func TestRunSync(t *testing.T) {
	service := &fakeService{syncRes: workflow.SyncResult{Branch: "feature", Base: "main", Ahead: 2}}
	env, out, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"sync"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if errOut.Len() != 0 {
		t.Errorf("Run() error output = %q, want none", errOut.String())
	}
	if !strings.Contains(out.String(), "Syncing feature") {
		t.Errorf("Run() output = %q, want the sync summary", out.String())
	}
	if service.syncCalls != 1 {
		t.Errorf("syncCalls = %d, want 1", service.syncCalls)
	}
}

func TestRunSyncRejectsArguments(t *testing.T) {
	env, _, errOut := newEnv(&fakeService{})

	if code := Run(context.Background(), env, []string{"sync", "extra"}); code != ExitUsage {
		t.Errorf("Run() = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(errOut.String(), "sync takes no arguments") {
		t.Errorf("Run() error output = %q, want it to explain the problem", errOut.String())
	}
}

func TestRunStatus(t *testing.T) {
	service := &fakeService{status: workflow.Status{Name: "demo", Branch: "main"}}
	env, out, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"status"}); code != ExitOK {
		t.Errorf("Run() = %d, want %d", code, ExitOK)
	}
	if errOut.Len() != 0 {
		t.Errorf("Run() error output = %q, want none", errOut.String())
	}
	if !strings.Contains(out.String(), "Repository: demo") {
		t.Errorf("Run() output = %q, want the repository summary", out.String())
	}
	if want := []string{testDir}; len(service.dirs) != 1 || service.dirs[0] != want[0] {
		t.Errorf("status ran in %v, want %v", service.dirs, want)
	}
}

func TestRunStatusRejectsArguments(t *testing.T) {
	env, _, errOut := newEnv(&fakeService{})

	if code := Run(context.Background(), env, []string{"status", "extra"}); code != ExitUsage {
		t.Errorf("Run() = %d, want %d", code, ExitUsage)
	}
	got := errOut.String()
	if !strings.Contains(got, "status takes no arguments") {
		t.Errorf("Run() error output = %q, want it to explain the problem", got)
	}
	if !strings.Contains(got, "usage: g status") {
		t.Errorf("Run() error output = %q, want it to show the usage", got)
	}
}

func TestRunStatusReportsNotARepository(t *testing.T) {
	service := &fakeService{err: fmt.Errorf("%w: %s", workflow.ErrNotARepository, testDir)}
	env, _, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"status"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if got, want := strings.TrimSpace(errOut.String()), "g: not inside a Git repository: "+testDir; got != want {
		t.Errorf("Run() error output = %q, want %q", got, want)
	}
}

func TestRunStatusReportsWorkflowFailure(t *testing.T) {
	service := &fakeService{err: errors.New("git exploded")}
	env, _, errOut := newEnv(service)

	if code := Run(context.Background(), env, []string{"status"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if got, want := strings.TrimSpace(errOut.String()), "g: git exploded"; got != want {
		t.Errorf("Run() error output = %q, want %q", got, want)
	}
}

func TestRunStatusReportsWriteFailure(t *testing.T) {
	service := &fakeService{status: workflow.Status{Name: "demo", Branch: "main"}}
	env, _, errOut := newEnv(service)
	env.Out = failingWriter{}

	if code := Run(context.Background(), env, []string{"status"}); code != ExitError {
		t.Errorf("Run() = %d, want %d", code, ExitError)
	}
	if !strings.Contains(errOut.String(), "write failed") {
		t.Errorf("Run() error output = %q, want it to report the broken stream", errOut.String())
	}
}

func TestCommandsAreWellFormed(t *testing.T) {
	seen := make(map[string]bool)

	for _, command := range Commands() {
		if command.Name == "" {
			t.Error("command with an empty name")
		}
		if seen[command.Name] {
			t.Errorf("command %q is declared twice", command.Name)
		}
		seen[command.Name] = true

		if want := "g " + command.Name; !strings.HasPrefix(command.Usage, want) {
			t.Errorf("command %q has usage %q, want it to start with %q", command.Name, command.Usage, want)
		}
		if command.Summary == "" {
			t.Errorf("command %q has no summary", command.Name)
		}
		switch {
		case !command.Available() && command.Roadmap == "":
			t.Errorf("planned command %q must name the milestone that delivers it", command.Name)
		case command.Available() && command.Roadmap != "":
			t.Errorf("available command %q must not be marked planned", command.Name)
		}
	}

	if len(seen) != len(Commands()) {
		t.Errorf("found %d distinct commands, want %d", len(seen), len(Commands()))
	}
}

func TestFind(t *testing.T) {
	command, ok := Find("status")
	if !ok {
		t.Fatal("Find(status) reported no such command")
	}
	if !command.Available() {
		t.Error("status must be available")
	}

	if _, ok := Find("help"); !ok {
		t.Error("Find(help) reported no such command")
	}
	if _, ok := Find("frobnicate"); ok {
		t.Error("Find(frobnicate) reported a command, want none")
	}
}

// newEnv returns an environment wired to service, plus the output buffers.
func newEnv(service Service) (env *Env, out, errOut *bytes.Buffer) {
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	env = &Env{
		Service: service,
		Config:  config.Default(),
		Dir:     testDir,
		Out:     out,
		Err:     errOut,
	}
	return env, out, errOut
}

// failingWriter reports every write as a failure.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
