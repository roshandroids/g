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

const branchTestDir = "/work/nexus_applicant"

// cleanMain is the repository state most tests start from: on main, nothing
// staged, nothing paused.
func cleanMain() *fakeRepository {
	return &fakeRepository{
		status:   git.Status{Root: branchTestDir, Branch: "main"},
		branches: []string{"main", "release/1.0"},
	}
}

func TestServiceNewBranch(t *testing.T) {
	repo := cleanMain()

	result, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{
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
	if result.RemoteExists {
		t.Error("RemoteExists = true, want false when no remote has the branch")
	}

	want := []createCall{{name: "HCM-37538-applicant-id-update", base: "main"}}
	if !reflect.DeepEqual(repo.createCalls, want) {
		t.Errorf("CreateBranch calls = %+v, want %+v", repo.createCalls, want)
	}
}

// A repository whose trunk is called master must work with no configuration,
// which is the whole point of trying both conventional names.
func TestServiceNewBranchFallsBackToMaster(t *testing.T) {
	repo := &fakeRepository{
		status:   git.Status{Root: branchTestDir, Branch: "master"},
		branches: []string{"master"},
	}

	result, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{
		Ticket: "HCM-1",
	})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}
	if result.Base != "master" {
		t.Errorf("Base = %q, want %q", result.Base, "master")
	}
}

func TestServiceNewBranchPrefersTheConfiguredBase(t *testing.T) {
	repo := &fakeRepository{
		status:   git.Status{Root: branchTestDir, Branch: "develop"},
		branches: []string{"develop", "main", "master"},
	}

	opts := Options{DefaultBaseBranch: "develop"}
	result, err := NewService(repo, opts).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-1"})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}
	if result.Base != "develop" {
		t.Errorf("Base = %q, want the configured %q", result.Base, "develop")
	}
}

func TestServiceNewBranchWithoutADescriptionUsesTheKeyAlone(t *testing.T) {
	repo := cleanMain()

	result, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-37538"})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}
	if want := "HCM-37538"; result.Branch != want {
		t.Errorf("Branch = %q, want %q", result.Branch, want)
	}
}

// Uncommitted changes are carried across by `git switch -c`, so they are
// reported rather than treated as an error: blocking here would be a lie about
// what git actually does.
func TestServiceNewBranchReportsADirtyTree(t *testing.T) {
	repo := cleanMain()
	repo.status.Files = []git.FileChange{{Path: "lib/foo.dart", Worktree: git.Modified}}

	result, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-1"})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}
	if !result.Dirty {
		t.Error("Dirty = false, want true")
	}
	if len(repo.createCalls) != 1 {
		t.Errorf("CreateBranch calls = %d, want the branch still created", len(repo.createCalls))
	}
}

func TestServiceNewBranchReportsAnExistingRemoteBranch(t *testing.T) {
	repo := cleanMain()
	repo.remotes = []string{"origin"}
	repo.remoteBranches = map[string]bool{"origin/HCM-1-work": true}

	result, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{
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
	if len(repo.createCalls) != 1 {
		t.Errorf("CreateBranch calls = %d, want the local branch still created", len(repo.createCalls))
	}
}

func TestServiceNewBranchRejectsAnExistingBranch(t *testing.T) {
	repo := cleanMain()
	repo.branches = append(repo.branches, "HCM-1-work")

	_, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{
		Ticket:      "HCM-1",
		Description: "work",
	})
	if !errors.Is(err, ErrBranchExists) {
		t.Fatalf("NewBranch() error = %v, want ErrBranchExists", err)
	}
	if !strings.Contains(err.Error(), "HCM-1-work") {
		t.Errorf("NewBranch() error = %v, want it to name the branch", err)
	}
	if len(repo.createCalls) != 0 {
		t.Errorf("CreateBranch calls = %d, want none", len(repo.createCalls))
	}
}

