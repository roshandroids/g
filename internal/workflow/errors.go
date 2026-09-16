package workflow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/roshandroids/g/internal/git"
)

// The conditions the workflow layer reports as values rather than as opaque
// failures, so callers can react to them without matching on message text.
var (
	// ErrNotARepository reports that the target directory is not inside a Git
	// work tree.
	ErrNotARepository = errors.New("not inside a Git repository")
	// ErrBranchExists reports an attempt to create a branch that is already
	// there.
	ErrBranchExists = errors.New("branch already exists")
	// ErrBranchNotFound reports a branch that does not exist.
	ErrBranchNotFound = errors.New("branch does not exist")
	// ErrDetachedHead reports that HEAD points at a commit rather than a
	// branch.
	ErrDetachedHead = errors.New("HEAD is detached")
	// ErrOperationInProgress reports that Git is part way through another
	// operation, such as a rebase.
	ErrOperationInProgress = errors.New("another Git operation is in progress")
)

// BranchExistsError reports that the branch to create is already present.
type BranchExistsError struct {
	// Name is the branch that already exists.
	Name string
}

func (e *BranchExistsError) Error() string {
	return fmt.Sprintf("branch %q already exists; run `g switch %s` to work on it", e.Name, e.Name)
}

// Is reports the error as ErrBranchExists so callers can test for the condition
// without depending on the message.
func (e *BranchExistsError) Is(target error) bool { return target == ErrBranchExists }

// BranchNotFoundError reports that no local branch has the requested name.
type BranchNotFoundError struct {
	// Name is the branch that was requested.
	Name string
	// Available lists the local branches that do exist.
	Available []string
}

func (e *BranchNotFoundError) Error() string {
	message := fmt.Sprintf("branch %q does not exist", e.Name)
	if branches := list(e.Available); branches != "" {
		message += "; local branches: " + branches
	}
	return message
}

// Is reports the error as ErrBranchNotFound.
func (e *BranchNotFoundError) Is(target error) bool { return target == ErrBranchNotFound }

// OperationError reports that Git is paused inside another operation.
//
// Creating or switching branches while a rebase is paused would leave the
// rebase without a branch to finish on, so the workflow refuses rather than
// letting the user discover it later.
type OperationError struct {
	// Operation is the operation Git is part way through.
	Operation git.Operation
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("a %s is already in progress; finish or abort it first", e.Operation)
}

// Is reports the error as ErrOperationInProgress.
func (e *OperationError) Is(target error) bool { return target == ErrOperationInProgress }

// maxListedBranches caps how many branch names an error message lists.
const maxListedBranches = 8

// list renders branch names for an error message, keeping it short enough to
// read when a repository has many branches.
func list(branches []string) string {
	if len(branches) <= maxListedBranches {
		return strings.Join(branches, ", ")
	}
	return fmt.Sprintf("%s and %d more",
		strings.Join(branches[:maxListedBranches], ", "),
		len(branches)-maxListedBranches)
}
