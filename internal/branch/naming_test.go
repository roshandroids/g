package branch

import (
	"strings"
	"testing"
)

func TestPolicyName(t *testing.T) {
	tests := []struct {
		name        string
		policy      Policy
		key         string
		description string
		want        string
	}{
		{
			name:        "the documented example",
			policy:      DefaultPolicy(),
			key:         "HCM-37538",
			description: "applicant id update",
			want:        "HCM-37538-applicant-id-update",
		},
		{
			name:        "without a description",
			policy:      DefaultPolicy(),
			key:         "HCM-37538",
			description: "",
			want:        "HCM-37538",
		},
		{
			name:        "description that is only whitespace",
			policy:      DefaultPolicy(),
			key:         "HCM-37538",
			description: "   \t  ",
			want:        "HCM-37538",
		},
		{
			name:        "description that is only punctuation",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "!!! ??? ...",
			want:        "HCM-1",
		},
		{
			name:        "repeated spaces collapse into one separator",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "applicant     id    update",
			want:        "HCM-1-applicant-id-update",
		},
		{
			name:        "surrounding punctuation is trimmed",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "  -- applicant id update --  ",
			want:        "HCM-1-applicant-id-update",
		},
		{
			name:        "punctuation inside words is replaced",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "applicant's id (final)",
			want:        "HCM-1-applicant-s-id-final",
		},
		{
			name:        "already slugged description is left alone",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "applicant-id-update",
			want:        "HCM-1-applicant-id-update",
		},
		{
			name:        "upper case description is lowered",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "Applicant ID Update",
			want:        "HCM-1-applicant-id-update",
		},
		{
			name:        "digits are kept",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "fix issue 2 of 12",
			want:        "HCM-1-fix-issue-2-of-12",
		},
		{
			name:        "lower case issue key is normalized",
			policy:      DefaultPolicy(),
			key:         "hcm-37538",
			description: "applicant id update",
			want:        "HCM-37538-applicant-id-update",
		},
		{
			name:        "surrounding whitespace in the key is ignored",
			policy:      DefaultPolicy(),
			key:         "  HCM-37538  ",
			description: "applicant id update",
			want:        "HCM-37538-applicant-id-update",
		},
		{
			name:        "another project key is not special cased",
			policy:      DefaultPolicy(),
			key:         "NEXUS-7",
			description: "wire up the thing",
			want:        "NEXUS-7-wire-up-the-thing",
		},
		{
			name:        "a project key containing digits",
			policy:      DefaultPolicy(),
			key:         "AB12-3",
			description: "work",
			want:        "AB12-3-work",
		},
		{
			name:        "underscore separator",
			policy:      Policy{Separator: "_"},
			key:         "HCM-1",
			description: "applicant id update",
			want:        "HCM-1_applicant_id_update",
		},
		{
			name:        "slash separator",
			policy:      Policy{Separator: "/"},
			key:         "HCM-1",
			description: "applicant id update",
			want:        "HCM-1/applicant/id/update",
		},
		{
			name:        "empty separator falls back to the default",
			policy:      Policy{},
			key:         "HCM-1",
			description: "applicant id update",
			want:        "HCM-1-applicant-id-update",
		},
		{
			name:        "non-ascii description characters become separators",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "caf\u00e9 update",
			want:        "HCM-1-caf-update",
		},
		{
			name:        "name exactly at the length limit is untouched",
			policy:      Policy{Separator: "-", MaxLength: 29},
			key:         "HCM-37538",
			description: "applicant id update",
			want:        "HCM-37538-applicant-id-update",
		},
		{
			name:        "description that looks like a slug",
			policy:      DefaultPolicy(),
			key:         "HCM-1",
			description: "fix: the applicant/id mapping",
			want:        "HCM-1-fix-the-applicant-id-mapping",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.policy.Name(test.key, test.description)
			if err != nil {
				t.Fatalf("Name(%q, %q) error = %v, want nil", test.key, test.description, err)
			}
			if got != test.want {
				t.Errorf("Name(%q, %q) = %q, want %q", test.key, test.description, got, test.want)
			}
			if err := Validate(got); err != nil {
				t.Errorf("Name(%q, %q) = %q, which is not a valid branch name: %v", test.key, test.description, got, err)
			}
		})
	}
}

