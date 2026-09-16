// Package workflow holds the UI-independent operations g performs on a
// repository.
//
// Everything here takes and returns plain values, so the CLI and any future GUI
// drive exactly the same logic. The package never executes anything itself: it
// depends on the Repository interface, which the git package implements and
// tests replace with a fake.
package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/roshandroids/g/internal/git"
)

// ErrNotARepository reports that the target directory is not inside a Git work
// tree.
var ErrNotARepository = errors.New("not inside a Git repository")

// Repository is the Git capability the workflow layer needs.
type Repository interface {
	// Status reports the state of the repository containing dir.
	Status(ctx context.Context, dir string) (git.Status, error)
}

// Service performs repository workflows.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Status summarises the repository containing dir.
func (s *Service) Status(ctx context.Context, dir string) (Status, error) {
	raw, err := s.repo.Status(ctx, dir)
	if err != nil {
		if errors.Is(err, git.ErrNotARepository) {
			return Status{}, fmt.Errorf("%w: %s", ErrNotARepository, dir)
		}
		return Status{}, err
	}
	return summarize(raw), nil
}
