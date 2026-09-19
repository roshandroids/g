package tests

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/workflow"
)

func TestSyncCleanFeatureBranch(t *testing.T) {
	requireGit(t)

	fixture := newSyncFixture(t)
	advanceRemoteMain(t, fixture)

	runGit(t, fixture.dir, "checkout", "-q", "-b", "feature")
	writeFile(t, fixture.dir, "feature.txt", "feature\n")
	runGit(t, fixture.dir, "add", "feature.txt")
	runGit(t, fixture.dir, "commit", "-qm", "feature work")

	result, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir)
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}

	if result.Branch != "feature" || result.Base != "main" {
		t.Errorf("Branch/Base = %q/%q, want feature/main", result.Branch, result.Base)
	}
	if !result.BaseUpdated || !result.Rebased || !result.HistoryRewritten {
		t.Errorf("BaseUpdated/Rebased/HistoryRewritten = %v/%v/%v, want true/true/true",
			result.BaseUpdated, result.Rebased, result.HistoryRewritten)
	}
	if result.Behind != 0 {
		t.Errorf("Behind = %d, want 0 after rebase", result.Behind)
	}
	if currentBranch(t, fixture.dir) != "feature" {
		t.Errorf("current branch = %q, want feature", currentBranch(t, fixture.dir))
	}
	// Feature commit must sit on top of the remote main tip.
	if !isAncestor(t, fixture.dir, "origin/main", "HEAD") {
		t.Error("feature HEAD must contain origin/main after sync")
	}
}

func TestSyncOnBaseBranch(t *testing.T) {
	requireGit(t)

	fixture := newSyncFixture(t)
	advanceRemoteMain(t, fixture)

	result, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir)
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if !result.OnBase || !result.BaseUpdated {
		t.Errorf("OnBase/BaseUpdated = %v/%v, want true/true", result.OnBase, result.BaseUpdated)
	}
	if result.Rebased {
		t.Error("Rebased = true, want false on the base branch")
	}
	if got := strings.TrimSpace(runGit(t, fixture.dir, "rev-parse", "main")); got != strings.TrimSpace(runGit(t, fixture.dir, "rev-parse", "origin/main")) {
		t.Error("main must match origin/main after sync")
	}
}

func TestSyncWithUnstagedChanges(t *testing.T) {
	requireGit(t)
	assertSyncPreservesDirtyTree(t, func(dir string) {
		writeFile(t, dir, "README.md", "local edit\n")
	}, "README.md", "local edit\n")
}

func TestSyncWithStagedChanges(t *testing.T) {
	requireGit(t)
	assertSyncPreservesDirtyTree(t, func(dir string) {
		writeFile(t, dir, "staged.txt", "staged\n")
		runGit(t, dir, "add", "staged.txt")
	}, "staged.txt", "staged\n")
}

func TestSyncWithUntrackedFiles(t *testing.T) {
	requireGit(t)
	assertSyncPreservesDirtyTree(t, func(dir string) {
		writeFile(t, dir, "scratch.txt", "scratch\n")
	}, "scratch.txt", "scratch\n")
}

func TestSyncWithMixedChanges(t *testing.T) {
	requireGit(t)

	fixture := newSyncFixture(t)
	advanceRemoteMain(t, fixture)

	runGit(t, fixture.dir, "checkout", "-q", "-b", "feature")
	writeFile(t, fixture.dir, "feature.txt", "feature\n")
	runGit(t, fixture.dir, "add", "feature.txt")
	runGit(t, fixture.dir, "commit", "-qm", "feature work")

	writeFile(t, fixture.dir, "README.md", "unstaged\n")
	writeFile(t, fixture.dir, "staged.txt", "staged\n")
	runGit(t, fixture.dir, "add", "staged.txt")
	writeFile(t, fixture.dir, "scratch.txt", "scratch\n")

	result, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir)
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if !result.Preserved || !result.Restored {
		t.Errorf("Preserved/Restored = %v/%v, want true/true", result.Preserved, result.Restored)
	}

	assertFileContent(t, fixture.dir, "README.md", "unstaged\n")
	assertFileContent(t, fixture.dir, "staged.txt", "staged\n")
	assertFileContent(t, fixture.dir, "scratch.txt", "scratch\n")
	if stashCount(t, fixture.dir) != 0 {
		t.Errorf("stash entries = %d, want 0 after successful restore", stashCount(t, fixture.dir))
	}
}