func TestPolicyNameTruncates(t *testing.T) {
	tests := []struct {
		name   string
		policy Policy
		want   string
	}{
		{
			name:   "drops the trailing word that does not fit",
			policy: Policy{Separator: "-", MaxLength: 15},
			want:   "HCM-37538",
		},
		{
			name:   "keeps the words that fit",
			policy: Policy{Separator: "-", MaxLength: 20},
			want:   "HCM-37538-applicant",
		},
		{
			name:   "a limit shorter than the key is impossible",
			policy: Policy{Separator: "-", MaxLength: 4},
			want:   "",
		},
		{
			name:   "a limit equal to the key leaves the key alone",
			policy: Policy{Separator: "-", MaxLength: 9},
			want:   "HCM-37538",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.policy.Name("HCM-37538", "applicant id update")
			if test.want == "" {
				if err == nil {
					t.Fatalf("Name() = %q, want an error for an impossible limit", got)
				}
				if !strings.Contains(err.Error(), "longer than") {
					t.Errorf("Name() error = %v, want it to explain the length limit", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Name() error = %v, want nil", err)
			}
			if got != test.want {
				t.Errorf("Name() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPolicyNameRejectsBadInput(t *testing.T) {
	tests := []struct {
		name        string
		policy      Policy
		key         string
		description string
		want        string
	}{
		{name: "empty key", policy: DefaultPolicy(), key: "", want: "invalid issue key"},
		{name: "key without a number", policy: DefaultPolicy(), key: "HCM", want: "invalid issue key"},
		{name: "key without a project", policy: DefaultPolicy(), key: "-1", want: "invalid issue key"},
		{name: "key with a space", policy: DefaultPolicy(), key: "HC M-1", want: "invalid issue key"},
		{name: "key with zero as the number", policy: DefaultPolicy(), key: "HCM-0", want: "invalid issue key"},
		{name: "key with a slug", policy: DefaultPolicy(), key: "HCM-1-extra", want: "invalid issue key"},
		{name: "key with a slash", policy: DefaultPolicy(), key: "HCM/1", want: "invalid issue key"},
		{
			name:   "separator that git would reject",
			policy: Policy{Separator: ":"},
			key:    "HCM-1",
			want:   "invalid branch name separator",
		},
		{
			name:   "separator that could build a range",
			policy: Policy{Separator: ".."},
			key:    "HCM-1",
			want:   "invalid branch name separator",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.policy.Name(test.key, test.description)
			if err == nil {
				t.Fatalf("Name(%q, %q) = %q, want an error", test.key, test.description, got)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("Name(%q, %q) error = %v, want it to contain %q", test.key, test.description, err, test.want)
			}
		})
	}
}

func TestIssueKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
		ok   bool
	}{
		{key: "HCM-37538", want: "HCM-37538", ok: true},
		{key: "hcm-37538", want: "HCM-37538", ok: true},
		{key: "  HCM-1  ", want: "HCM-1", ok: true},
		{key: "A-1", want: "A-1", ok: true},
		{key: "AB12-9", want: "AB12-9", ok: true},
		{key: "CMIC-42", want: "CMIC-42", ok: true},
		{key: "", ok: false},
		{key: "   ", ok: false},
		{key: "HCM", ok: false},
		{key: "HCM-", ok: false},
		{key: "-1", ok: false},
		{key: "1-2", ok: false},
		{key: "HCM-01", ok: false},
		{key: "HCM-1a", ok: false},
		{key: "HC-M-1", ok: false},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			got, err := IssueKey(test.key)
			if !test.ok {
				if err == nil {
					t.Fatalf("IssueKey(%q) = %q, want an error", test.key, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("IssueKey(%q) error = %v, want nil", test.key, err)
			}
			if got != test.want {
				t.Errorf("IssueKey(%q) = %q, want %q", test.key, got, test.want)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		separator string
		want      string
	}{
		{name: "simple words", text: "applicant id update", separator: "-", want: "applicant-id-update"},
		{name: "empty", text: "", separator: "-", want: ""},
		{name: "only separators", text: "---", separator: "-", want: ""},
		{name: "leading and trailing", text: "  hi  ", separator: "-", want: "hi"},
		{name: "no duplicate separators", text: "a---b", separator: "-", want: "a-b"},
		{name: "mixed separators", text: "a - _ b", separator: "-", want: "a-b"},
		{name: "underscore output", text: "a b", separator: "_", want: "a_b"},
		{name: "slash output", text: "a b", separator: "/", want: "a/b"},
		{name: "input already using the separator", text: "a/b", separator: "/", want: "a/b"},
		{name: "control characters", text: "a\nb\tc", separator: "-", want: "a-b-c"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Slug(test.text, test.separator); got != test.want {
				t.Errorf("Slug(%q, %q) = %q, want %q", test.text, test.separator, got, test.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	valid := []string{
		"main",
		"HCM-37538-applicant-id-update",
		"feature/branch-workflows",
		"a/b/c",
		"release-1.2",
		"v0.2.0",
		"fix_underscore",
		"A",
	}
	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if err := Validate(name); err != nil {
				t.Errorf("Validate(%q) error = %v, want nil", name, err)
			}
		})
	}

	invalid := []struct {
		name string
		want string
	}{
		{name: "", want: "empty"},
		{name: "@", want: "invalid branch name"},
		{name: "-leading", want: "start with"},
		{name: "/leading", want: "start or end"},
		{name: "trailing/", want: "start or end"},
		{name: "trailing.", want: "end with"},
		{name: "a..b", want: ".."},
		{name: "a@{b", want: "@{"},
		{name: "a//b", want: "//"},
		{name: "a b", want: "must not contain"},
		{name: "a~b", want: "must not contain"},
		{name: "a^b", want: "must not contain"},
		{name: "a:b", want: "must not contain"},
		{name: "a?b", want: "must not contain"},
		{name: "a*b", want: "must not contain"},
		{name: "a[b", want: "must not contain"},
		{name: "a\\b", want: "must not contain"},
		{name: "a\x7fb", want: "control characters"},
		{name: ".hidden", want: "start with"},
		{name: "a/.hidden", want: "start with"},
		{name: "a.lock", want: ".lock"},
		{name: "a/b.lock", want: ".lock"},
	}

	for _, test := range invalid {
		t.Run("invalid/"+test.name, func(t *testing.T) {
			err := Validate(test.name)
			if err == nil {
				t.Fatalf("Validate(%q) error = nil, want an error", test.name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("Validate(%q) error = %v, want it to contain %q", test.name, err, test.want)
			}
		})
	}
}

// The issue key decides how long a branch name is at minimum, so a short key
// must not make a long description overflow the limit.
func TestPolicyNameStaysWithinTheLimit(t *testing.T) {
	policy := Policy{Separator: "-", MaxLength: 30}

	name, err := policy.Name("HCM-1", "a very long description that will not fit in the branch name")
	if err != nil {
		t.Fatalf("Name() error = %v, want nil", err)
	}
	if len(name) > policy.MaxLength {
		t.Errorf("Name() = %q (%d characters), want at most %d", name, len(name), policy.MaxLength)
	}
	if !strings.HasPrefix(name, "HCM-1") {
		t.Errorf("Name() = %q, want it to preserve the issue key", name)
	}
}

// The separator is a policy knob, so the default must remain what the
// documented example uses.
func TestDefaultPolicy(t *testing.T) {
	if got := DefaultPolicy().Separator; got != DefaultSeparator {
		t.Errorf("DefaultPolicy().Separator = %q, want %q", got, DefaultSeparator)
	}
	if got := DefaultPolicy().MaxLength; got != 0 {
		t.Errorf("DefaultPolicy().MaxLength = %d, want 0 for no limit", got)
	}
}
