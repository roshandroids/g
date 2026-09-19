// Package tests holds integration tests that exercise g against real Git
// repositories.
package tests

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/process"
	"github.com/roshandroids/g/internal/workflow"
)

func TestNewBranchCreatesAndSwitches(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")

	result, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{
		Ticket:      "HCM-37538",
		Description: "applicant id update",
	})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}

	if want := "HCM-37538-applicant-id-update"; result.Branch != want {
		t.Errorf("Branch = %q, want %q", result.Branch, want)
	}
	if result.Base != "main" {
		t.Errorf("Base = %q, want %q", result.Base, "main")
	}
	if result.Dirty {
		t.Error("Dirty = true, want false for a clean repository")
	}

	if got := currentBranch(t, dir); got != "HCM-37538-applicant-id-update" {
		t.Errorf("current branch = %q, want the branch to be checked out", got)
	}
}

func TestNewBranchWorksInARepositoryOnMaster(t *testing.T) {
	requireGit(t)

	dir := initRepositoryOn(t, "master", "README.md")

	result, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{Ticket: "HCM-1"})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}
	if result.Base != "master" {
		t.Errorf("Base = %q, want %q", result.Base, "master")
	}
}

// Creating a branch must not reach for the network: the starting point has to
// be the local base branch as it stands, or the branch would depend on remote
// state that is not visible in the repository.
func TestNewBranchDoesNotContactTheRemote(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "remote", "add", "origin", "https://invalid.example.com/nope.git")

	_, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{Ticket: "HCM-1"})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil even with an unreachable remote", err)
	}
}

func TestNewBranchCarriesUncommittedChangesAcross(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	writeFile(t, dir, "README.md", "changed\n")

	result, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{Ticket: "HCM-1"})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}
	if !result.Dirty {
		t.Error("Dirty = false, want true")
	}

	// The whole point of reporting rather than blocking is that the work is
	// still there afterwards.
	status := statusOf(t, dir)
	if want := 1; len(status.Unstaged) != want {
		t.Errorf("Unstaged = %+v, want %d entry so nothing was discarded", status.Unstaged, want)
	}
}

func TestNewBranchRejectsAnExistingBranch(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "branch", "HCM-1-work")

	_, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{
		Ticket:      "HCM-1",
		Description: "work",
	})
	if !errors.Is(err, workflow.ErrBranchExists) {
		t.Fatalf("NewBranch() error = %v, want ErrBranchExists", err)
	}

	if got := currentBranch(t, dir); got != "main" {
		t.Errorf("current branch = %q, want main to be left alone", got)
	}
}

func TestNewBranchReportsABranchThatOnlyExistsOnTheRemote(t *testing.T) {
	requireGit(t)

	upstream := initBareRepository(t)
	dir := initRepository(t, "README.md")
	runGit(t, dir, "remote", "add", "origin", upstream)
	runGit(t, dir, "push", "-q", "origin", "main")
	runGit(t, dir, "push", "-q", "origin", "main:HCM-1-work")
	runGit(t, dir, "fetch", "-q", "origin")

	result, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{
		Ticket:      "HCM-1",
		Description: "work",
	})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}
	if !result.RemoteExists {
		t.Fatal("RemoteExists = false, want true")
	}
	if result.Remote != "origin" {
		t.Errorf("Remote = %q, want %q", result.Remote, "origin")
	}
}

func TestNewBranchRejectsADetachedHead(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "--detach", "HEAD")

	_, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{Ticket: "HCM-1"})
	if !errors.Is(err, workflow.ErrDetachedHead) {
		t.Fatalf("NewBranch() error = %v, want ErrDetachedHead", err)
	}
}

// During a rebase git reports an empty branch name, so a naive implementation
// would blame a "detached HEAD". The operation has to be recognised, because
// creating a branch mid-rebase would leave the rebase with nowhere to finish.
func TestNewBranchRejectsARebaseInProgress(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	startConflictingRebase(t, dir)

	// Guard against a vacuous test: the repository has to really be mid-rebase,
	// which is the state that used to be mistaken for a detached HEAD.
	raw, err := git.NewClient(process.ExecRunner{}).Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}
	if raw.Operation != git.OperationRebase {
		t.Fatalf("Operation = %q, want %q before asserting on the error", raw.Operation, git.OperationRebase)
	}
	if !raw.Detached {
		t.Fatal("Detached = false, want true so the mistake this guards against is possible")
	}

	_, err = newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{Ticket: "HCM-1"})
	if !errors.Is(err, workflow.ErrOperationInProgress) {
		t.Fatalf("NewBranch() error = %v, want ErrOperationInProgress", err)
	}
	if errors.Is(err, workflow.ErrDetachedHead) {
		t.Error("NewBranch() blamed a detached HEAD, want the rebase reported instead")
	}
}

func TestNewBranchRejectsAMergeInProgress(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	startConflictingMerge(t, dir)

	_, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{Ticket: "HCM-1"})
	if !errors.Is(err, workflow.ErrOperationInProgress) {
		t.Fatalf("NewBranch() error = %v, want ErrOperationInProgress", err)
	}
}

func TestNewBranchWithoutABaseBranch(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "branch", "-m", "main", "topic")

	_, err := newService(t, workflow.Options{}).NewBranch(context.Background(), dir, workflow.NewBranchRequest{Ticket: "HCM-1"})
	if err == nil {
		t.Fatal("NewBranch() error = nil, want it to report the missing base branch")
	}
	if !strings.Contains(err.Error(), "cannot find a base branch") {
		t.Errorf("NewBranch() error = %v, want it to explain the missing base", err)
	}
}

