// Package commit defines the commit message convention g enforces.
//
// The convention is deliberately small: a fixed set of types followed by the
// author's own words. g never invents or rewrites a message, so the only thing
// this package does is refuse input that would produce a commit nobody meant to
// make. Like internal/branch it is pure: no processes, no filesystem.
package commit

import (
	"errors"
	"fmt"
	"strings"
)

// Type is a conventional commit type that g accepts.
type Type string

// The accepted commit types.
const (
	Feature     Type = "feat"
	Fix         Type = "fix"
	UI          Type = "ui"
	Refactor    Type = "refactor"
	Test        Type = "test"
	Docs        Type = "docs"
	Chore       Type = "chore"
	Continuous  Type = "ci"
	Performance Type = "perf"
)

// accepted lists the types in the order they are shown to the user.
var accepted = []Type{Feature, Fix, UI, Refactor, Test, Docs, Chore, Continuous, Performance}

// Types returns the accepted commit types.
func Types() []Type {
	return append([]Type(nil), accepted...)
}

// ParseType validates a commit type, ignoring case and surrounding whitespace.
func ParseType(value string) (Type, error) {
	normalized := Type(strings.ToLower(strings.TrimSpace(value)))
	for _, candidate := range accepted {
		if candidate == normalized {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid commit type %q: use one of %s", value, strings.Join(names(), ", "))
}

// Message assembles the commit subject from a type and a subject.
//
// The words are the author's own: nothing is reworded, re-cased or generated.
// Only surrounding whitespace is trimmed, which the shell would otherwise
// smuggle into the middle of the message after "type: ".
func Message(commitType Type, subject string) string {
	return string(commitType) + ": " + strings.TrimSpace(subject)
}

// ValidateSubject rejects a subject that cannot become a usable commit message.
func ValidateSubject(subject string) error {
	switch {
	case strings.TrimSpace(subject) == "":
		return errors.New("the commit message is empty")
	case strings.ContainsAny(subject, "\r\n"):
		return errors.New("the commit message must be a single line; g does not rewrite it")
	}
	return nil
}

// names returns the accepted type names for an error message.
func names() []string {
	result := make([]string, 0, len(accepted))
	for _, commitType := range accepted {
		result = append(result, string(commitType))
	}
	return result
}
