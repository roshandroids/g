# g

A personal Git and GitHub workflow assistant.

`g` is a small command line tool for the Git sequences I repeat every day. It is
not a reimplementation of Git: it wraps the installed `git` executable, keeps the
output short, and stays out of the way. Native `git` remains the escape hatch for
anything `g` does not do.

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

| Command    | Description                                          |
| ---------- | ---------------------------------------------------- |
| `g status` | Show a concise summary of the repository state       |
| `g help`   | Show the full command reference (`g --help` works too) |

Planned, declared in the command surface but intentionally not implemented yet:

| Command                       | Milestone | Intent                                          |
| ----------------------------- | --------- | ----------------------------------------------- |
| `g new <ticket> [description]`| v0.2      | Create a branch for a new piece of work          |
| `g switch <branch>`           | v0.2      | Switch to an existing branch                     |
| `g sync`                      | v0.2      | Update the current branch from upstream/base     |
| `g commit <type> <message>`   | v0.2      | Stage everything and record a commit             |
| `g push`                      | v0.2      | Push the current branch to its upstream          |
| `g undo`                      | v0.3      | Undo the latest commit while keeping its changes |
| `g clean`                     | v0.3      | Delete local branches that are already merged    |

Invoking a planned command exits with an error explaining that it is not
implemented; it never pretends to succeed.

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
internal/workflow UI-independent operations: `g status` lives here
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
   `status --porcelain -z`).
2. **Presentation is separate from logic.** `internal/output` turns workflow
   results (and nothing else) into text. The CLI holds no Git logic, so a future
   GUI can call exactly the same workflow layer.

`workflow` depends on a small `Repository` interface rather than the git client
itself, so workflow behaviour is tested with a fake and no processes are spawned.
Process execution is funnelled through a single `process.Runner` seam.

## Safety philosophy

`g` must never silently discard work.

- Destructive operations require explicit confirmation: `git reset --hard`,
  branch deletion, `git stash clear`, force pushes, and anything else that throws
  away uncommitted work.
- `git push --force-with-lease` is preferred over `--force`, and `g` never
  force-pushes on its own.
- `internal/config` defaults `confirmDestructive` to `true`.

Commands that discard work do not exist yet, so no confirmation machinery exists
yet either. It will be added with the first command that needs it (`g undo`,
`g clean`) rather than speculatively.

## Configuration

The configuration file is optional and deliberately minimal. It lives in the
user configuration directory (`os.UserConfigDir()`), under `g/config.json`:

- macOS: `~/Library/Application Support/g/config.json`
- Linux: `~/.config/g/config.json`

A missing file means the defaults are used:

```json
{
  "defaultBaseBranch": "main",
  "confirmDestructive": true
}
```

Only the fields present in the file override the defaults, and unknown fields are
ignored. A file that cannot be read or parsed is reported on stderr and `g`
continues with the defaults, so a typo cannot lock you out of read-only commands.
No command consumes these settings yet; they are the foundation for v0.2.

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
upstream tracking, ahead/behind counts, and the not-a-repository case. They skip
themselves in short mode and when `git` is missing.

## Roadmap

**V0.1** — project foundation, Git process abstraction, repository state, `g status`

**V0.2** — `g new`, `g switch`, `g sync`, `g commit`, `g push`

**V0.3** — `g undo`, `g clean`, GitHub CLI integration, PR workflows

**V0.4+** — configuration, improved interactive workflows, desktop GUI, Wails
integration

## License

MIT. See [LICENSE](LICENSE).