func TestSyncMasterBase(t *testing.T) {
	requireGit(t)

	upstream := initBareRepository(t)
	dir := initRepositoryOn(t, "master", "README.md")
	runGit(t, dir, "remote", "add", "origin", upstream)
	runGit(t, dir, "push", "-q", "-u", "origin", "master")
	runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/master")

	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "feature.txt", "feature\n")
	runGit(t, dir, "add", "feature.txt")
	runGit(t, dir, "commit", "-qm", "feature")

	// Advance remote master from another clone.
	other := t.TempDir()
	runGit(t, other, "clone", "-q", upstream, other)
	writeFile(t, other, "remote.txt", "remote\n")
	runGit(t, other, "add", "remote.txt")
	runGit(t, other, "commit", "-qm", "remote")
	runGit(t, other, "push", "-q", "origin", "master")

	result, err := newService(t, workflow.Options{DefaultBaseBranch: "master"}).Sync(context.Background(), dir)
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if result.Base != "master" {
		t.Errorf("Base = %q, want master", result.Base)
	}
	if !result.Rebased {
		t.Error("Rebased = false, want true")
	}
}

func TestSyncConfiguredBase(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "develop")
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "feature.txt", "feature\n")
	runGit(t, dir, "add", "feature.txt")
	runGit(t, dir, "commit", "-qm", "feature")

	runGit(t, dir, "checkout", "-q", "develop")
	writeFile(t, dir, "develop.txt", "develop\n")
	runGit(t, dir, "add", "develop.txt")
	runGit(t, dir, "commit", "-qm", "develop ahead")
	runGit(t, dir, "checkout", "-q", "feature")

	result, err := newService(t, workflow.Options{DefaultBaseBranch: "develop"}).Sync(context.Background(), dir)
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if result.Base != "develop" {
		t.Errorf("Base = %q, want develop", result.Base)
	}
	if !isAncestor(t, dir, "develop", "HEAD") {
		t.Error("feature must contain develop after sync")
	}
}

func TestSyncDetachedHead(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "--detach", "HEAD")

	_, err := newService(t, workflow.Options{}).Sync(context.Background(), dir)
	if !errors.Is(err, workflow.ErrDetachedHead) {
		t.Fatalf("Sync() error = %v, want ErrDetachedHead", err)
	}
}

func TestSyncRejectsExistingRebase(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	startConflictingRebase(t, dir)

	_, err := newService(t, workflow.Options{}).Sync(context.Background(), dir)
	if !errors.Is(err, workflow.ErrOperationInProgress) {
		t.Fatalf("Sync() error = %v, want ErrOperationInProgress", err)
	}
}

func TestSyncRejectsExistingMerge(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	startConflictingMerge(t, dir)

	_, err := newService(t, workflow.Options{}).Sync(context.Background(), dir)
	if !errors.Is(err, workflow.ErrOperationInProgress) {
		t.Fatalf("Sync() error = %v, want ErrOperationInProgress", err)
	}
}

func TestSyncRejectsExistingCherryPick(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "README.md", "feature\n")
	runGit(t, dir, "commit", "-qam", "feature change")
	runGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "README.md", "main\n")
	runGit(t, dir, "commit", "-qam", "main change")
	if out, err := tryGit(dir, "cherry-pick", "feature"); err == nil {
		t.Fatalf("cherry-pick unexpectedly succeeded:\n%s", out)
	}

	_, err := newService(t, workflow.Options{}).Sync(context.Background(), dir)
	if !errors.Is(err, workflow.ErrOperationInProgress) {
		t.Fatalf("Sync() error = %v, want ErrOperationInProgress", err)
	}
}

func TestSyncWithoutRemote(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "feature.txt", "feature\n")
	runGit(t, dir, "add", "feature.txt")
	runGit(t, dir, "commit", "-qm", "feature")

	runGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "main.txt", "main\n")
	runGit(t, dir, "add", "main.txt")
	runGit(t, dir, "commit", "-qm", "main ahead")
	runGit(t, dir, "checkout", "-q", "feature")

	result, err := newService(t, workflow.Options{}).Sync(context.Background(), dir)
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if result.Fetched {
		t.Error("Fetched = true, want false")
	}
	if !result.Rebased {
		t.Error("Rebased = false, want true against local main")
	}
}

