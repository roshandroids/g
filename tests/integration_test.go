// Package tests holds integration tests that exercise g against real Git
// repositories.
//
// Unlike the unit tests, these require the git executable. They are skipped in
// short mode (`go test -short ./...`) and when git is unavailable.
package tests

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
	"github.com/roshandroids/g/internal/process"
	"github.com/roshandroids/g/internal/workflow"
)

func TestStatusOnCleanRepository(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	status := statusOf(t, dir)

	if status.Branch != "main" {
		t.Errorf("Branch = %q, want %q", status.Branch, "main")
	}
	if status.Detached {
		t.Error("Detached = true, want false")
	}
	if !status.Clean() {
		t.Errorf("Clean() = false, want true: %+v", status)
	}
	if status.Upstream != nil {
		t.Errorf("Upstream = %+v, want nil for an untracked branch", status.Upstream)
	}
	if want := filepath.Base(resolved(t, dir)); status.Name != want {
		t.Errorf("Name = %q, want %q", status.Name, want)
	}
	if want := resolved(t, dir); status.Root != want {
		t.Errorf("Root = %q, want %q", status.Root, want)
	}
}

func TestStatusReportsUntrackedFiles(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	writeFile(t, dir, "notes.txt", "scratch\n")

	status := statusOf(t, dir)

	if status.Clean() {
		t.Error("Clean() = true, want false")
	}
	if want := []string{"notes.txt"}; !reflect.DeepEqual(status.Untracked, want) {
		t.Errorf("Untracked = %v, want %v", status.Untracked, want)
	}
	if len(status.Staged) != 0 || len(status.Unstaged) != 0 {
		t.Errorf("Staged/Unstaged = %v/%v, want both empty", status.Staged, status.Unstaged)
	}
}

func TestStatusSeparatesStagedFromUnstaged(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	writeFile(t, dir, "staged.txt", "new\n")
	runGit(t, dir, "add", "staged.txt")
	writeFile(t, dir, "README.md", "changed\n")

	status := statusOf(t, dir)

	wantStaged := []workflow.Change{{Path: "staged.txt", Kind: git.Added}}
	if !reflect.DeepEqual(status.Staged, wantStaged) {
		t.Errorf("Staged = %+v, want %+v", status.Staged, wantStaged)
	}

	wantUnstaged := []workflow.Change{{Path: "README.md", Kind: git.Modified}}
	if !reflect.DeepEqual(status.Unstaged, wantUnstaged) {
		t.Errorf("Unstaged = %+v, want %+v", status.Unstaged, wantUnstaged)
	}
}

func TestStatusReportsConflicts(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "README.md", "feature\n")
	runGit(t, dir, "commit", "-qam", "feature change")

	runGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "README.md", "main\n")
	runGit(t, dir, "commit", "-qam", "main change")

	if out, err := tryGit(dir, "merge", "feature"); err == nil {
		t.Fatalf("merge unexpectedly succeeded:\n%s", out)
	}

	status := statusOf(t, dir)

	if want := []string{"README.md"}; !reflect.DeepEqual(status.Conflicted, want) {
		t.Errorf("Conflicted = %v, want %v", status.Conflicted, want)
	}
	if status.Clean() {
		t.Error("Clean() = true, want false")
	}
}

func TestStatusReportsDetachedHead(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "checkout", "-q", "--detach", "HEAD")

	status := statusOf(t, dir)

	if !status.Detached {
		t.Error("Detached = false, want true")
	}
	if status.Branch != "" {
		t.Errorf("Branch = %q, want empty for a detached HEAD", status.Branch)
	}
	if len(status.Head) != 7 {
		t.Errorf("Head = %q, want a seven character abbreviated commit", status.Head)
	}
	if status.Upstream != nil {
		t.Errorf("Upstream = %+v, want nil for a detached HEAD", status.Upstream)
	}
}

func TestStatusTracksUpstreamAndDivergence(t *testing.T) {
	requireGit(t)

	upstream := filepath.Join(t.TempDir(), "upstream.git")
	runGit(t, t.TempDir(), "init", "--bare", "-q", upstream)

	dir := initRepository(t, "README.md")
	runGit(t, dir, "remote", "add", "origin", upstream)
	runGit(t, dir, "push", "-q", "-u", "origin", "main")

	status := statusOf(t, dir)
	if status.Upstream == nil {
		t.Fatal("Upstream = nil, want origin/main")
	}
	if status.Upstream.Name != "origin/main" {
		t.Errorf("Upstream.Name = %q, want %q", status.Upstream.Name, "origin/main")
	}
	if status.Upstream.Ahead != 0 || status.Upstream.Behind != 0 {
		t.Errorf("Ahead/Behind = %d/%d, want 0/0", status.Upstream.Ahead, status.Upstream.Behind)
	}

	writeFile(t, dir, "second.txt", "second\n")
	runGit(t, dir, "add", "second.txt")
	runGit(t, dir, "commit", "-qm", "second")
	runGit(t, dir, "push", "-q", "origin", "main")

	// Rewinding locally leaves the upstream holding a commit HEAD lacks, which
	// is the only way to be behind without a second clone.
	runGit(t, dir, "reset", "-q", "--hard", "HEAD~1")

	status = statusOf(t, dir)
	if status.Upstream == nil {
		t.Fatal("Upstream = nil, want origin/main")
	}
	if status.Upstream.Ahead != 0 || status.Upstream.Behind != 1 {
		t.Errorf("Ahead/Behind = %d/%d, want 0/1", status.Upstream.Ahead, status.Upstream.Behind)
	}
}

func TestStatusOutsideRepository(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()

	_, err := workflow.NewService(git.NewClient(process.ExecRunner{})).Status(context.Background(), dir)
	if !errors.Is(err, workflow.ErrNotARepository) {
		t.Fatalf("Status() error = %v, want workflow.ErrNotARepository", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("Status() error = %v, want it to name %q", err, dir)
	}
}

// statusOf reports the state of dir, failing the test on any error.
func statusOf(t *testing.T, dir string) workflow.Status {
	t.Helper()

	status, err := workflow.NewService(git.NewClient(process.ExecRunner{})).Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("Status(%q) error = %v, want nil", dir, err)
	}
	return status
}

// requireGit skips the test when git cannot be executed.
func requireGit(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// initRepository creates a repository on main with one commit containing the
// named files.
func initRepository(t *testing.T, files ...string) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	for _, name := range files {
		writeFile(t, dir, name, "initial\n")
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-qm", "initial")
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// runGit runs git in dir and fails the test if it does not succeed.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := tryGit(dir, args...)
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return out
}

// tryGit runs git in dir and returns its combined output.
//
// The author and committer are set explicitly so the tests do not depend on the
// machine's Git identity.
func tryGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=g test",
		"GIT_AUTHOR_EMAIL=g@example.com",
		"GIT_COMMITTER_NAME=g test",
		"GIT_COMMITTER_EMAIL=g@example.com",
	)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// resolved follows symlinks so that temporary directories compare equal on
// platforms where /tmp is a symlink.
func resolved(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolving %q: %v", path, err)
	}
	return resolved
}
