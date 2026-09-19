# g

A personal Git and GitHub workflow assistant.

`g` is a small command line tool for the Git sequences I repeat every day. It is
not a reimplementation of Git: it wraps the installed `git` executable, keeps the
output short, and stays out of the way. Native `git` remains the escape hatch for
anything `g` does not do.

Starting a piece of work and recording it:

```console
$ g new HCM-37538 "applicant id update"
Created branch HCM-37538-applicant-id-update from main

$ git add lib/applicant/history.dart
$ g commit fix "resolve applicant history issue"
Committed:
  modified     lib/applicant/history.dart

fix: resolve applicant history issue

$ g push
Pushed HCM-37538-applicant-id-update to origin/HCM-37538-applicant-id-update

$ g switch main
Switched to branch main (from HCM-37538-applicant-id-update)
```

Understanding where you are:

```console
$ g status
Repository: nexus_applicant
Root:       /Users/me/Projects/nexus_applicant
Branch:     HCM-37538-applicant-id-update
Upstream:   origin/HCM-37538-applicant-id-update

Working tree: dirty

Staged:
  added        lib/applicant/history.dart

Unstaged:
  modified     lib/applicant/history.dart

Untracked:
  test/applicant/history_test.dart

Ahead:  2
Behind: 0
```

## Requirements

- Go 1.27 or newer (only to build)
- Git on `PATH` (required at run time)
- The GitHub CLI `gh` (optional, unused so far)

## Installation and development

```sh
git clone <this repository> ~/Projects/g
cd ~/Projects/g

go run ./cmd/g          # run from source
go build -o g ./cmd/g   # build a local binary
go install ./cmd/g      # install `g` into $(go env GOPATH)/bin
```

To call `g` from anywhere, make sure `$(go env GOPATH)/bin` is on your `PATH`.

## Commands

Implemented:

| Command                       | Description                                          |
| ----------------------------- | ---------------------------------------------------- |
| `g status`                    | Show a concise summary of the repository state       |
| `g new <ticket> [description]`| Create a branch for a new piece of work              |
| `g switch <branch>`           | Switch to an existing branch                         |
| `g commit <type> <message>`   | Commit what is staged                                |
| `g push [--force]`            | Push the current branch to its upstream              |
| `g help`                      | Show the full command reference (`g --help` works too) |

Planned, declared in the command surface but intentionally not implemented yet:

| Command                       | Milestone | Intent                                          |
| ----------------------------- | --------- | ----------------------------------------------- |
| `g sync`                      | v0.4      | Update the current branch from upstream/base     |
| `g undo`                      | v0.5      | Undo the latest commit while keeping its changes |
| `g clean`                     | v0.5      | Delete local branches that are already merged    |

Invoking a planned command exits with an error explaining that it is not
implemented; it never pretends to succeed.

### `g status`

Shows the state the other workflows actually act on, and nothing else:

- the repository name and the **work tree root**, so a shell `cd` has somewhere
  to go;
- the **current branch**, or `detached at <commit>` when HEAD is not on one;
- the **upstream** the branch tracks, or `none`;
- the **paused operation**, if a rebase, merge, cherry-pick or revert is part way
  through;
- whether the working tree is **clean or dirty**, stated outright rather than
  left to be inferred from whether a list follows;
- the **staged**, **unstaged**, **untracked** and **conflicted** paths;
- how far the branch has **diverged** from its upstream.

This is deliberately not a replacement for `git status`. The full output already
exists and is better at being exhaustive — it lists hints, ignored files and
directory rollups that a summary has no business reproducing. `g status` answers
the narrower question of what `g new`, `g switch`, `g commit` and `g push` are
about to do.

The operation line exists because of a trap worth naming. While a rebase is
paused, `git branch --show-current` is **empty** — the branch is detached for the
duration — so a summary built only on the branch name would report a detached
HEAD and leave you to work out why. The two facts are shown together:

```console
$ g status
Repository: nexus_applicant
Root:       /Users/me/Projects/nexus_applicant
Branch:     detached at 81f45c8
Upstream:   none
Operation:  rebase in progress

Working tree: dirty

Conflicted:
  lib/applicant/history.dart
```

