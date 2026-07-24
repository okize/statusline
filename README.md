# Claude Code Statusline

[![CI](https://github.com/okize/statusline/actions/workflows/ci.yml/badge.svg)](https://github.com/okize/statusline/actions/workflows/ci.yml)

Custom status line for Claude Code that displays model info, context window usage, rate limits, and git status directly in the terminal.

## What it displays

**Line 1:** Model name with bracketed reasoning effort (`Fable 5 [xhigh]`, omitted when the model doesn't support effort) | rate limit usage (5h/7d with reset times) | context window gradient bar with bracketed percentage (`[42%]`) | cache hit rate and output tokens of the most recent API call. Before the first API call renders as a skeleton with `--` placeholders.

**Line 2:** Current directory, or worktree tag (`[wt:name]`) in place of the directory when inside a git worktree | git branch, ahead/behind counts vs upstream (`↑N ↓M`, only when non-zero), and last commit time

**Line 3:** Pull request badge (`PR #N (state)`, clickable, only when the branch has an open PR) | Shortcut ticket link (if branch matches `sc-#####`) | staged/unstaged file counts with insertion/deletion stats

When Claude Code provides the terminal width (`COLUMNS`, v2.1.153+), long directory paths and branch names are truncated with a middle ellipsis (`…`).

## Files

A single Go binary: a thin `main.go` that calls into the `internal/statusline` package.

| File | Purpose |
|------|---------|
| `main.go` | Entry point (`package main`). Reads stdin/args and `COLUMNS`, calls `statusline.Render`/`RenderGit`, prints the result. |
| `internal/statusline/statusline.go` | Exported API: `Render` (full status) and `RenderGit` (the two git lines). |
| `internal/statusline/input.go`, `types.go` | JSON decode and the optional/nullable-field defaulting. |
| `internal/statusline/render.go` | Line 1 (model, effort, rate limits, context bar, cache, out), the location/worktree tag, and the PR badge. |
| `internal/statusline/git.go` | Git helper: branch, ahead/behind, sync age, change stats. Shells out to `git`. |
| `internal/statusline/ticket.go` | Ticket-tracker detection (currently Shortcut) from branch names. |
| `internal/statusline/ansi.go` | ANSI palette, context gradient, and display helpers (`truncateMiddle`, token/reset formatting). |
| `internal/statusline/*_test.go` | Test suite. Run with `go test ./...`; exits non-zero on failure. |

## Docs

Official documentation: https://code.claude.com/docs/en/statusline

## Setup

Clone this repo and build the binary:

```bash
make build   # or: go build -o statusline .
```

The binary is gitignored — each machine builds its own, so it stays portable
across architectures and OSes. If this repo is managed by your dotfiles, add
`make -C ~/src/statusline build` to your bootstrap step.

Then add the following to your Claude Code `settings.json` (user or project
level), pointing `command` at the built binary:

```json
{
  "statusLine": {
    "type": "command",
    "command": "~/src/statusline/statusline"
  }
}
```

Claude Code pipes a JSON object to stdin containing session context (model,
workspace, context window usage, rate limits). The binary parses it and renders
the status line.

## Configuration

Shortcut ticket links (for branches matching `sc-#####`) need an org slug. Set
it via an environment variable in your shell profile; without it, no ticket link
is shown:

```bash
export STATUSLINE_SHORTCUT_ORG=your-org   # https://app.shortcut.com/your-org/...
```

## Dependencies

- Go 1.24+ (build-time only)
- `git` (repository status; invoked at runtime)

## Development

```bash
make test    # go test ./...
make vet     # go vet ./...
make lint    # golangci-lint run (install: https://golangci-lint.run/welcome/install/)
```

GitHub Actions runs the tests, `go vet`, gofmt, and golangci-lint on every push
and PR (`.github/workflows/ci.yml`).

### Preview locally

Render the status line by piping in the sample payload:

```bash
go run . < examples/stdin-payload-example.json
```

`examples/stdin-payload-example.json` is a full example of the stdin contract.
Edit its values to preview different states — `context_window.used_percentage`
drives the context bar (the committed sample sets it to `100` for a full bar),
`rate_limits` the 5h/7d segments, `pr` the PR badge. The location and git lines
read the real repository at `workspace.current_dir`, so point that at an actual
checkout to see branch, ahead/behind, and ticket output.
