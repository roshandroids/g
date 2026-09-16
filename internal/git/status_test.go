package git

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []FileChange
	}{
		{
			name:  "no output",
			input: "",
			want:  nil,
		},
		{
			name:  "staged modification",
			input: "M  staged.txt\x00",
			want:  []FileChange{{Path: "staged.txt", Index: Modified}},
		},
		{
			name:  "unstaged modification",
			input: " M staged.txt\x00",
			want:  []FileChange{{Path: "staged.txt", Worktree: Modified}},
		},
		{
			name:  "staged and unstaged modification",
			input: "MM both.txt\x00",
			want:  []FileChange{{Path: "both.txt", Index: Modified, Worktree: Modified}},
		},
		{
			name:  "addition deleted in the working tree",
			input: "AD gone.txt\x00",
			want:  []FileChange{{Path: "gone.txt", Index: Added, Worktree: Deleted}},
		},
		{
			name:  "untracked",
			input: "?? new.txt\x00",
			want:  []FileChange{{Path: "new.txt", Untracked: true}},
		},
		{
			name:  "ignored paths are skipped",
			input: "!! vendor/\x00",
			want:  nil,
		},
		{
			name:  "type change",
			input: " T link.txt\x00",
			want:  []FileChange{{Path: "link.txt", Worktree: TypeChanged}},
		},
		{
			name:  "rename carries the original path",
			input: "R  new.txt\x00old.txt\x00",
			want:  []FileChange{{Path: "new.txt", OriginalPath: "old.txt", Index: Renamed}},
		},
		{
			name:  "copy carries the original path",
			input: "C  copy.txt\x00source.txt\x00",
			want:  []FileChange{{Path: "copy.txt", OriginalPath: "source.txt", Index: Copied}},
		},
		{
			name:  "conflict",
			input: "UU conflict.txt\x00",
			want:  []FileChange{{Path: "conflict.txt", Index: Unmerged, Worktree: Unmerged, Unmerged: true}},
		},
		{
			name:  "both added is a conflict",
			input: "AA conflict.txt\x00",
			want:  []FileChange{{Path: "conflict.txt", Index: Unmerged, Worktree: Unmerged, Unmerged: true}},
		},
		{
			name:  "both deleted is a conflict",
			input: "DD conflict.txt\x00",
			want:  []FileChange{{Path: "conflict.txt", Index: Unmerged, Worktree: Unmerged, Unmerged: true}},
		},
		{
			name:  "added by us is a conflict",
			input: "AU conflict.txt\x00",
			want:  []FileChange{{Path: "conflict.txt", Index: Unmerged, Worktree: Unmerged, Unmerged: true}},
		},
		{
			name:  "path containing spaces",
			input: " M my file.txt\x00",
			want:  []FileChange{{Path: "my file.txt", Worktree: Modified}},
		},
		{
			name:  "several entries",
			input: "M  a.txt\x00?? b.txt\x00",
			want:  []FileChange{{Path: "a.txt", Index: Modified}, {Path: "b.txt", Untracked: true}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseStatus(test.input)
			if err != nil {
				t.Fatalf("parseStatus(%q) error = %v, want nil", test.input, err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("parseStatus(%q) = %+v, want %+v", test.input, got, test.want)
			}
		})
	}
}

func TestParseStatusRejectsMalformedOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "entry without a path",
			input: "X\x00",
			want:  "malformed status entry",
		},
		{
			name:  "truncated entry",
			input: "M\x00",
			want:  "malformed status entry",
		},
		{
			name:  "rename without the original path",
			input: "R  new.txt\x00",
			want:  "missing original path",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseStatus(test.input)
			if err == nil {
				t.Fatalf("parseStatus(%q) error = nil, want an error", test.input)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("parseStatus(%q) error = %v, want it to contain %q", test.input, err, test.want)
			}
		})
	}
}