### `g new <ticket> [description]`

Creates a branch named after the work and switches to it. The issue key is
preserved in upper case and the description is reduced to a slug, so
`g new hcm-37538 "applicant id UPDATE!!"` produces
`HCM-37538-applicant-id-update`. Spaces, punctuation, repeated separators and
characters that are invalid in a Git ref all collapse into single separators, and
a description that reduces to nothing leaves the issue key on its own.

The branch is created from the configured `defaultBaseBranch`, falling back to
`main` and then `master`, so a repository works with no configuration whichever
name its trunk uses. **The base branch is used exactly as it stands locally.**
Nothing is fetched, pulled or rebased: updating the base is a separate decision
with its own safety story, and doing it implicitly here would make the starting
point of a branch depend on network state.

`g new` refuses, rather than guessing, when the repository is not on a branch
(HEAD is detached) or when another Git operation is paused. A rebase is worth
calling out: while one is in progress HEAD is detached and `git branch
--show-current` is empty, so the naive error would be "detached HEAD". `g`
reports the rebase instead, because creating a branch mid-rebase would leave the
rebase with nowhere to finish.

Uncommitted changes are **not** an error. `git switch -c` carries them across to
the new branch without discarding anything, so `g new` says so and continues
rather than blocking on a change that git handles safely.

### `g switch <branch>`

Checks out an existing local branch, matching the name exactly. It does not
guess: creating work on the wrong branch is far harder to notice than an error,
so an unknown branch fails and lists the branches that do exist. Names
containing `/` work as-is.

Switching is done with `git switch`, and `g` never passes `--force` or
`--discard-changes`. When the working tree is dirty the switch may still be
safe, so `g` notes it and lets git decide; if changes would be overwritten git
refuses and that refusal is reported unchanged. `g` will not discard work to
make a switch succeed.

### `g commit <type> <message>`

```console
$ g commit fix "resolve applicant history issue"
Committed:
  modified     lib/applicant/history.dart

fix: resolve applicant history issue
```

The type must be one of `feat`, `fix`, `ui`, `refactor`, `test`, `docs`,
`chore`, `ci` or `perf`; it is lower-cased and joined to the message with `: `.

**Only what is already staged is committed.** `g` never runs `git add` for you.
"Commit everything" is convenient right up to the moment it sweeps up a stray
file, and there is no undo for a commit that should never have existed, so the
index is treated as your explicit decision about what belongs in this commit. If
nothing is staged, `g commit` fails and says how many files are waiting and how
to stage them, rather than guessing.

**The message is yours.** `g` adds the type and trims surrounding whitespace; it
never rewords, re-cases or generates the text. A message that is empty or spans
more than one line is rejected rather than mangled into shape.

Only `git commit -m` is run, so everything else about committing is unchanged:
hooks run, hooks can reject the commit, and the index is left alone when they do.
Unlike the branch workflows, `g commit` does not require an idle repository —
committing is how a paused merge is finished. A commit made on a detached HEAD is
allowed, because git allows it, but it is reported, because such a commit is
reachable only from `HEAD`.

### `g push [--force]`

```console
$ g push
Pushed HCM-37538-applicant-id-update to origin/HCM-37538-applicant-id-update
```

A branch that already tracks a remote is pushed with a plain `git push`, naming
nothing, because git already knows which remote and branch are meant.

A branch that tracks **nothing** is published with `git push -u <remote> HEAD`.
`origin` is preferred when it exists; a sole remote of any other name is used as
is. Several remotes and no `origin` is refused rather than guessed, because the
wrong remote is harder to notice than an error. The result reports that the
upstream was set:

```console
$ g push
Pushed HCM-37538-applicant-id-update and set upstream to origin/HCM-37538-applicant-id-update
```

**`--force` is not a shorthand for "push harder".** It is a separate, explicit
mode because it is the one operation here that can destroy somebody else's work,
and it is never part of a normal `g push`. It always asks first:

