package tests

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/workflow"
)

// pushFixture is a repository with a bare remote, so pushing is exercised
// against real git rather than a scripted runner.
type pushFixture struct {
	dir      string
	upstream string
}

func newPushFixture(t *testing.T) pushFixture {
	t.Helper()

	upstream := initBareRepository(t)
	dir := initRepository(t, "README.md")
	runGit(t, dir, "remote", "add", "origin", upstream)

	return pushFixture{dir: dir, upstream: upstream}
}

func TestPushPublishesABranchAndSetsTheUpstream(t *testing.T) {
	requireGit(t)

	fixture := newPushFixture(t)

	result, err := newService(t, workflow.Options{}).Push(context.Background(), fixture.dir, workflow.PushRequest{})
	if err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}

	if !result.SetUpstream {
		t.Error("SetUpstream = false, want true")
	}
	if want := "origin/main"; result.Upstream != want {
		t.Errorf("Upstream = %q, want %q", result.Upstream, want)
	}

	if branches := remoteBranches(t, fixture.upstream); len(branches) != 1 || branches[0] != "main" {
		t.Errorf("remote branches = %v, want [main]", branches)
	}

	// git has to have recorded the upstream, not just accepted the push.
	if got := strings.TrimSpace(runGit(t, fixture.dir, "rev-parse", "--abbrev-ref", "main@{upstream}")); got != "origin/main" {
		t.Errorf("upstream = %q, want origin/main to be tracked", got)
	}
}

func TestPushToAnExistingUpstream(t *testing.T) {
	requireGit(t)

	fixture := newPushFixture(t)
	runGit(t, fixture.dir, "push", "-q", "-u", "origin", "main")

	writeFile(t, fixture.dir, "second.txt", "second\n")
	runGit(t, fixture.dir, "add", "second.txt")
	runGit(t, fixture.dir, "commit", "-qm", "second")

	result, err := newService(t, workflow.Options{}).Push(context.Background(), fixture.dir, workflow.PushRequest{})
	if err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}

	if result.SetUpstream {
		t.Error("SetUpstream = true, want false when the upstream already existed")
	}
	if result.Forced {
		t.Error("Forced = true, want false for an ordinary push")
	}

	// The commit really has to be on the remote, not merely reported as sent.
	if got := strings.TrimSpace(runGit(t, fixture.upstream, "log", "-1", "--format=%s", "main")); got != "second" {
		t.Errorf("remote HEAD = %q, want the new commit to have arrived", got)
	}
}

func TestPushWithoutARemote(t *testing.T) {
	requireGit(t)

	dir := initRepository(t, "README.md")

	_, err := newService(t, workflow.Options{}).Push(context.Background(), dir, workflow.PushRequest{})
	if !errors.Is(err, workflow.ErrNoRemote) {
		t.Fatalf("Push() error = %v, want ErrNoRemote", err)
	}
}

func TestPushRejectsADetachedHead(t *testing.T) {
	requireGit(t)

	fixture := newPushFixture(t)
	runGit(t, fixture.dir, "checkout", "-q", "--detach", "HEAD")

	_, err := newService(t, workflow.Options{}).Push(context.Background(), fixture.dir, workflow.PushRequest{})
	if !errors.Is(err, workflow.ErrDetachedHead) {
		t.Fatalf("Push() error = %v, want ErrDetachedHead", err)
	}
}

func TestPushRejectsARebaseInProgress(t *testing.T) {
	requireGit(t)

	fixture := newPushFixture(t)
	startConflictingRebase(t, fixture.dir)

	_, err := newService(t, workflow.Options{}).Push(context.Background(), fixture.dir, workflow.PushRequest{})
	if !errors.Is(err, workflow.ErrOperationInProgress) {
		t.Fatalf("Push() error = %v, want ErrOperationInProgress", err)
	}
}

// A force push has to refuse when the remote has moved since the last fetch.
// That refusal is the whole reason --force-with-lease is used instead of
// --force.
func TestPushForceWithLeaseRefusesAStaleRemote(t *testing.T) {
	requireGit(t)

	fixture := newPushFixture(t)
	runGit(t, fixture.dir, "push", "-q", "-u", "origin", "main")

	// Somebody else pushes to the same branch.
	other := initRepository(t, "README.md")
	runGit(t, other, "remote", "add", "origin", fixture.upstream)
	runGit(t, other, "fetch", "-q", "origin")
	runGit(t, other, "checkout", "-q", "main")
	runGit(t, other, "reset", "-q", "--hard", "origin/main")
	writeFile(t, other, "theirs.txt", "theirs\n")
	runGit(t, other, "add", "theirs.txt")
	runGit(t, other, "commit", "-qm", "somebody else's work")
	runGit(t, other, "push", "-q", "origin", "main")

	// The local branch moves on without fetching their work, so the two
	// histories diverge and the remote-tracking ref is stale.
	writeFile(t, fixture.dir, "mine.txt", "mine\n")
	runGit(t, fixture.dir, "add", "mine.txt")
	runGit(t, fixture.dir, "commit", "-qm", "mine")

	_, err := newService(t, workflow.Options{}).Push(context.Background(), fixture.dir, workflow.PushRequest{Force: true})
	if err == nil {
		t.Fatal("Push() error = nil, want the stale lease to be refused")
	}

	// The other person's commit must still be there.
	if got := strings.TrimSpace(runGit(t, fixture.upstream, "log", "--format=%s", "main")); !strings.Contains(got, "somebody else's work") {
		t.Errorf("remote log = %q, want the other commit to survive", got)
	}
}

