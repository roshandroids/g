// Package prompt asks the user to confirm an action.
//
// It exists so that the commands which can overwrite work elsewhere have
// somewhere to ask, and so the workflow layer never has to. A workflow exposes
// a flag such as "force this push"; deciding whether the user really meant it is
// a conversation, and conversations belong at the edge of the program.
package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Prompter asks the user to confirm an action.
type Prompter interface {
	// Confirm asks a yes/no question and reports the answer.
	//
	// It returns false, and no error, when the answer is no and when there is
	// nobody there to answer: an unanswered confirmation is never a yes.
	Confirm(ctx context.Context, question string) (bool, error)
}

// Terminal asks questions on a terminal, reading from In and writing to Out.
type Terminal struct {
	in  *bufio.Reader
	out io.Writer
}

// New returns a Terminal reading answers from in and writing questions to out.
func New(in io.Reader, out io.Writer) *Terminal {
	return &Terminal{in: bufio.NewReader(in), out: out}
}

// Confirm implements Prompter.
func (t *Terminal) Confirm(ctx context.Context, question string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	if _, err := fmt.Fprintf(t.out, "%s [y/N] ", question); err != nil {
		return false, fmt.Errorf("asking for confirmation: %w", err)
	}

	answer, err := t.in.ReadString('\n')
	// Reaching the end of the input without an answer means there is nobody to
	// confirm, which is a no. Any other read failure is a real failure.
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading the answer: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// Deny is a Prompter that refuses everything.
//
// It stands in when g has no terminal to ask on, so that a command requiring
// confirmation fails closed rather than assuming consent.
type Deny struct{}

// Confirm implements Prompter.
func (Deny) Confirm(context.Context, string) (bool, error) { return false, nil }