func TestSyncRemoteUnavailable(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	runGit(t, dir, "checkout", "-q", "-b", "feature")

	_, err := newService(t, workflow.Options{}).Sync(context.Background(), dir)
	if err == nil {
		t.Fatal("Sync() error = nil, want fetch failure")
	}
	if !strings.Contains(err.Error(), "fetching origin") {
		t.Errorf("Sync() error = %v, want it to name the fetch failure", err)
	}
}

func TestSyncLocalBaseAheadOfRemote(t *testing.T) {
	requireGit(t)

	fixture := newSyncFixture(t)
	writeFile(t, fixture.dir, "local.txt", "local\n")
	runGit(t, fixture.dir, "add", "local.txt")
	runGit(t, fixture.dir, "commit", "-qm", "local only")

	runGit(t, fixture.dir, "checkout", "-q", "-b", "feature")
	writeFile(t, fixture.dir, "feature.txt", "feature\n")
	runGit(t, fixture.dir, "add", "feature.txt")
	runGit(t, fixture.dir, "commit", "-qm", "feature")

	result, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir)
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if result.BaseUpdated {
		t.Error("BaseUpdated = true, want false when local base is ahead")
	}
}

func TestSyncDivergentBase(t *testing.T) {
	requireGit(t)

	fixture := newSyncFixture(t)

	// Local main diverges from origin/main.
	writeFile(t, fixture.dir, "local.txt", "local\n")
	runGit(t, fixture.dir, "add", "local.txt")
	runGit(t, fixture.dir, "commit", "-qm", "local")
	advanceRemoteMain(t, fixture)

	runGit(t, fixture.dir, "checkout", "-q", "-b", "feature")

	_, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir)
	if !errors.Is(err, workflow.ErrDivergentBase) {
		t.Fatalf("Sync() error = %v, want ErrDivergentBase", err)
	}
}

func TestSyncRebaseConflict(t *testing.T) {
	requireGit(t)

	fixture := newSyncFixture(t)

	runGit(t, fixture.dir, "checkout", "-q", "-b", "feature")
	writeFile(t, fixture.dir, "README.md", "feature\n")
	runGit(t, fixture.dir, "commit", "-qam", "feature change")

	// Remote/main changes the same file.
	other := cloneRemote(t, fixture.upstream)
	writeFile(t, other, "README.md", "remote\n")
	runGit(t, other, "commit", "-qam", "remote change")
	runGit(t, other, "push", "-q", "origin", "main")

	_, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir)
	if err == nil {
		t.Fatal("Sync() error = nil, want rebase conflict")
	}
	if !strings.Contains(err.Error(), "Rebase stopped because of conflicts") {
		t.Errorf("Sync() error = %v, want conflict instructions", err)
	}

	status := statusOf(t, fixture.dir)
	if status.Operation != git.OperationRebase {
		t.Errorf("Operation = %q, want rebase left in progress", status.Operation)
	}
}

func TestSyncStashRestoreConflict(t *testing.T) {
	requireGit(t)

	fixture := newSyncFixture(t)

	runGit(t, fixture.dir, "checkout", "-q", "-b", "feature")
	writeFile(t, fixture.dir, "feature.txt", "feature\n")
	runGit(t, fixture.dir, "add", "feature.txt")
	runGit(t, fixture.dir, "commit", "-qm", "feature")

	// Remote rewrites README; local dirty edit of the same file will not apply cleanly
	// after the base catches up.
	other := cloneRemote(t, fixture.upstream)
	writeFile(t, other, "README.md", "from remote\n")
	runGit(t, other, "commit", "-qam", "remote readme")
	runGit(t, other, "push", "-q", "origin", "main")

	writeFile(t, fixture.dir, "README.md", "local dirty\n")

	_, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir)
	if err == nil {
		t.Fatal("Sync() error = nil, want stash restore conflict")
	}

	var restoreErr *git.StashRestoreError
	if !errors.As(err, &restoreErr) {
		t.Fatalf("Sync() error = %v, want StashRestoreError", err)
	}
	if stashCount(t, fixture.dir) == 0 {
		t.Fatal("stash entry must be kept when restoration conflicts")
	}

	stashed := runGit(t, fixture.dir, "stash", "show", "-p")
	if !strings.Contains(stashed, "local dirty") {
		t.Errorf("stash patch missing local dirty content:\n%s", stashed)
	}

	// Rebase must have finished; only the restore is conflicted.
	status := statusOf(t, fixture.dir)
	if status.Operation != git.OperationNone {
		t.Errorf("Operation = %q, want none after a finished rebase", status.Operation)
	}
}