// With the remote unchanged since the last fetch, the lease is satisfied and
// the rewrite goes through.
func TestPushForceWithLeaseSucceedsWhenTheRemoteHasNotMoved(t *testing.T) {
	requireGit(t)

	fixture := newPushFixture(t)
	runGit(t, fixture.dir, "push", "-q", "-u", "origin", "main")

	writeFile(t, fixture.dir, "second.txt", "second\n")
	runGit(t, fixture.dir, "add", "second.txt")
	runGit(t, fixture.dir, "commit", "-qm", "second")
	runGit(t, fixture.dir, "push", "-q")

	// Replace the pushed commit with a different one.
	runGit(t, fixture.dir, "reset", "-q", "--hard", "HEAD~1")
	writeFile(t, fixture.dir, "replacement.txt", "replacement\n")
	runGit(t, fixture.dir, "add", "replacement.txt")
	runGit(t, fixture.dir, "commit", "-qm", "replacement")

	result, err := newService(t, workflow.Options{}).Push(context.Background(), fixture.dir, workflow.PushRequest{Force: true})
	if err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}
	if !result.Forced {
		t.Error("Forced = false, want true")
	}

	if got := strings.TrimSpace(runGit(t, fixture.upstream, "log", "-1", "--format=%s", "main")); got != "replacement" {
		t.Errorf("remote HEAD = %q, want the rewritten commit", got)
	}
}

func TestPushForceWithoutAnUpstream(t *testing.T) {
	requireGit(t)

	fixture := newPushFixture(t)

	_, err := newService(t, workflow.Options{}).Push(context.Background(), fixture.dir, workflow.PushRequest{Force: true})
	if !errors.Is(err, workflow.ErrNoUpstream) {
		t.Fatalf("Push() error = %v, want ErrNoUpstream", err)
	}
	if branches := remoteBranches(t, fixture.upstream); len(branches) != 0 {
		t.Errorf("remote branches = %v, want nothing pushed", branches)
	}
}

// A rejected push is reported, not worked around: when the remote has moved on,
// fetching and integrating is the answer, and that is the user's call.
func TestPushReportsARejectedPush(t *testing.T) {
	requireGit(t)

	fixture := newPushFixture(t)
	runGit(t, fixture.dir, "push", "-q", "-u", "origin", "main")

	other := initRepository(t, "README.md")
	runGit(t, other, "remote", "add", "origin", fixture.upstream)
	runGit(t, other, "fetch", "-q", "origin")
	runGit(t, other, "checkout", "-q", "main")
	runGit(t, other, "reset", "-q", "--hard", "origin/main")
	writeFile(t, other, "theirs.txt", "theirs\n")
	runGit(t, other, "add", "theirs.txt")
	runGit(t, other, "commit", "-qm", "their work")
	runGit(t, other, "push", "-q", "origin", "main")

	// A local commit on the original history diverges from theirs, so the push
	// is not a fast-forward.
	writeFile(t, fixture.dir, "mine.txt", "mine\n")
	runGit(t, fixture.dir, "add", "mine.txt")
	runGit(t, fixture.dir, "commit", "-qm", "my work")

	_, err := newService(t, workflow.Options{}).Push(context.Background(), fixture.dir, workflow.PushRequest{})
	if err == nil {
		t.Fatal("Push() error = nil, want the push to be rejected")
	}
	if !strings.Contains(err.Error(), "rejected") && !strings.Contains(err.Error(), "non-fast-forward") {
		t.Errorf("Push() error = %v, want it to keep git's explanation", err)
	}
}

func TestPushOutsideARepository(t *testing.T) {
	requireGit(t)

	_, err := newService(t, workflow.Options{}).Push(context.Background(), t.TempDir(), workflow.PushRequest{})
	if !errors.Is(err, workflow.ErrNotARepository) {
		t.Fatalf("Push() error = %v, want ErrNotARepository", err)
	}
}

// remoteBranches lists the branches a bare repository holds.
func remoteBranches(t *testing.T, bare string) []string {
	t.Helper()

	out := strings.TrimSpace(runGit(t, bare, "for-each-ref", "--format=%(refname:short)", "refs/heads"))
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}
