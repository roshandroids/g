// Package branch derives and validates branch names.
//
// Naming is a policy rather than a rule of the tool: the default convention is
// "<ISSUE-KEY><separator><slug>", and the parts that can reasonably vary are
// configurable. Nothing here executes git or touches the filesystem, so the
// naming rules are testable on their own and reusable by any front end.
package branch

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DefaultSeparator joins the issue key and the description slug.
const DefaultSeparator = "-"

// Namer derives a branch name from an issue key and a description.
type Namer interface {
	// Name returns the branch name for the given work, or an error when no
	// safe name can be derived.
	Name(key, description string) (string, error)
}

// Policy is the branch naming convention.
type Policy struct {
	// Separator joins the issue key and the description words. Only "-", "_"
	// and "/" are accepted, because every other candidate is either invalid
	// in a Git ref or makes an invalid one easy to produce.
	Separator string
	// MaxLength caps the length of the generated name; zero means no limit.
	// Truncation drops whole trailing words where it can.
	MaxLength int
}

// DefaultPolicy returns the naming convention g uses unless configured
// otherwise.
func DefaultPolicy() Policy {
	return Policy{Separator: DefaultSeparator}
}

// Name builds the branch name for an issue key and a description.
//
// The issue key is preserved in upper case and the description is reduced to a
// slug, so `HCM-37538` and `"applicant id update"` become
// `HCM-37538-applicant-id-update`. A description that reduces to nothing leaves
// the issue key alone rather than producing a trailing separator.
func (p Policy) Name(key, description string) (string, error) {
	issue, err := IssueKey(key)
	if err != nil {
		return "", err
	}

	separator := p.Separator
	if separator == "" {
		separator = DefaultSeparator
	}
	if err := validateSeparator(separator); err != nil {
		return "", err
	}

	name := issue
	if slug := Slug(description, separator); slug != "" {
		name = issue + separator + slug
	}

	if p.MaxLength > 0 && len(name) > p.MaxLength {
		if name, err = truncate(name, p.MaxLength, separator, issue); err != nil {
			return "", err
		}
	}

	if err := Validate(name); err != nil {
		return "", err
	}
	return name, nil
}

// issueKeyPattern matches a Jira issue key: a project prefix followed by an
// issue number.
//
// The project prefix is deliberately not fixed, so this works for any Jira
// project rather than one hard-coded key.
var issueKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*-[1-9][0-9]*$`)

// IssueKey normalizes and validates an issue key.
//
// It returns the key in upper case, so a lower case key typed at the prompt
// still produces the conventional branch name.
func IssueKey(key string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(key))
	if !issueKeyPattern.MatchString(normalized) {
		return "", fmt.Errorf("invalid issue key %q: expected a key like HCM-37538", key)
	}
	return normalized, nil
}

// Slug reduces free text to a branch-safe fragment.
//
// Every character that is not an ASCII letter or digit becomes a separator,
// runs of separators collapse into one, and separators never appear at either
// end. The result is lower case and safe to embed in a Git ref.
func Slug(text, separator string) string {
	var b strings.Builder
	pending := false

	for _, r := range strings.ToLower(text) {
		if isSlugRune(r) {
			if pending && b.Len() > 0 {
				b.WriteString(separator)
			}
			pending = false
			b.WriteRune(r)
			continue
		}
		pending = true
	}

	return b.String()
}

// isSlugRune reports whether r survives slugging.
//
// The check is intentionally ASCII-only: non-ASCII letters have no single
// obvious transliteration, and turning them into a separator is predictable.
func isSlugRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// Validate reports whether name is a branch name git will accept.
//
// The rules mirror git-check-ref-format(1). Rejecting a name here means the
// user gets an explanation instead of a raw git failure, and it keeps a bad
// configured separator from ever reaching the repository.
func Validate(name string) error {
	switch {
	case name == "":
		return errors.New("branch name is empty")
	case name == "@":
		return fmt.Errorf("invalid branch name %q", name)
	case strings.HasPrefix(name, "-"):
		return fmt.Errorf("invalid branch name %q: must not start with \"-\"", name)
	case strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/"):
		return fmt.Errorf("invalid branch name %q: must not start or end with \"/\"", name)
	case strings.HasSuffix(name, "."):
		return fmt.Errorf("invalid branch name %q: must not end with \".\"", name)
	case strings.Contains(name, ".."):
		return fmt.Errorf("invalid branch name %q: must not contain \"..\"", name)
	case strings.Contains(name, "@{"):
		return fmt.Errorf("invalid branch name %q: must not contain \"@{\"", name)
	case strings.Contains(name, "//"):
		return fmt.Errorf("invalid branch name %q: must not contain \"//\"", name)
	case strings.ContainsAny(name, " ~^:?*[\\"):
		return fmt.Errorf("invalid branch name %q: must not contain %q", name, offending(name, " ~^:?*[\\"))
	case hasControlRune(name):
		return fmt.Errorf("invalid branch name %q: must not contain control characters", name)
	}

	for _, component := range strings.Split(name, "/") {
		switch {
		case strings.HasPrefix(component, "."):
			return fmt.Errorf("invalid branch name %q: no part may start with \".\"", name)
		case strings.HasSuffix(component, ".lock"):
			return fmt.Errorf("invalid branch name %q: no part may end with \".lock\"", name)
		}
	}

	return nil
}

// truncate shortens name to at most limit characters, preferring to drop a
// whole trailing word rather than leaving half of one behind.
//
// The issue key is never shortened; a limit that cannot hold it is an error
// rather than a silently mangled branch name.
func truncate(name string, limit int, separator, issue string) (string, error) {
	if len(issue) > limit {
		return "", fmt.Errorf("issue key %q is longer than the %d character branch limit", issue, limit)
	}

	cut := name[:limit]

	// When the cut lands exactly where a separator starts, it already falls on
	// a word boundary and nothing more has to be dropped.
	if !strings.HasPrefix(name[limit:], separator) {
		if i := strings.LastIndex(cut, separator); i >= 0 {
			cut = cut[:i]
		}
	}

	return strings.TrimRight(cut, separator), nil
}

// validateSeparator rejects separators that could produce a name git refuses.
func validateSeparator(separator string) error {
	switch separator {
	case "-", "_", "/":
		return nil
	default:
		return fmt.Errorf("invalid branch name separator %q: use \"-\", \"_\" or \"/\"", separator)
	}
}

// offending returns the first character of invalid that appears in name.
func offending(name, invalid string) string {
	if i := strings.IndexAny(name, invalid); i >= 0 {
		return string(name[i])
	}
	return invalid[:1]
}

// hasControlRune reports whether name contains an ASCII control character.
func hasControlRune(name string) bool {
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
