# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A custom multi-line status line for Claude Code, written in Bash. Three scripts render a three-line bar showing model, subscription rate limits, context-window usage, working directory, and git status.

This repo is the source of truth. Claude Code runs it via the `statusLine.command` field in `~/.claude/settings.json`, which points at `~/src/statusline/statusline-main.sh`. Edits here take effect on the next Claude Code interaction — there is no build or restart.

User-facing docs (what each line shows, color tables) live in `README.md`; this file covers how the code fits together.

## Develop / test

No build step, test framework, or linter is configured. Test by piping a sample stdin payload to the entry point, exactly as Claude Code invokes it:

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
}' | ./statusline-main.sh
```

- Test git rendering in isolation: `./statusline-git.sh /path/to/repo` (prints its two lines).
- Syntax check: `bash -n statusline-main.sh statusline-git.sh colors.sh`.
- Recommended linter: `shellcheck *.sh` (`brew install shellcheck`).

Keep runs fast. Claude Code debounces updates at 300ms and **cancels an in-flight run** when a new update arrives, so avoid slow work (network calls, unbounded git operations).

## Architecture

Flow: Claude Code → stdin JSON → `statusline-main.sh` → (shells out to) `statusline-git.sh` → stdout (3 lines).

- **statusline-main.sh** — entry point. Reads all of stdin, extracts 12 values in a **single `jq` call** (model, cwd, context-window size/usage, rate-limit percentages/resets), builds the context bar and rate-limit segment, then calls `statusline-git.sh "$cwd"` for the git lines.
- **statusline-git.sh** — standalone git helper. Takes cwd as `$1` and prints **exactly two lines**. main.sh splits them positionally with `sed -n '1p'` / `'2p'`, so this script must always emit two lines (the second may be empty) — changing the line count silently breaks main.sh. It parses `git status --porcelain` once, then runs `--shortstat` only when a file count is > 0, and uses `--no-optional-locks` throughout.
- **colors.sh** — shared ANSI palette, sourced by both scripts.

Both scripts locate siblings via `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`, so **all three files must stay in the same directory**.

Output contract (three lines):
1. model | 5h/7d rate limits | context bar + `In:`/`Out:` token counts
2. directory | git branch, upstream, relative sync time
3. Shortcut link (only if branch matches `sc-#####`) + staged/unstaged stats, or `No pending changes`

## Conventions and gotchas

- **Target runtime is macOS Bash 3.2.** No Bash 4+ features (e.g. associative arrays). OSC 8 hyperlinks are emitted with `printf '%b'`, not `echo -e` (unreliable for `\e` on this shell). Timestamps use BSD `date -r <epoch>`, which is **not portable to GNU/Linux `date`** — this status line is currently macOS-only.
- **stdin JSON contract**: full field list at https://code.claude.com/docs/en/statusline. Several consumed fields are conditionally absent, and the code already guards them — preserve that:
  - `context_window.current_usage` is `null` before the first API call and again after `/compact`. main.sh detects this (`context_initialized`) and renders an empty bar.
  - `rate_limits` (and each `five_hour` / `seven_day` window independently) appears only for Pro/Max subscribers after the first API response. Guarded with `jq`'s `// ""`.
  - `context_window.used_percentage` may be `null` early; main.sh falls back to computing it from `current_usage`.
- **Context percentage is input-only**: `input_tokens + cache_creation_input_tokens + cache_read_input_tokens` (excludes `output_tokens`). Any manual percentage math must use this same formula to match Claude Code's `used_percentage`.
- **`resets_at` is Unix epoch seconds.**
- **Color thresholds** live in main.sh functions: context bar at 50 / 70 / 85% (green / yellow / orange / red); rate limits at 70 / 85% (green / yellow / red).
- **Shortcut integration is Wistia-specific**: branches matching `sc-#####` link to the hardcoded `wistia-pde` Shortcut org.

## Dependencies

`jq` (JSON parsing), `git`, BSD `date`, Bash 3.2+.