```console
$ g push --force
Force-push with --force-with-lease, replacing the remote branch if it has moved? [y/N] y
Force-pushed HCM-37538-applicant-id-update to origin/HCM-37538-applicant-id-update (--force-with-lease)
```

The underlying command is always `git push --force-with-lease`, which still
refuses to overwrite a remote branch that has moved since your last fetch. A
plain `--force` is not reachable at all: no combination of options produces it,
which is asserted by a test that enumerates them.

Force-pushing a branch with no upstream is refused outright — there would be no
remote branch to compare against, so it could only be a blind overwrite. With no
terminal to ask on — a script, a pipe, a future GUI — a force confirmation is
treated as no.

A rejected push is reported with git's own explanation and left there. When the
remote has moved on, the answer is to fetch and integrate, and that is your call
rather than something `g` gets to decide.

### Exit codes

| Code | Meaning                                                                 |
| ---- | ----------------------------------------------------------------------- |
| 0    | Success                                                                 |
| 1    | Runtime error, including running outside a Git repository               |
| 2    | Usage error, such as an unknown command, flag, or unexpected argument   |

## Architecture

```
cmd/g            entry point: wires the layers together
internal/command command definitions, dispatch, and help
internal/workflow UI-independent operations: status, branch, commit and push work
internal/branch  branch naming policy: slugging and ref validation
internal/commit  commit message policy: types and subject validation
internal/prompt  the only place that asks the user a question
internal/git     git client: runs git and parses its machine-readable output
internal/github  boundary for `gh` based GitHub operations (availability only)
internal/config  optional user configuration, with defaults
internal/output  all terminal rendering
internal/process the single seam for executing external programs
tests/           integration tests against real Git repositories
```

The dependency chain is one-directional:

```
command (CLI)
    ↓
workflow (application)
    ↓
git · github (services)
    ↓
process (process runner)
    ↓
installed git / gh executables
```

Two rules keep the design honest:

1. **Nothing reads `.git` internals.** Every fact comes from running `git` and
   parsing documented output (`rev-parse`, `symbolic-ref`, `rev-list`,
   `for-each-ref`, `status --porcelain -z`). Even the paused-operation state —
   rebase, merge, cherry-pick, revert — is detected by asking git to resolve the
   marker refs it maintains (`REBASE_HEAD`, `MERGE_HEAD`, `CHERRY_PICK_HEAD`,
   `REVERT_HEAD`), so g never has to assume a layout for the Git directory.
2. **Presentation is separate from logic.** `internal/output` turns workflow
   results (and nothing else) into text. Workflow results carry facts, not
   sentences, so the same call renders as terminal text today and as a GUI field
   later. The CLI holds no Git logic, so a future GUI can call exactly the same
   workflow layer.

`workflow` depends on a small `Repository` interface rather than the git client
itself, so workflow behaviour is tested with a fake and no processes are spawned.
Process execution is funnelled through a single `process.Runner` seam.

Branch naming lives in `internal/branch` and the commit message convention in
`internal/commit`. Both are pure policies — no processes, no filesystem — so the
rules are testable in isolation and reusable by any front end. The command layer
validates them in order to report a usage error (exit 2), and the workflow layer
validates them again so that every caller gets the same guarantees.

## Safety philosophy

`g` must never silently discard work.

- Destructive operations require explicit confirmation: `git reset --hard`,
  branch deletion, `git stash clear`, force pushes, and anything else that throws
  away uncommitted work.
- `git push --force-with-lease` is preferred over `--force`, and `g` never
  force-pushes on its own.
- `internal/config` defaults `confirmDestructive` to `true`.

In practice this means the implemented commands are deliberately conservative about
what they will do on your behalf:

- `g new` and `g switch` never pass `--force` or `--discard-changes`. When a
  dirty working tree means a switch cannot be done safely, git refuses and `g`
  reports that refusal rather than forcing it through.
- `g switch` matches branch names exactly. Guessing at a branch that does not
  exist risks putting work on the wrong branch, which is much harder to notice
  than an error.