func TestSwitchToAnExistingBranch(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "branch", "feature/deep/nested")

	result, err := newService(t, workflow.Options{}).Switch(context.Background(), dir, workflow.SwitchRequest{Branch: "feature/deep/nested"})
	if err != nil {
		t.Fatalf("Switch() error = %v, want nil", err)
	}

	if result.Previous != "main" {
		t.Errorf("Previous = %q, want %q", result.Previous, "main")
	}
	if got := currentBranch(t, dir); got != "feature/deep/nested" {
		t.Errorf("current branch = %q, want %q", got, "feature/deep/nested")
	}
}

func TestSwitchReportsAnUnknownBranch(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")

	_, err := newService(t, workflow.Options{}).Switch(context.Background(), dir, workflow.SwitchRequest{Branch: "nope"})
	if !errors.Is(err, workflow.ErrBranchNotFound) {
		t.Fatalf("Switch() error = %v, want ErrBranchNotFound", err)
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("Switch() error = %v, want it to list the branches that exist", err)
	}
	if got := currentBranch(t, dir); got != "main" {
		t.Errorf("current branch = %q, want main to be left alone", got)
	}
}

// git refuses to switch when that would overwrite local changes. That refusal
// is the safety net that keeps uncommitted work, so it must surface rather than
// be worked around.
func TestSwitchRefusesToOverwriteLocalChanges(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "other")
	writeFile(t, dir, "README.md", "other content\n")
	runGit(t, dir, "commit", "-qam", "other change")

	runGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "README.md", "uncommitted local content\n")

	_, err := newService(t, workflow.Options{}).Switch(context.Background(), dir, workflow.SwitchRequest{Branch: "other"})
	if err == nil {
		t.Fatal("Switch() error = nil, want git to refuse the switch")
	}

	if got := currentBranch(t, dir); got != "main" {
		t.Errorf("current branch = %q, want the switch to have been refused", got)
	}
	if status := statusOf(t, dir); len(status.Unstaged) != 1 {
		t.Errorf("Unstaged = %+v, want the local change to survive", status.Unstaged)
	}
}

func TestSwitchRejectsADetachedHead(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "--detach", "HEAD")

	_, err := newService(t, workflow.Options{}).Switch(context.Background(), dir, workflow.SwitchRequest{Branch: "main"})
	if !errors.Is(err, workflow.ErrDetachedHead) {
		t.Fatalf("Switch() error = %v, want ErrDetachedHead", err)
	}
}

func TestSwitchReportsAlreadyBeingOnTheBranch(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")

	result, err := newService(t, workflow.Options{}).Switch(context.Background(), dir, workflow.SwitchRequest{Branch: "main"})
	if err != nil {
		t.Fatalf("Switch() error = %v, want nil", err)
	}
	if !result.AlreadyOn {
		t.Error("AlreadyOn = false, want true")
	}
}

func TestBranchWorkflowsOutsideARepository(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	service := newService(t, workflow.Options{})

	if _, err := service.NewBranch(context.Background(), dir, workflow.NewBranchRequest{Ticket: "HCM-1"}); !errors.Is(err, workflow.ErrNotARepository) {
		t.Errorf("NewBranch() error = %v, want ErrNotARepository", err)
	}
	if _, err := service.Switch(context.Background(), dir, workflow.SwitchRequest{Branch: "main"}); !errors.Is(err, workflow.ErrNotARepository) {
		t.Errorf("Switch() error = %v, want ErrNotARepository", err)
	}
}

// newService builds a workflow service backed by the real git executable.
func newService(t *testing.T, opts workflow.Options) *workflow.Service {
	t.Helper()

	return workflow.NewService(git.NewClient(process.ExecRunner{}), opts)
}

// currentBranch reports the branch HEAD points at.
func currentBranch(t *testing.T, dir string) string {
	t.Helper()

	return strings.TrimSpace(runGit(t, dir, "branch", "--show-current"))
}

// initRepositoryOn creates a repository on the named branch with one commit.
func initRepositoryOn(t *testing.T, branch string, files ...string) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", branch)
	for _, name := range files {
		writeFile(t, dir, name, "initial\n")
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-qm", "initial")
	return dir
}

// initBareRepository creates an empty bare repository to act as a remote.
func initBareRepository(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "--bare", "-q")
	return dir
}

// startConflictingMerge leaves the repository paused in a conflicted merge.
func startConflictingMerge(t *testing.T, dir string) {
	t.Helper()

	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "README.md", "feature\n")
	runGit(t, dir, "commit", "-qam", "feature change")

	runGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "README.md", "main\n")
	runGit(t, dir, "commit", "-qam", "main change")

	if out, err := tryGit(dir, "merge", "feature"); err == nil {
		t.Fatalf("merge unexpectedly succeeded:\n%s", out)
	}
}

// startConflictingRebase leaves the repository paused in a conflicted rebase.
func startConflictingRebase(t *testing.T, dir string) {
	t.Helper()

	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "README.md", "feature\n")
	runGit(t, dir, "commit", "-qam", "feature change")

	runGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "README.md", "main\n")
	runGit(t, dir, "commit", "-qam", "main change")

	if out, err := tryGit(dir, "rebase", "feature"); err == nil {
		t.Fatalf("rebase unexpectedly succeeded:\n%s", out)
	}
}