func TestSyncDoesNotPush(t *testing.T) {
	requireGit(t)

	fixture := newSyncFixture(t)
	runGit(t, fixture.dir, "checkout", "-q", "-b", "feature")
	writeFile(t, fixture.dir, "feature.txt", "feature\n")
	runGit(t, fixture.dir, "add", "feature.txt")
	runGit(t, fixture.dir, "commit", "-qm", "feature")
	runGit(t, fixture.dir, "push", "-q", "-u", "origin", "feature")

	before := strings.TrimSpace(runGit(t, fixture.upstream, "rev-parse", "feature"))

	advanceRemoteMain(t, fixture)
	if _, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir); err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}

	after := strings.TrimSpace(runGit(t, fixture.upstream, "rev-parse", "feature"))
	if before != after {
		t.Error("Sync must not push the feature branch")
	}
}

// syncFixture is a repo on main tracking a bare origin.
type syncFixture struct {
	dir      string
	upstream string
}

func newSyncFixture(t *testing.T) syncFixture {
	t.Helper()

	upstream := initBareRepository(t)
	dir := initRepository(t, "README.md")
	runGit(t, dir, "remote", "add", "origin", upstream)
	runGit(t, dir, "push", "-q", "-u", "origin", "main")
	runGit(t, upstream, "symbolic-ref", "HEAD", "refs/heads/main")
	runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	return syncFixture{dir: dir, upstream: upstream}
}

func advanceRemoteMain(t *testing.T, fixture syncFixture) {
	t.Helper()

	other := cloneRemote(t, fixture.upstream)
	runGit(t, other, "checkout", "-q", "main")
	writeFile(t, other, "remote.txt", "from remote\n")
	runGit(t, other, "add", "remote.txt")
	runGit(t, other, "commit", "-qm", "remote advance")
	runGit(t, other, "push", "-q", "origin", "main")
}

func cloneRemote(t *testing.T, upstream string) string {
	t.Helper()

	parent := t.TempDir()
	dir := filepath.Join(parent, "clone")
	runGit(t, parent, "clone", "-q", upstream, dir)
	// Bare fixtures may predate HEAD→main; force the branch we pushed.
	if branch := strings.TrimSpace(runGit(t, dir, "branch", "--show-current")); branch != "main" {
		runGit(t, dir, "checkout", "-q", "-B", "main", "origin/main")
	}
	return dir
}

func assertSyncPreservesDirtyTree(t *testing.T, dirty func(string), path, content string) {
	t.Helper()

	fixture := newSyncFixture(t)
	advanceRemoteMain(t, fixture)

	runGit(t, fixture.dir, "checkout", "-q", "-b", "feature")
	writeFile(t, fixture.dir, "feature.txt", "feature\n")
	runGit(t, fixture.dir, "add", "feature.txt")
	runGit(t, fixture.dir, "commit", "-qm", "feature work")

	dirty(fixture.dir)

	result, err := newService(t, workflow.Options{}).Sync(context.Background(), fixture.dir)
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil", err)
	}
	if !result.Preserved || !result.Restored {
		t.Errorf("Preserved/Restored = %v/%v, want true/true", result.Preserved, result.Restored)
	}
	assertFileContent(t, fixture.dir, path, content)
	if stashCount(t, fixture.dir) != 0 {
		t.Errorf("stash entries = %d, want 0", stashCount(t, fixture.dir))
	}
}

func assertFileContent(t *testing.T, dir, name, want string) {
	t.Helper()

	got := fileContent(t, dir, name)
	if got != want {
		t.Errorf("%s content = %q, want %q", name, got, want)
	}
}

func fileContent(t *testing.T, dir, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(data)
}

func stashCount(t *testing.T, dir string) int {
	t.Helper()

	out := strings.TrimSpace(runGit(t, dir, "stash", "list"))
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func isAncestor(t *testing.T, dir, ancestor, rev string) bool {
	t.Helper()

	_, err := tryGit(dir, "merge-base", "--is-ancestor", ancestor, rev)
	return err == nil
}
