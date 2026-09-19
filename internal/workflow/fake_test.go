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

	remoteDefault    string
	remoteDefaultErr error

	createCalls []createCall
	createErr   error

	switchCalls []string
	switchErr   error

	commitCalls []string
	commitErr   error

	pushCalls []git.PushOptions
	pushErr   error

	fetchCalls []string
	fetchErr   error

	aheadBehind      aheadBehindResult
	aheadBehindSeq   []aheadBehindResult
	aheadBehindErr   error
	aheadBehindCalls []aheadBehindCall

	revParse    string
	revParseErr error
	revParses   []string

	ffBranchCalls []ffBranchCall
	ffBranchErr   error

	mergeFFCalls []string
	mergeFFErr   error

	rebaseCalls []string
	rebaseErr   error

	preserveResult git.PreserveResult
	preserveErr    error
	preserveCalls  int

	restoreErr   error
	restoreCalls int
}

type remoteBranchCall struct {
	remote string
	name   string
}

type createCall struct {
	name string
	base string
}

type aheadBehindCall struct {
	left  string
	right string
}

type aheadBehindResult struct {
	ahead  int
	behind int
}

type ffBranchCall struct {
	branch string
	tip    string
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

func (f *fakeRepository) RemoteDefaultBranch(_ context.Context, _, _ string) (string, error) {
	if f.remoteDefaultErr != nil {
		return "", f.remoteDefaultErr
	}
	return f.remoteDefault, nil
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

func (f *fakeRepository) Fetch(_ context.Context, _, remote string) error {
	f.fetchCalls = append(f.fetchCalls, remote)
	return f.fetchErr
}

func (f *fakeRepository) AheadBehind(_ context.Context, _, left, right string) (int, int, error) {
	f.aheadBehindCalls = append(f.aheadBehindCalls, aheadBehindCall{left: left, right: right})
	if f.aheadBehindErr != nil {
		return 0, 0, f.aheadBehindErr
	}
	if len(f.aheadBehindSeq) > 0 {
		next := f.aheadBehindSeq[0]
		f.aheadBehindSeq = f.aheadBehindSeq[1:]
		return next.ahead, next.behind, nil
	}
	return f.aheadBehind.ahead, f.aheadBehind.behind, nil
}

func (f *fakeRepository) RevParse(_ context.Context, _, _ string) (string, error) {
	if f.revParseErr != nil {
		return "", f.revParseErr
	}
	if len(f.revParses) > 0 {
		value := f.revParses[0]
		f.revParses = f.revParses[1:]
		return value, nil
	}
	return f.revParse, nil
}

func (f *fakeRepository) FastForwardBranch(_ context.Context, _, branch, tip string) error {
	f.ffBranchCalls = append(f.ffBranchCalls, ffBranchCall{branch: branch, tip: tip})
	return f.ffBranchErr
}

func (f *fakeRepository) MergeFastForward(_ context.Context, _, rev string) error {
	f.mergeFFCalls = append(f.mergeFFCalls, rev)
	return f.mergeFFErr
}

func (f *fakeRepository) Rebase(_ context.Context, _, onto string) error {
	f.rebaseCalls = append(f.rebaseCalls, onto)
	return f.rebaseErr
}

func (f *fakeRepository) PreserveChanges(_ context.Context, _, _ string) (git.PreserveResult, error) {
	f.preserveCalls++
	if f.preserveErr != nil {
		return git.PreserveResult{}, f.preserveErr
	}
	return f.preserveResult, nil
}

func (f *fakeRepository) RestoreChanges(_ context.Context, _ string) error {
	f.restoreCalls++
	return f.restoreErr
}
