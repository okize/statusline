# Claude Code Statusline

Custom status line for Claude Code that displays model info, context window usage, rate limits, and git status directly in the terminal.

## What it displays

**Line 1:** Model name | rate limit usage (5h/7d with reset times) | context window progress bar with token counts (in/out)

**Line 2:** Current directory, worktree tag (`[wt:name]`, only inside a git worktree) | git branch, upstream tracking with ahead/behind counts (`↑N ↓M`, only when non-zero), and last commit time

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

| Usage | Color |
|-------|-------|
| 0-19% | Green |
| 20-34% | Yellow |
| 35-49% | Orange |
| 50%+ | Red |

## Rate limit colors

| Usage | Color |
|-------|-------|
| 0-69% | Green |
| 70-84% | Yellow |
| 85%+ | Red |

## PR review state colors

| State | Color |
|-------|-------|
| approved | Muted green |
| changes_requested | Muted red |
| draft | Light grey |
| pending / other | Yellow |
