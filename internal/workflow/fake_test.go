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

	pausedOps    []git.Operation
	pausedOpsErr error

	continueCalls []git.Operation
	continueErr   error

	abortCalls []git.Operation
	abortErr   error

	softResetCalls []int
	softResetErr   error

	recentCommits    []git.CommitSummary
	recentCommitsErr error

	commitCount    int
	commitCountErr error

	goneBranches    []string
	goneBranchesErr error

	mergedBranches    []string
	mergedBranchesErr error

	deleteCalls []deleteCall
	deleteErr   error
}

type deleteCall struct {
	name  string
	force bool
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

func (f *fakeRepository) PausedOperations(_ context.Context, _ string) ([]git.Operation, error) {
	if f.pausedOpsErr != nil {
		return nil, f.pausedOpsErr
	}
	if f.pausedOps != nil {
		return f.pausedOps, nil
	}
	if f.status.Operation != git.OperationNone {
		return []git.Operation{f.status.Operation}, nil
	}
	return nil, nil
}

func (f *fakeRepository) Continue(_ context.Context, _ string, op git.Operation) error {
	f.continueCalls = append(f.continueCalls, op)
	return f.continueErr
}

func (f *fakeRepository) Abort(_ context.Context, _ string, op git.Operation) error {
	f.abortCalls = append(f.abortCalls, op)
	return f.abortErr
}

func (f *fakeRepository) SoftReset(_ context.Context, _ string, count int) error {
	f.softResetCalls = append(f.softResetCalls, count)
	return f.softResetErr
}

func (f *fakeRepository) RecentCommits(_ context.Context, _ string, _ int) ([]git.CommitSummary, error) {
	if f.recentCommitsErr != nil {
		return nil, f.recentCommitsErr
	}
	return f.recentCommits, nil
}

func (f *fakeRepository) CommitCount(_ context.Context, _ string) (int, error) {
	if f.commitCountErr != nil {
		return 0, f.commitCountErr
	}
	if f.commitCount == 0 {
		return len(f.recentCommits) + 1, nil
	}
	return f.commitCount, nil
}

func (f *fakeRepository) GoneBranches(_ context.Context, _ string) ([]string, error) {
	if f.goneBranchesErr != nil {
		return nil, f.goneBranchesErr
	}
	return f.goneBranches, nil
}

func (f *fakeRepository) MergedBranches(_ context.Context, _, _ string) ([]string, error) {
	if f.mergedBranchesErr != nil {
		return nil, f.mergedBranchesErr
	}
	return f.mergedBranches, nil
}

func (f *fakeRepository) DeleteBranch(_ context.Context, _, name string, force bool) error {
	f.deleteCalls = append(f.deleteCalls, deleteCall{name: name, force: force})
	return f.deleteErr
}
