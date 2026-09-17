package workflow

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
)

// trackedRepository is on a branch that already has an upstream.
func trackedRepository() *fakeRepository {
	return &fakeRepository{
		status: git.Status{
			Root:     branchTestDir,
			Branch:   "HCM-1-work",
			Upstream: "origin/HCM-1-work",
		},
		remotes: []string{"origin"},
	}
}

func TestServicePushToAnExistingUpstream(t *testing.T) {
	repo := trackedRepository()

	result, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{})
	if err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}

	if result.Branch != "HCM-1-work" {
		t.Errorf("Branch = %q, want %q", result.Branch, "HCM-1-work")
	}
	if result.Upstream != "origin/HCM-1-work" {
		t.Errorf("Upstream = %q, want %q", result.Upstream, "origin/HCM-1-work")
	}
	if result.SetUpstream {
		t.Error("SetUpstream = true, want false when the upstream already existed")
	}

	// Nothing is named on the command line: git already knows the upstream, so
	// repeating it could only get it wrong.
	want := []git.PushOptions{{}}
	if !reflect.DeepEqual(repo.pushCalls, want) {
		t.Errorf("Push calls = %+v, want %+v", repo.pushCalls, want)
	}
}

// A branch that tracks nothing is not pushed on a guess: the caller has to
// agree to publishing a new branch first.
func TestServicePushWithoutAnUpstreamAsksFirst(t *testing.T) {
	repo := &fakeRepository{
		status:  git.Status{Root: branchTestDir, Branch: "HCM-1-work"},
		remotes: []string{"origin"},
	}

	_, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{})
	if !errors.Is(err, ErrNoUpstream) {
		t.Fatalf("Push() error = %v, want ErrNoUpstream", err)
	}
	if !strings.Contains(err.Error(), "HCM-1-work") {
		t.Errorf("Push() error = %v, want it to name the branch", err)
	}
	if len(repo.pushCalls) != 0 {
		t.Errorf("Push calls = %+v, want nothing pushed before the user agreed", repo.pushCalls)
	}
}

func TestServicePushCreatingAnUpstream(t *testing.T) {
	repo := &fakeRepository{
		status:  git.Status{Root: branchTestDir, Branch: "HCM-1-work"},
		remotes: []string{"origin"},
	}

	result, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{CreateUpstream: true})
	if err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}

	if !result.SetUpstream {
		t.Error("SetUpstream = false, want true")
	}
	if result.Upstream != "origin/HCM-1-work" {
		t.Errorf("Upstream = %q, want %q", result.Upstream, "origin/HCM-1-work")
	}

	want := []git.PushOptions{{Remote: "origin", Ref: "HEAD", SetUpstream: true}}
	if !reflect.DeepEqual(repo.pushCalls, want) {
		t.Errorf("Push calls = %+v, want %+v", repo.pushCalls, want)
	}
}

func TestServicePushChoosesTheRemote(t *testing.T) {
	tests := []struct {
		name    string
		remotes []string
		want    string
		wantErr bool
	}{
		{name: "origin wins over others", remotes: []string{"upstream", "origin"}, want: "origin"},
		{name: "a sole remote is unambiguous", remotes: []string{"github"}, want: "github"},
		{name: "origin alone", remotes: []string{"origin"}, want: "origin"},
		{name: "several without origin is ambiguous", remotes: []string{"a", "b"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeRepository{
				status:  git.Status{Root: branchTestDir, Branch: "work"},
				remotes: test.remotes,
			}

			result, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{CreateUpstream: true})
			if test.wantErr {
				if !errors.Is(err, ErrNoRemote) {
					t.Fatalf("Push() error = %v, want ErrNoRemote for an ambiguous choice", err)
				}
				if len(repo.pushCalls) != 0 {
					t.Errorf("Push calls = %+v, want none", repo.pushCalls)
				}
				return
			}

			if err != nil {
				t.Fatalf("Push() error = %v, want nil", err)
			}
			if result.Upstream != test.want+"/work" {
				t.Errorf("Upstream = %q, want %q", result.Upstream, test.want+"/work")
			}
		})
	}
}