func TestServiceNewBranchRejectsAnInvalidIssueKey(t *testing.T) {
	repo := cleanMain()

	_, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "nope"})
	if err == nil {
		t.Fatal("NewBranch() error = nil, want a naming error")
	}
	if !strings.Contains(err.Error(), "invalid issue key") {
		t.Errorf("NewBranch() error = %v, want it to explain the issue key", err)
	}
	if len(repo.dirs) != 0 {
		t.Error("NewBranch() inspected the repository before validating the name")
	}
}

func TestServiceNewBranchRejectsADetachedHead(t *testing.T) {
	repo := &fakeRepository{
		status:   git.Status{Root: branchTestDir, Detached: true, Head: "a1b2c3d"},
		branches: []string{"main"},
	}

	_, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-1"})
	if !errors.Is(err, ErrDetachedHead) {
		t.Fatalf("NewBranch() error = %v, want ErrDetachedHead", err)
	}
	if !strings.Contains(err.Error(), "a1b2c3d") {
		t.Errorf("NewBranch() error = %v, want it to name the commit", err)
	}
	if len(repo.createCalls) != 0 {
		t.Errorf("CreateBranch calls = %d, want none", len(repo.createCalls))
	}
}

func TestServiceNewBranchRejectsAnOperationInProgress(t *testing.T) {
	for _, operation := range []git.Operation{
		git.OperationRebase,
		git.OperationMerge,
		git.OperationCherryPick,
		git.OperationRevert,
	} {
		t.Run(string(operation), func(t *testing.T) {
			repo := cleanMain()
			repo.status.Operation = operation

			_, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-1"})
			if !errors.Is(err, ErrOperationInProgress) {
				t.Fatalf("NewBranch() error = %v, want ErrOperationInProgress", err)
			}
			if !strings.Contains(err.Error(), string(operation)) {
				t.Errorf("NewBranch() error = %v, want it to name the %s", err, operation)
			}
			if len(repo.createCalls) != 0 {
				t.Errorf("CreateBranch calls = %d, want none", len(repo.createCalls))
			}
		})
	}
}

// During a rebase HEAD is detached, so reporting "detached HEAD" would hide the
// real problem. The operation has to win.
func TestServiceNewBranchPrefersTheOperationOverDetachedHead(t *testing.T) {
	repo := cleanMain()
	repo.status.Detached = true
	repo.status.Head = "a1b2c3d"
	repo.status.Operation = git.OperationRebase

	_, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-1"})
	if !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("NewBranch() error = %v, want ErrOperationInProgress", err)
	}
}

func TestServiceNewBranchRejectsAnUnusableRepository(t *testing.T) {
	repo := &fakeRepository{statusErr: fmt.Errorf("%w: %s", git.ErrNotARepository, branchTestDir)}

	_, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-1"})
	if !errors.Is(err, ErrNotARepository) {
		t.Fatalf("NewBranch() error = %v, want ErrNotARepository", err)
	}
}

