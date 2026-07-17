# Claude Code Statusline

Custom status line for Claude Code that displays model info, context window usage, rate limits, and git status directly in the terminal.

## What it displays

**Line 1:** Model name with bracketed reasoning effort (`Fable 5 [xhigh]`, omitted when the model doesn't support effort) | rate limit usage (5h/7d with reset times) | context window gradient bar with bracketed percentage (`[42%]`) | cache hit rate and output tokens of the most recent API call. Before the first API call renders as a skeleton with `--` placeholders.

**Line 2:** Current directory, or worktree tag (`[wt:name]`) in place of the directory when inside a git worktree | git branch, ahead/behind counts vs upstream (`↑N ↓M`, only when non-zero), and last commit time

**Line 3:** Pull request badge (`PR #N (state)`, clickable, only when the branch has an open PR) | Shortcut ticket link (if branch matches `sc-#####`) | staged/unstaged file counts with insertion/deletion stats

When Claude Code provides the terminal width (`COLUMNS`, v2.1.153+), long directory paths and branch names are truncated with a middle ellipsis (`…`).

## Files

| File | Purpose |
|------|---------|
| `statusline-main.sh` | Entry point. Parses JSON from stdin, builds context and rate limit display, calls `statusline-git.sh`, prints output. |
| `statusline-git.sh` | Git helper. Outputs branch/upstream/sync info and staged/unstaged change stats. Detects ticket-tracker IDs (currently Shortcut) from branch names. |
| `lib.sh` | Shared library: ANSI palette and display helpers used by both scripts. |
| `tests/run-tests.sh` | Test suite. Run directly; exits non-zero on failure. |

## Docs

Official documentation: https://code.claude.com/docs/en/statusline

## Setup

Clone this repo, then add the following to your Claude Code `settings.json` (user or project level), pointing `command` at the cloned location:

```json
{
  "statusLine": {
    "type": "command",
    "command": "~/src/statusline/statusline-main.sh"
  }
}
```

Claude Code pipes a JSON object to stdin containing session context (model, workspace, context window usage, rate limits). The script parses this with `jq` and renders the status line.

## Dependencies

- `jq` (JSON parsing)
- `git` (repository status)
- `date` (timestamp formatting)
- Bash 3.2+ (macOS default works)

## Context window colors

The bar is 20 square segments (`■`, each = 5%); filled and empty segments
share the glyph and differ only by color, with unfilled segments in dim grey. Filled segments form a fixed
positional gradient (modeled on abtop's context meter): bright blue at 0%
through steel, sage, and olive to gold at ~50% and deep orange at 100%. The
fill reveals the gradient, and the percentage value takes the color of the
last filled segment, wrapped in grey brackets. Colors are fixed xterm-256
codes and do not remap with the terminal theme.

## Rate limit colors

| Usage | Color |
|-------|-------|
| 0-69% | Blue |
| 70-84% | Yellow |
| 85%+ | Red |

## PR review state colors

| State | Color |
|-------|-------|
| approved | Muted green |
| changes_requested | Muted red |
| draft | Light grey |
| pending / other | Yellow |
