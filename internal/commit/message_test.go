package commit

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseType(t *testing.T) {
	valid := []struct {
		input string
		want  Type
	}{
		{input: "feat", want: Feature},
		{input: "fix", want: Fix},
		{input: "ui", want: UI},
		{input: "refactor", want: Refactor},
		{input: "test", want: Test},
		{input: "docs", want: Docs},
		{input: "chore", want: Chore},
		{input: "ci", want: Continuous},
		{input: "perf", want: Performance},
		{input: "FIX", want: Fix},
		{input: "Fix", want: Fix},
		{input: "  feat  ", want: Feature},
	}

	for _, test := range valid {
		t.Run("valid/"+test.input, func(t *testing.T) {
			got, err := ParseType(test.input)
			if err != nil {
				t.Fatalf("ParseType(%q) error = %v, want nil", test.input, err)
			}
			if got != test.want {
				t.Errorf("ParseType(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}

	invalid := []string{"", "  ", "feature", "bugfix", "wip", "style", "build", "revert", "f"}
	for _, input := range invalid {
		t.Run("invalid/"+input, func(t *testing.T) {
			got, err := ParseType(input)
			if err == nil {
				t.Fatalf("ParseType(%q) = %q, want an error", input, got)
			}
			if !strings.Contains(err.Error(), "invalid commit type") {
				t.Errorf("ParseType(%q) error = %v, want it to explain the type", input, err)
			}
			// The message has to list what is actually accepted, or the user
			// cannot correct the mistake.
			for _, want := range []string{"feat", "fix", "ui", "refactor", "test", "docs", "chore", "ci", "perf"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("ParseType(%q) error = %v, want it to list %q", input, err, want)
				}
			}
		})
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name    string
		typ     Type
		subject string
		want    string
	}{
		{
			name:    "the documented example",
			typ:     Fix,
			subject: "resolve applicant history issue",
			want:    "fix: resolve applicant history issue",
		},
		{
			name:    "feature",
			typ:     Feature,
			subject: "add applicant document workflow",
			want:    "feat: add applicant document workflow",
		},
		{
			name:    "ui",
			typ:     UI,
			subject: "update applicant history layout",
			want:    "ui: update applicant history layout",
		},
		{
			name:    "the words are left alone",
			typ:     Fix,
			subject: "Do NOT reword THIS, or re-case it",
			want:    "fix: Do NOT reword THIS, or re-case it",
		},
		{
			name:    "punctuation is kept",
			typ:     Docs,
			subject: "explain `g commit` (finally)",
			want:    "docs: explain `g commit` (finally)",
		},
		{
			name:    "surrounding whitespace is trimmed",
			typ:     Chore,
			subject: "   tidy up   ",
			want:    "chore: tidy up",
		},
		{
			name:    "inner whitespace is kept",
			typ:     Test,
			subject: "cover  the  edge  cases",
			want:    "test: cover  the  edge  cases",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Message(test.typ, test.subject); got != test.want {
				t.Errorf("Message(%q, %q) = %q, want %q", test.typ, test.subject, got, test.want)
			}
		})
	}
}

func TestValidateSubject(t *testing.T) {
	valid := []string{"a", "resolve the thing", "  padded  ", "with `code` and : colons"}
	for _, subject := range valid {
		t.Run("valid/"+subject, func(t *testing.T) {
			if err := ValidateSubject(subject); err != nil {
				t.Errorf("ValidateSubject(%q) error = %v, want nil", subject, err)
			}
		})
	}

	invalid := []struct {
		name    string
		subject string
		want    string
	}{
		{name: "empty", subject: "", want: "empty"},
		{name: "spaces", subject: "   ", want: "empty"},
		{name: "tab", subject: "\t", want: "empty"},
		{name: "newline", subject: "first\nsecond", want: "single line"},
		{name: "carriage return", subject: "first\rsecond", want: "single line"},
	}

	for _, test := range invalid {
		t.Run("invalid/"+test.name, func(t *testing.T) {
			err := ValidateSubject(test.subject)
			if err == nil {
				t.Fatalf("ValidateSubject(%q) error = nil, want an error", test.subject)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("ValidateSubject(%q) error = %v, want it to contain %q", test.subject, err, test.want)
			}
		})
	}
}

func TestTypes(t *testing.T) {
	want := []Type{Feature, Fix, UI, Refactor, Test, Docs, Chore, Continuous, Performance}

	if got := Types(); !reflect.DeepEqual(got, want) {
		t.Errorf("Types() = %v, want %v", got, want)
	}
}

// The returned slice is a copy, so a caller cannot reshape the accepted set for
// everyone else.
func TestTypesReturnsACopy(t *testing.T) {
	first := Types()
	first[0] = "mutated"

	if got := Types()[0]; got != Feature {
		t.Errorf("Types()[0] = %q after mutating a previous result, want %q", got, Feature)
	}
}
