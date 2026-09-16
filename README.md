# g

A personal Git and GitHub workflow assistant.

`g` is a small command line tool for the Git sequences I repeat every day. It is
not a reimplementation of Git: it wraps the installed `git` executable, keeps the
output short, and stays out of the way. Native `git` remains the escape hatch for
anything `g` does not do.

Starting a piece of work:

```console
$ g new HCM-37538 "applicant id update"
Created branch HCM-37538-applicant-id-update from main

$ g switch main
Switched to branch main (from HCM-37538-applicant-id-update)
```

Understanding where you are:

```console
$ g status
Repository: nexus_applicant
Branch:     HCM-37538-applicant-id-update
Upstream:   origin/HCM-37538-applicant-id-update

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
| `g help`                      | Show the full command reference (`g --help` works too) |

Planned, declared in the command surface but intentionally not implemented yet:

| Command                       | Milestone | Intent                                          |
| ----------------------------- | --------- | ----------------------------------------------- |
| `g sync`                      | v0.2      | Update the current branch from upstream/base     |
| `g commit <type> <message>`   | v0.2      | Record a commit from the staged changes          |
| `g push`                      | v0.2      | Push the current branch to its upstream          |
| `g undo`                      | v0.3      | Undo the latest commit while keeping its changes |
| `g clean`                     | v0.3      | Delete local branches that are already merged    |

Invoking a planned command exits with an error explaining that it is not
implemented; it never pretends to succeed.

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
internal/workflow UI-independent operations: `g status`, `g new`, `g switch`
internal/branch  branch naming policy: slugging and ref validation
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

Branch naming lives in `internal/branch` as a pure policy: no processes, no
filesystem, so the naming rules are testable in isolation and reusable by any
front end.

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
- Neither command touches the network. `g new` uses the base branch as it stands
  locally, so starting work never depends on what a remote happens to be
  serving.

Commands that discard work do not exist yet, so no confirmation machinery exists
yet either. It will be added with the first command that needs it (`g push
--force`, `g undo`, `g clean`) rather than speculatively.

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
consumed by `g new`. `confirmDestructive` is not consumed yet: it belongs to the
commands that discard work, which do not exist.

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
upstream tracking, ahead/behind counts, branch creation and switching, the
paused-operation states, and the not-a-repository case. They skip themselves in
short mode and when `git` is missing.

## Roadmap

**V0.1** — project foundation, Git process abstraction, repository state,
`g status`

**V0.2** (in progress) — `g new`, `g switch`, `g commit`, `g push`, `g sync`

- `g new` and `g switch` are implemented.
- `g commit` and `g push` are next.
- `g sync` is designed before it is implemented: the states it has to cope with
  are written down first, because turning a stash/checkout/rebase/stash-pop
  sequence into a command is exactly where a workflow tool can lose work.

**V0.3** — `g undo`, `g clean`, GitHub CLI integration, PR workflows

**V0.4+** — improved interactive workflows, desktop GUI, Wails integration

## License

MIT. See [LICENSE](LICENSE).
