package workflow

import (
	"context"

	"github.com/roshandroids/g/internal/git"
)

// fakeRepository is a scripted Repository.
//
// Each method records the calls it receives and returns whatever the test
// configured, so workflow behaviour is tested without executing any process.
type fakeRepository struct {
	status    git.Status
	statusErr error
	dirs      []string

	branches    []string
	branchesErr error

	remotes    []string
	remotesErr error

	remoteBranches    map[string]bool
	remoteBranchErr   error
	remoteBranchCalls []remoteBranchCall

	createCalls []createCall
	createErr   error

	switchCalls []string
	switchErr   error

	commitCalls []string
	commitErr   error

	pushCalls []git.PushOptions
	pushErr   error
}

// remoteBranchCall records one RemoteBranchExists question.
type remoteBranchCall struct {
	remote string
	name   string
}

// createCall records one CreateBranch request.
type createCall struct {
	name string
	base string
}

func (f *fakeRepository) Status(_ context.Context, dir string) (git.Status, error) {
	f.dirs = append(f.dirs, dir)
	if f.statusErr != nil {
		return git.Status{}, f.statusErr
	}
	return f.status, nil
}

func (f *fakeRepository) LocalBranches(_ context.Context, _ string) ([]string, error) {
	if f.branchesErr != nil {
		return nil, f.branchesErr
	}
	return f.branches, nil
}

func (f *fakeRepository) Remotes(_ context.Context, _ string) ([]string, error) {
	if f.remotesErr != nil {
		return nil, f.remotesErr
	}
	return f.remotes, nil
}

func (f *fakeRepository) RemoteBranchExists(_ context.Context, _, remote, name string) (bool, error) {
	f.remoteBranchCalls = append(f.remoteBranchCalls, remoteBranchCall{remote: remote, name: name})
	if f.remoteBranchErr != nil {
		return false, f.remoteBranchErr
	}
	return f.remoteBranches[remote+"/"+name], nil
}

func (f *fakeRepository) CreateBranch(_ context.Context, _, name, base string) error {
	f.createCalls = append(f.createCalls, createCall{name: name, base: base})
	return f.createErr
}

func (f *fakeRepository) SwitchBranch(_ context.Context, _, name string) error {
	f.switchCalls = append(f.switchCalls, name)
	return f.switchErr
}

func (f *fakeRepository) Commit(_ context.Context, _, message string) error {
	f.commitCalls = append(f.commitCalls, message)
	return f.commitErr
}

func (f *fakeRepository) Push(_ context.Context, _ string, opts git.PushOptions) error {
	f.pushCalls = append(f.pushCalls, opts)
	return f.pushErr
}
