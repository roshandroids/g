package workflow

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/git"
)

// fakeRepository returns a fixed git state, so workflow behaviour is tested
// without executing any process.
type fakeRepository struct {
	status git.Status
	err    error
	dirs   []string
}

func (f *fakeRepository) Status(_ context.Context, dir string) (git.Status, error) {
	f.dirs = append(f.dirs, dir)
	if f.err != nil {
		return git.Status{}, f.err
	}
	return f.status, nil
}

func TestServiceStatusSummarizesRepository(t *testing.T) {
	const dir = "/work/nexus_applicant/lib"

	repo := &fakeRepository{status: git.Status{
		Root:     "/work/nexus_applicant",
		Branch:   "HCM-37538-applicant-id-update",
		Upstream: "origin/HCM-37538-applicant-id-update",
		Ahead:    2,
		Behind:   1,
		Files: []git.FileChange{
			{Path: "lib/foo.dart", Index: git.Modified},
			{Path: "lib/bar.dart", Worktree: git.Modified},
			{Path: "lib/baz.dart", OriginalPath: "lib/bar.dart", Index: git.Renamed},
			{Path: "test/foo_test.dart", Untracked: true},
			{Path: "lib/conflict.dart", Index: git.Unmerged, Worktree: git.Unmerged, Unmerged: true},
			{Path: "lib/both.dart", Index: git.Modified, Worktree: git.Modified},
		},
	}}

	status, err := NewService(repo).Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}

	if status.Name != "nexus_applicant" {
		t.Errorf("Name = %q, want %q", status.Name, "nexus_applicant")
	}
	if status.Root != "/work/nexus_applicant" {
		t.Errorf("Root = %q, want %q", status.Root, "/work/nexus_applicant")
	}
	if status.Branch != "HCM-37538-applicant-id-update" {
		t.Errorf("Branch = %q, want %q", status.Branch, "HCM-37538-applicant-id-update")
	}
	if status.Detached {
		t.Error("Detached = true, want false")
	}
	if status.Clean() {
		t.Error("Clean() = true, want false for a dirty repository")
	}

	wantUpstream := &Upstream{Name: "origin/HCM-37538-applicant-id-update", Ahead: 2, Behind: 1}
	if !reflect.DeepEqual(status.Upstream, wantUpstream) {
		t.Errorf("Upstream = %+v, want %+v", status.Upstream, wantUpstream)
	}

	wantStaged := []Change{
		{Path: "lib/foo.dart", Kind: git.Modified},
		{Path: "lib/baz.dart", OriginalPath: "lib/bar.dart", Kind: git.Renamed},
		{Path: "lib/both.dart", Kind: git.Modified},
	}
	if !reflect.DeepEqual(status.Staged, wantStaged) {
		t.Errorf("Staged = %+v, want %+v", status.Staged, wantStaged)
	}

	wantUnstaged := []Change{
		{Path: "lib/bar.dart", Kind: git.Modified},
		{Path: "lib/both.dart", Kind: git.Modified},
	}
	if !reflect.DeepEqual(status.Unstaged, wantUnstaged) {
		t.Errorf("Unstaged = %+v, want %+v", status.Unstaged, wantUnstaged)
	}

	if want := []string{"test/foo_test.dart"}; !reflect.DeepEqual(status.Untracked, want) {
		t.Errorf("Untracked = %v, want %v", status.Untracked, want)
	}
	if want := []string{"lib/conflict.dart"}; !reflect.DeepEqual(status.Conflicted, want) {
		t.Errorf("Conflicted = %v, want %v", status.Conflicted, want)
	}

	if want := []string{dir}; !reflect.DeepEqual(repo.dirs, want) {
		t.Errorf("repository was asked about %v, want %v", repo.dirs, want)
	}
}

func TestServiceStatusCleanRepository(t *testing.T) {
	repo := &fakeRepository{status: git.Status{Root: "/work/demo", Branch: "main"}}

	status, err := NewService(repo).Status(context.Background(), "/work/demo")
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}

	if !status.Clean() {
		t.Error("Clean() = false, want true")
	}
	if status.Upstream != nil {
		t.Errorf("Upstream = %+v, want nil when nothing is tracked", status.Upstream)
	}
	if status.Name != "demo" {
		t.Errorf("Name = %q, want %q", status.Name, "demo")
	}
}

func TestServiceStatusDetachedHead(t *testing.T) {
	repo := &fakeRepository{status: git.Status{Root: "/work/demo", Detached: true, Head: "a1b2c3d"}}

	status, err := NewService(repo).Status(context.Background(), "/work/demo")
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}

	if !status.Detached {
		t.Error("Detached = false, want true")
	}
	if status.Branch != "" {
		t.Errorf("Branch = %q, want empty for a detached HEAD", status.Branch)
	}
	if status.Head != "a1b2c3d" {
		t.Errorf("Head = %q, want %q", status.Head, "a1b2c3d")
	}
}

func TestServiceStatusReportsNotARepository(t *testing.T) {
	repo := &fakeRepository{err: fmt.Errorf("%w: %s", git.ErrNotARepository, "/tmp")}

	_, err := NewService(repo).Status(context.Background(), "/tmp")
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("Status() error = %v, want ErrNotARepository", err)
	}
	if !strings.Contains(err.Error(), "/tmp") {
		t.Errorf("Status() error = %v, want it to name the directory", err)
	}
}

func TestServiceStatusPropagatesGitFailures(t *testing.T) {
	repo := &fakeRepository{err: &git.CommandError{Args: []string{"status"}, ExitCode: 128, Stderr: "boom"}}

	_, err := NewService(repo).Status(context.Background(), "/work/demo")

	var commandErr *git.CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Status() error = %v, want *git.CommandError", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("Status() error = %v, want it to keep the git diagnostic", err)
	}
}

func TestStatusClean(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "empty", status: Status{}, want: true},
		{name: "staged", status: Status{Staged: []Change{{Path: "a"}}}, want: false},
		{name: "unstaged", status: Status{Unstaged: []Change{{Path: "a"}}}, want: false},
		{name: "untracked", status: Status{Untracked: []string{"a"}}, want: false},
		{name: "conflicted", status: Status{Conflicted: []string{"a"}}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.status.Clean(); got != test.want {
				t.Errorf("Clean() = %t, want %t", got, test.want)
			}
		})
	}
}
