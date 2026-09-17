package git

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roshandroids/g/internal/process"
)

func TestPushOptionsArgs(t *testing.T) {
	tests := []struct {
		name string
		opts PushOptions
		want string
	}{
		{
			name: "plain push",
			opts: PushOptions{},
			want: "push",
		},
		{
			name: "force with lease",
			opts: PushOptions{ForceWithLease: true},
			want: "push --force-with-lease",
		},
		{
			name: "setting the upstream",
			opts: PushOptions{Remote: "origin", Ref: "HEAD", SetUpstream: true},
			want: "push --set-upstream origin HEAD",
		},
		{
			name: "force with lease and an explicit remote",
			opts: PushOptions{Remote: "origin", Ref: "HEAD", SetUpstream: true, ForceWithLease: true},
			want: "push --force-with-lease --set-upstream origin HEAD",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := strings.Join(test.opts.args(), " "); got != test.want {
				t.Errorf("args() = %q, want %q", got, test.want)
			}
		})
	}
}

// A plain --force overwrites whatever is on the remote, including commits
// somebody else pushed after the last fetch. It must not be reachable through
// any combination of options.
func TestPushOptionsNeverProduceAPlainForce(t *testing.T) {
	booleans := []bool{false, true}

	for _, force := range booleans {
		for _, setUpstream := range booleans {
			for _, remote := range []string{"", "origin"} {
				for _, ref := range []string{"", "HEAD"} {
					opts := PushOptions{
						Remote:         remote,
						Ref:            ref,
						SetUpstream:    setUpstream,
						ForceWithLease: force,
					}

					for _, arg := range opts.args() {
						if arg == "--force" || arg == "-f" {
							t.Fatalf("args() for %+v contains %q, want --force-with-lease only", opts, arg)
						}
					}
				}
			}
		}
	}
}

func TestClientPush(t *testing.T) {
	runner := newScriptedRunner().respond("push", process.Result{})

	if err := NewClient(runner).Push(context.Background(), testRoot, PushOptions{}); err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("Push() made %d invocations, want 1", len(runner.calls))
	}
	call := runner.calls[0]
	if call.String() != "git push" {
		t.Errorf("invocation = %q, want a bare push", call)
	}
	if call.Dir != testRoot {
		t.Errorf("directory = %q, want %q", call.Dir, testRoot)
	}
}

func TestClientPushSetsTheUpstream(t *testing.T) {
	runner := newScriptedRunner().respond("push --set-upstream origin HEAD", process.Result{})

	opts := PushOptions{Remote: "origin", Ref: "HEAD", SetUpstream: true}
	if err := NewClient(runner).Push(context.Background(), testRoot, opts); err != nil {
		t.Fatalf("Push() error = %v, want nil", err)
	}

	if want := "git push --set-upstream origin HEAD"; runner.calls[0].String() != want {
		t.Errorf("invocation = %q, want %q", runner.calls[0], want)
	}
}

// A rejected push has to keep git's explanation, because "the remote has moved"
// and "you are not allowed" need very different responses.
func TestClientPushReportsRejection(t *testing.T) {
	runner := newScriptedRunner().respond("push", process.Result{
		ExitCode: 1,
		Stderr:   " ! [rejected]        main -> main (non-fast-forward)\n",
	})

	err := NewClient(runner).Push(context.Background(), testRoot, PushOptions{})

	var commandErr *CommandError
	if !errors.As(err, &commandErr) {
		t.Fatalf("Push() error = %v, want *CommandError", err)
	}
	if !strings.Contains(commandErr.Stderr, "non-fast-forward") {
		t.Errorf("Stderr = %q, want it to keep git's diagnostic", commandErr.Stderr)
	}
}

func TestClientPushReportsAStaleLease(t *testing.T) {
	runner := newScriptedRunner().respond("push --force-with-lease", process.Result{
		ExitCode: 1,
		Stderr:   " ! [rejected]        main -> main (stale info)\n",
	})

	err := NewClient(runner).Push(context.Background(), testRoot, PushOptions{ForceWithLease: true})
	if err == nil {
		t.Fatal("Push() error = nil, want the stale lease to be reported")
	}
	if !strings.Contains(err.Error(), "stale info") {
		t.Errorf("Push() error = %v, want it to keep git's explanation", err)
	}
}