- `g new` and `g switch` never touch the network. `g new` uses the base branch as
  it stands locally, so starting work never depends on what a remote happens to
  be serving.
- `g commit` never stages files. It commits the index and nothing else, so it
  cannot sweep an unrelated file into a commit that has no undo.
- `g commit` never rewrites your message and never passes `--no-verify`, so your
  hooks keep the power to reject a commit.
- `g push` never force-pushes on its own. `--force` is a separate, explicit mode
  that asks first, and even then only `--force-with-lease` is used.
- `g push` publishes a branch with no upstream via `git push -u`, so you never
  have to type the remote and branch name. Several remotes and no `origin` is
  refused rather than guessed.

Confirmation lives at the edge of the program. Workflows expose a flag such as
"force this push" and treat it as the caller's assertion that the user agreed;
`internal/prompt` is the only place that actually asks. A missing terminal counts
as a refusal, so a command that needs an answer fails closed rather than assuming
consent.

`confirmDestructive` is still not consulted by any command. It is documented as
governing operations that discard *local* work — commits and uncommitted changes
— and none of the commands so far do that. Force push overwrites remote history
rather than local work, which is why it always asks instead of deferring to the
setting. `g undo` and `g clean` are where it will apply.

## Configuration

The configuration file is optional and deliberately minimal. It lives in the
user configuration directory (`os.UserConfigDir()`), under `g/config.json`:

- macOS: `~/Library/Application Support/g/config.json`
- Linux: `~/.config/g/config.json`

A missing file means the defaults are used:

```json
{
  "defaultBaseBranch": "main",
  "confirmDestructive": true,
  "branchNaming": {
    "separator": "-",
    "maxLength": 0
  }
}
```

Only the fields present in the file override the defaults, and unknown fields are
ignored. A file that cannot be read or parsed is reported on stderr and `g`
continues with the defaults, so a typo cannot lock you out of read-only commands.

- **`defaultBaseBranch`** is the branch `g new` starts work from, tried before
  the `main`/`master` fallbacks. Set it to `develop` in a repository whose trunk
  is called something else.
- **`confirmDestructive`** requires explicit confirmation before an operation
  discards commits or uncommitted work.
- **`branchNaming.separator`** joins the issue key and the description slug. One
  of `-`, `_` or `/`; any other value is rejected when a branch is created,
  rather than building a name git would refuse.
- **`branchNaming.maxLength`** caps the length of a generated branch name. `0`
  means no limit; otherwise whole trailing words are dropped to fit, and the
  issue key is never shortened.

`defaultBaseBranch`, `branchNaming.separator` and `branchNaming.maxLength` are
consumed by `g new`. `confirmDestructive` is not consumed yet: it governs
operations that discard *local* work (`g undo`, `g clean`), not force push.

## Testing

```sh
gofmt -l .        # formatting
go vet ./...      # static checks
go test ./...     # unit tests plus integration tests
go test -short ./...
```

Unit tests never execute Git: the git client is driven by a scripted process
runner, and the workflow layer by a fake repository. Everything covered by
mocks lives in `internal/*`.

`tests/` holds integration tests that do run real Git against throwaway
repositories in `t.TempDir()`: repository detection, clean and dirty trees,
staged versus unstaged changes, untracked files, conflicts, detached HEAD,
upstream tracking, ahead/behind counts, branch creation and switching, commit
recording, pushing and force-with-lease refusals, the paused-operation states,
and the not-a-repository case. They skip themselves in
short mode and when `git` is missing.

## Roadmap

**V0.1 Foundation** — project foundation, Git process abstraction, repository
state, `g status`

**V0.2 Branch workflow** — `g new`, `g switch`

**V0.3 Commit & Push** — `g commit`, `g push`

**V0.4 Safe Synchronization** — `g sync`

**V0.5 Recovery** — `g undo`, `g clean`

**V0.6 GitHub workflows** — GitHub CLI integration, PR workflows

**V1.0 Distribution** — packaging, installation, desktop GUI / Wails

## License

MIT. See [LICENSE](LICENSE).