func TestServicePushWithoutARemote(t *testing.T) {
	repo := &fakeRepository{
		status:  git.Status{Root: branchTestDir, Branch: "work", Upstream: "origin/work"},
		remotes: nil,
	}

	_, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{})
	if !errors.Is(err, ErrNoRemote) {
		t.Fatalf("Push() error = %v, want ErrNoRemote", err)
	}
	if !strings.Contains(err.Error(), "git remote add") {
		t.Errorf("Push() error = %v, want it to say how to add a remote", err)
	}
	if len(repo.pushCalls) != 0 {
		t.Errorf("Push calls = %+v, want none", repo.pushCalls)
	}
}

// A plain --force overwrites commits the user has not seen. The strongest thing
// g will do is --force-with-lease, and only when it was asked to.
func TestServicePushForceUsesForceWithLease(t *testing.T) {
	normal := trackedRepository()

	if _, err := NewService(normal, Options{}).Push(context.Background(), branchTestDir, PushRequest{}); err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}
	if normal.pushCalls[0].ForceWithLease {
		t.Error("a normal push used --force-with-lease, want a plain push")
	}

	forced := trackedRepository()

	result, err := NewService(forced, Options{}).Push(context.Background(), branchTestDir, PushRequest{Force: true})
	if err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}
	if !result.Forced {
		t.Error("Forced = false, want true")
	}
	if !forced.pushCalls[0].ForceWithLease {
		t.Error("a forced push did not use --force-with-lease")
	}
}

// There is no remote branch to replace, so a force push could only be a plain
// overwrite of whatever is already there.
func TestServicePushForceWithoutAnUpstream(t *testing.T) {
	repo := &fakeRepository{
		status:  git.Status{Root: branchTestDir, Branch: "work"},
		remotes: []string{"origin"},
	}

	_, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{Force: true, CreateUpstream: true})
	if !errors.Is(err, ErrNoUpstream) {
		t.Fatalf("Push() error = %v, want ErrNoUpstream", err)
	}
	if len(repo.pushCalls) != 0 {
		t.Errorf("Push calls = %+v, want none", repo.pushCalls)
	}
}

func TestServicePushRejectsADetachedHead(t *testing.T) {
	repo := &fakeRepository{
		status:  git.Status{Root: branchTestDir, Detached: true, Head: "a1b2c3d"},
		remotes: []string{"origin"},
	}

	_, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{})
	if !errors.Is(err, ErrDetachedHead) {
		t.Fatalf("Push() error = %v, want ErrDetachedHead", err)
	}
	if len(repo.pushCalls) != 0 {
		t.Errorf("Push calls = %+v, want none", repo.pushCalls)
	}
}

func TestServicePushRejectsAnOperationInProgress(t *testing.T) {
	repo := trackedRepository()
	repo.status.Operation = git.OperationRebase

	_, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{})
	if !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("Push() error = %v, want ErrOperationInProgress", err)
	}
}

func TestServicePushPropagatesFailures(t *testing.T) {
	tests := []struct {
		name string
		repo *fakeRepository
	}{
		{
			name: "listing remotes",
			repo: &fakeRepository{
				status:     git.Status{Root: branchTestDir, Branch: "work", Upstream: "origin/work"},
				remotesErr: errors.New("remote exploded"),
			},
		},
		{
			name: "the push is rejected",
			repo: &fakeRepository{
				status:  git.Status{Root: branchTestDir, Branch: "work", Upstream: "origin/work"},
				remotes: []string{"origin"},
				pushErr: errors.New("! [rejected] non-fast-forward"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(test.repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{})
			if err == nil {
				t.Fatal("Push() error = nil, want the underlying failure")
			}
			if !strings.Contains(err.Error(), "exploded") && !strings.Contains(err.Error(), "rejected") {
				t.Errorf("Push() error = %v, want it to keep the cause", err)
			}
		})
	}
}

func TestServicePushOutsideARepository(t *testing.T) {
	repo := &fakeRepository{statusErr: git.ErrNotARepository}

	_, err := NewService(repo, Options{}).Push(context.Background(), branchTestDir, PushRequest{})
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("Push() error = %v, want ErrNotARepository", err)
	}
}

// Nil and out-of-range inputs must not panic; the defaults are the safe ones.
func TestChooseRemoteEdgeCases(t *testing.T) {
	if _, err := chooseRemote(nil); err == nil {
		t.Error("chooseRemote(nil) returned no error, want the ambiguous case reported")
	}
	if _, err := chooseRemote([]string{}); err == nil {
		t.Error("chooseRemote([]) returned no error, want the ambiguous case reported")
	}
}