func TestServiceNewBranchWithoutABaseBranch(t *testing.T) {
	repo := &fakeRepository{
		status:   git.Status{Root: branchTestDir, Branch: "topic"},
		branches: []string{"topic"},
	}

	_, err := NewService(repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-1"})
	if err == nil {
		t.Fatal("NewBranch() error = nil, want a base branch error")
	}
	if !strings.Contains(err.Error(), "cannot find a base branch") {
		t.Errorf("NewBranch() error = %v, want it to explain the missing base", err)
	}
	if len(repo.createCalls) != 0 {
		t.Errorf("CreateBranch calls = %d, want none", len(repo.createCalls))
	}
}

func TestServiceNewBranchPropagatesFailures(t *testing.T) {
	tests := []struct {
		name string
		repo *fakeRepository
	}{
		{
			name: "listing local branches",
			repo: &fakeRepository{
				status:      git.Status{Root: branchTestDir, Branch: "main"},
				branchesErr: errors.New("for-each-ref exploded"),
			},
		},
		{
			name: "listing remotes",
			repo: &fakeRepository{
				status:     git.Status{Root: branchTestDir, Branch: "main"},
				branches:   []string{"main"},
				remotesErr: errors.New("remote exploded"),
			},
		},
		{
			name: "asking the remote about the branch",
			repo: &fakeRepository{
				status:          git.Status{Root: branchTestDir, Branch: "main"},
				branches:        []string{"main"},
				remotes:         []string{"origin"},
				remoteBranchErr: errors.New("ls-remote exploded"),
			},
		},
		{
			name: "creating the branch",
			repo: &fakeRepository{
				status:    git.Status{Root: branchTestDir, Branch: "main"},
				branches:  []string{"main"},
				createErr: errors.New("switch exploded"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(test.repo, Options{}).NewBranch(context.Background(), branchTestDir, NewBranchRequest{Ticket: "HCM-1"})
			if err == nil {
				t.Fatal("NewBranch() error = nil, want the underlying failure")
			}
			if !strings.Contains(err.Error(), "exploded") {
				t.Errorf("NewBranch() error = %v, want it to keep the cause", err)
			}
		})
	}
}

func TestServiceSwitch(t *testing.T) {
	repo := cleanMain()

	result, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: "release/1.0"})
	if err != nil {
		t.Fatalf("Switch() error = %v, want nil", err)
	}

	if result.Branch != "release/1.0" {
		t.Errorf("Branch = %q, want %q", result.Branch, "release/1.0")
	}
	if result.Previous != "main" {
		t.Errorf("Previous = %q, want %q", result.Previous, "main")
	}
	if result.AlreadyOn {
		t.Error("AlreadyOn = true, want false")
	}
	if want := []string{"release/1.0"}; !reflect.DeepEqual(repo.switchCalls, want) {
		t.Errorf("SwitchBranch calls = %v, want %v", repo.switchCalls, want)
	}
}

// A branch name may contain slashes, and g must not treat them as path
// separators or as anything other than part of the name.
func TestServiceSwitchAcceptsSlashesInTheName(t *testing.T) {
	repo := &fakeRepository{
		status:   git.Status{Root: branchTestDir, Branch: "main"},
		branches: []string{"main", "feature/deep/nested"},
	}

	result, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: "feature/deep/nested"})
	if err != nil {
		t.Fatalf("Switch() error = %v, want nil", err)
	}
	if result.Branch != "feature/deep/nested" {
		t.Errorf("Branch = %q, want %q", result.Branch, "feature/deep/nested")
	}
}

func TestServiceSwitchReportsAlreadyBeingOnTheBranch(t *testing.T) {
	repo := cleanMain()

	result, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: "main"})
	if err != nil {
		t.Fatalf("Switch() error = %v, want nil", err)
	}
	if !result.AlreadyOn {
		t.Error("AlreadyOn = false, want true")
	}
	if len(repo.switchCalls) != 0 {
		t.Errorf("SwitchBranch calls = %v, want none when already on the branch", repo.switchCalls)
	}
}

func TestServiceSwitchReportsADirtyTree(t *testing.T) {
	repo := cleanMain()
	repo.status.Files = []git.FileChange{{Path: "lib/foo.dart", Worktree: git.Modified}}

	result, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: "release/1.0"})
	if err != nil {
		t.Fatalf("Switch() error = %v, want nil", err)
	}
	if !result.Dirty {
		t.Error("Dirty = false, want true")
	}
}

func TestServiceSwitchRejectsAnUnknownBranch(t *testing.T) {
	repo := cleanMain()

	_, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: "nope"})
	if !errors.Is(err, ErrBranchNotFound) {
		t.Fatalf("Switch() error = %v, want ErrBranchNotFound", err)
	}
	if !strings.Contains(err.Error(), `"nope"`) {
		t.Errorf("Switch() error = %v, want it to name the branch", err)
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("Switch() error = %v, want it to list the branches that do exist", err)
	}
	if len(repo.switchCalls) != 0 {
		t.Errorf("SwitchBranch calls = %v, want none", repo.switchCalls)
	}
}

