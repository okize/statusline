# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A custom multi-line status line for Claude Code, written in Go. A single binary renders a three-line bar showing model, subscription rate limits, context-window usage, working directory, and git status.

This repo is the source of truth. Claude Code runs it via the `statusLine.command` field in `~/.claude/settings.json`, which points at the built binary `~/src/statusline/statusline`. Unlike the previous Bash version, there **is a build step**: run `make build` after changing the code (and in the dotfiles bootstrap) so the binary is up to date.

User-facing docs (what each line shows, color tables, setup) live in `README.md`; this file covers how the code fits together.

## Develop / test

Build and test:

```bash
make build        # go build -o statusline .
make test         # go test ./...
make vet          # go vet ./...
make lint         # golangci-lint run (needs golangci-lint installed)
```

CI (`.github/workflows/ci.yml`) runs gofmt/vet/`go test` and golangci-lint on every push to `main` and every PR. The linter config is `.golangci.yml` (v2 schema, `standard` set plus `misspell`/`unconvert`, gofmt formatter); the golangci-lint version is pinned in the workflow. Keep the tree gofmt-clean and lint-clean — CI fails otherwise.

The test suite (`*_test.go`) covers sync-age clamping and buckets, ahead/behind counts, truncation, the context bar/gradient, rate-limit colors, the skeleton, cache/out, the PR badge, the worktree tag, ticket detection, and the two-line git contract. Git-fixture tests build throwaway repos under `t.TempDir()` via `exec.Command("git", ...)`; no network needed. Add a failing test before changing behavior (TDD).

For manual checks, pipe a sample stdin payload to the binary, exactly as Claude Code invokes it:

```bash
echo '{
  "model": { "display_name": "Opus 4.8" },
  "workspace": { "current_dir": "'"$PWD"'" },
  "context_window": {
    "context_window_size": 1000000,
    "used_percentage": 42,
    "current_usage": {
      "input_tokens": 30000, "output_tokens": 12000,
      "cache_creation_input_tokens": 5000, "cache_read_input_tokens": 200000
    }
  },
  "rate_limits": {
    "five_hour": { "used_percentage": 18, "resets_at": 1788000000 },
    "seven_day": { "used_percentage": 55, "resets_at": 1788400000 }
  }
}' | ./statusline
```

- Test git rendering in isolation: `./statusline git /path/to/repo` (prints its two lines).
- **Byte-for-byte parity is the design constraint** (this is a faithful rewrite of the original Bash, not a redesign). When changing rendering, verify output is unchanged.

Keep runs fast. Claude Code debounces updates at 300ms and **cancels an in-flight run** when a new update arrives, so avoid slow work (network calls, unbounded git operations). The single binary already avoids the old script's many `jq`/`sed`/`grep`/`date` subprocesses; only `git` is still shelled out.

## Architecture

Flow: Claude Code → stdin JSON → `statusline` binary → (shells out to `git`) → stdout (3 lines).

A thin `main` (root) plus the `statusline` implementation package:

- **main.go** (`package main`) — entry point. Reads `COLUMNS`, dispatches the `git <dir>` subcommand vs stdin mode, and calls the exported `statusline.Render` / `statusline.RenderGit`. Holds no rendering logic.
- **internal/statusline/** (`package statusline`) — the renderer and its tests:
  - **statusline.go** — exported API: `Render(data, columns)` (decode + full status) and `RenderGit(cwd, columns)` (the two git lines).
  - **input.go / types.go** — decode the stdin JSON into a struct and apply the defaults the old single `jq` call did. Optional/nullable fields are pointers so absent/null can be distinguished; integer-ish fields are decoded as `float64` for tolerance (see gotchas).
  - **render.go** — `renderMain` (full assembly + exact newline structure), the context bar/gradient, rate-limit segment, cache/out, effort badge, worktree tag, and PR badge.
  - **git.go** — git helper. `renderGitLines(cwd, columns) (line1, line2 string)`: branch, ahead/behind (`rev-list --left-right --count`), sync age, and change stats (`status --porcelain` parsed once, `--shortstat` only when a count > 0). Uses `--no-optional-locks` throughout. In Go the two lines are just two return values — there is no two-process boundary or positional `sed` split to keep in sync.
  - **ticket.go** — ticket-tracker detection (see below).
  - **ansi.go** — ANSI palette constants, the `contextGradient` array, `truncateMiddle`, and small formatters (`formatTokens`, `rateLimitColor`, `formatResetTime`/`resetLayout`).

Output contract (three lines, after a leading blank line):
1. model + `[effort]` (only when the model supports it) | 5h/7d rate limits | context gradient bar + `[N%]` | `Cache:` hit rate and `Out:` tokens (most recent API call)
2. directory, or `[wt:name]` tag in place of the directory inside a worktree | git branch, `↑N ↓M` ahead/behind vs upstream (only when non-zero), relative sync time
3. `PR #N (state)` badge (only when an open PR exists) + ticket link (only if a tracker matches the branch, e.g. Shortcut `sc-#####`) + staged/unstaged stats, or `No pending changes`

## Conventions and gotchas

- **Byte-exact rendering.** ANSI codes are raw escape bytes in string literals (e.g. `"\x1b[38;5;33m"`, `"\a"`), reproducing what the old `echo -e` / `printf '%b'` emitted. The palette constants stay verbatim; the context gradient is the one deliberate exception — redesigned to a 24-bit truecolor ramp (`contextGradient`, emitted via `fgRGB`) and no longer tied to the bash output. OSC 8 hyperlinks (PR badge, ticket link) are written as literal bytes, not via a library.
- **Cross-platform.** Timestamps use Go's `time` package (`time.Unix(...).Local().Format(...)`) instead of BSD `date -r`, so the binary is **not** macOS-only anymore. `formatResetTime` layouts: 5h → `"3:04 PM"`, 7d → `"1/2/06 3:04 PM"`.
- **Intentional divergence from bash: dynamic values are written literally.** Bash's final `echo -e` / `printf '%b'` interpreted backslash escapes *anywhere* in the line, including inside the model name, cwd, branch, worktree name, and URLs. Go writes those verbatim. This is safer — a branch or path containing a literal `\n` cannot inject a newline and break the line structure — and is the one deliberate behavioral difference from the original.
- **Numeric tolerance.** `context_window_size`, `current_usage.*`, `pr.number`, and `resets_at` are decoded as `float64` (then truncated/converted). A value that arrives as `200000.0` rather than `200000` still renders instead of failing the whole decode and blanking the line — mirroring jq's tolerance. Truncation matches bash integer arithmetic (`int(f)`).
- **stdin JSON contract**: full field list at https://code.claude.com/docs/en/statusline. Conditionally-absent fields are guarded via pointer/nil checks (replicating jq's `//` defaults):
  - `context_window.current_usage` is `null` before the first API call and again after `/compact`. `renderMain` detects this (`initialized`) and renders a skeleton: dim bar plus `--` placeholders for context %, cache, out, and (when rate data is also absent) the 5h/7d rate limits.
  - `rate_limits` (and each `five_hour` / `seven_day` window independently) appears only for Pro/Max subscribers after the first API response.
  - `context_window.used_percentage` may be `null` early; `renderMain` falls back to computing it from `current_usage` (input-only).
  - `pr.*` is absent until an open PR is found and removed once it merges/closes; `pr.review_state` may be independently absent. `worktree.name` appears only in `--worktree` sessions; `workspace.git_worktree` for any linked worktree. Resolution is `worktree.name // workspace.git_worktree // ""`.
  - `effort.level` reflects the live session effort (including mid-session `/effort` changes) and is absent when the model doesn't support the effort parameter — the `[effort]` badge disappears.
- **Width-aware truncation**: Claude Code (>= 2.1.153) sets `COLUMNS` before running the binary (`tput cols` does not work — output is captured). The directory truncates to `COLUMNS/3` (floor 20) and the displayed branch to `COLUMNS/4` (floor 15) via `truncateMiddle`; `COLUMNS` unset/0 means no truncation. Truncate plain text only — never strings that already contain ANSI codes. Detection logic (ticket tracker matching) must use the full branch, never the truncated display.
- **Commit timestamps can be ahead of the system clock** — the sync-age math clamps negative diffs to 0 rather than rendering `synced -86400s ago`.
- **Context percentage is input-only**: `input_tokens + cache_creation_input_tokens + cache_read_input_tokens` (excludes `output_tokens`). Any manual percentage math must use this same formula to match Claude Code's `used_percentage`.
- **`resets_at` is Unix epoch seconds.**
- **Colors** live in `internal/statusline/` (`ansi.go` / `render.go`): the context bar is a fixed positional gradient (`contextGradient`, one 24-bit RGB triple per segment rendered as truecolor `\x1b[38;2;R;G;Bm` via `fgRGB`, a smooth blue → yellow → orange ramp); the fill reveals it and the percentage takes the leading-edge color. Rate limits threshold at 70 / 85% (blue / yellow / red).
- **Ticket trackers** live in `internal/statusline/ticket.go`: each tracker is a `detectTicket<Name>` function returning `(label, text, url)`; `detectTicket` chains them, first match wins. To add a tracker (Linear, Asana, ...), write a new detector and add one line to the chain. The Shortcut detector links branches matching `sc-#####` to a story; the org slug comes from the `STATUSLINE_SHORTCUT_ORG` environment variable, and no link is produced when it is unset.

## Dependencies

Go 1.24+ (build), `git` (runtime). golangci-lint is optional locally (`make lint`) but enforced in CI. The pinned toolchain is in `.tool-versions` (`golang 1.25.7`).