func TestServiceSwitchRejectsAnEmptyName(t *testing.T) {
	repo := cleanMain()

	for _, name := range []string{"", "   "} {
		_, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: name})
		if err == nil {
			t.Fatalf("Switch(%q) error = nil, want an error", name)
		}
		if !strings.Contains(err.Error(), "branch name is required") {
			t.Errorf("Switch(%q) error = %v, want it to ask for a name", name, err)
		}
	}
	if len(repo.dirs) != 0 {
		t.Error("Switch() inspected the repository before validating the name")
	}
}

func TestServiceSwitchRejectsADetachedHead(t *testing.T) {
	repo := &fakeRepository{
		status:   git.Status{Root: branchTestDir, Detached: true, Head: "a1b2c3d"},
		branches: []string{"main"},
	}

	_, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: "main"})
	if !errors.Is(err, ErrDetachedHead) {
		t.Fatalf("Switch() error = %v, want ErrDetachedHead", err)
	}
}

func TestServiceSwitchRejectsAnOperationInProgress(t *testing.T) {
	repo := cleanMain()
	repo.status.Operation = git.OperationRebase

	_, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: "release/1.0"})
	if !errors.Is(err, ErrOperationInProgress) {
		t.Fatalf("Switch() error = %v, want ErrOperationInProgress", err)
	}
}

// git refusing a switch because local changes would be overwritten is the
// safety net that keeps uncommitted work; the diagnostic must survive.
func TestServiceSwitchSurfacesGitsRefusal(t *testing.T) {
	repo := cleanMain()
	repo.switchErr = errors.New("error: Your local changes would be overwritten")

	_, err := NewService(repo, Options{}).Switch(context.Background(), branchTestDir, SwitchRequest{Branch: "release/1.0"})
	if err == nil {
		t.Fatal("Switch() error = nil, want git's refusal")
	}
	if !strings.Contains(err.Error(), "would be overwritten") {
		t.Errorf("Switch() error = %v, want it to keep git's explanation", err)
	}
}

func TestNewServiceAppliesDefaults(t *testing.T) {
	service := NewService(&fakeRepository{}, Options{})

	if got := service.opts.DefaultBaseBranch; got != "main" {
		t.Errorf("DefaultBaseBranch = %q, want %q", got, "main")
	}
	if got := service.opts.BranchSeparator; got != "-" {
		t.Errorf("BranchSeparator = %q, want %q", got, "-")
	}
	if service.naming == nil {
		t.Error("naming = nil, want a usable policy")
	}
}

func TestServiceNewBranchHonoursTheConfiguredSeparator(t *testing.T) {
	repo := cleanMain()

	opts := Options{BranchSeparator: "_"}
	result, err := NewService(repo, opts).NewBranch(context.Background(), branchTestDir, NewBranchRequest{
		Ticket:      "HCM-1",
		Description: "applicant id update",
	})
	if err != nil {
		t.Fatalf("NewBranch() error = %v, want nil", err)
	}
	if want := "HCM-1_applicant_id_update"; result.Branch != want {
		t.Errorf("Branch = %q, want %q", result.Branch, want)
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name     string
		branches []string
		want     string
	}{
		{name: "none", branches: nil, want: ""},
		{name: "one", branches: []string{"main"}, want: "main"},
		{name: "several", branches: []string{"main", "dev"}, want: "main, dev"},
		{
			name:     "truncated",
			branches: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
			want:     "a, b, c, d, e, f, g, h and 1 more",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := list(test.branches); got != test.want {
				t.Errorf("list() = %q, want %q", got, test.want)
			}
		})
	}
}
