#!/bin/bash

# Git status helper for statusline
# Arguments: $1 = current working directory
# Output (2 lines):
#   Line 1: branch [↑N ↓M] • synced Xd ago
#   Line 2: [ticket link |] Staged/Unstaged stats [or "No pending changes"]

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

cwd="$1"

if [ ! -d "$cwd/.git" ] && ! git --no-optional-locks -C "$cwd" rev-parse --git-dir > /dev/null 2>&1; then
  echo "not a git repo"
  echo ""
  exit 0
fi

# Helper: all git commands quote $cwd to handle paths with spaces
run_git() { git --no-optional-locks -C "$cwd" "$@"; }

# --- Ticket tracker detection ---
# Maps a branch name to a ticket link. Each tracker is a detect_ticket_<name>
# function that inspects the branch name in $1 and, on match, sets:
#   ticket_label  display prefix (e.g. "Shortcut")
#   ticket_text   link text (e.g. "sc-12345")
#   ticket_url    link target
# To add a tracker (Linear, Asana, ...), write a detect_ticket_<name> function
# and add it to the chain in detect_ticket. First match wins.

detect_ticket_shortcut() {
  local match num
  match=$(echo "$1" | grep -oE 'sc-[0-9]+' | head -1)
  [ -z "$match" ] && return 1
  num=$(echo "$match" | grep -oE '[0-9]+')
  ticket_label="Shortcut"
  ticket_text="$match"
  ticket_url="https://app.shortcut.com/wistia-pde/story/${num}"
}

detect_ticket() {
  ticket_label=""
  ticket_text=""
  ticket_url=""
  detect_ticket_shortcut "$1" && return 0
  return 1
}

# --- Branch ---
branch=$(run_git rev-parse --abbrev-ref HEAD 2>/dev/null)

# Width-aware truncation of the displayed branch name (COLUMNS is set by
# Claude Code >= 2.1.153). Detection logic (e.g. ticket tracker matching)
# must keep using the full $branch, not $branch_display.
branch_display="$branch"
if [ -n "$COLUMNS" ] && [ "$COLUMNS" -gt 0 ] 2>/dev/null; then
  branch_max=$((COLUMNS / 4))
  [ "$branch_max" -lt 15 ] && branch_max=15
  branch_display=$(truncate_middle "$branch" "$branch_max")
fi

# --- Upstream ---
# Upstream detection: check exit code explicitly since a deleted remote branch
# can cause git to output literal "@{u}" instead of erroring on some versions
upstream=""
if run_git rev-parse --abbrev-ref --symbolic-full-name @{u} > /dev/null 2>&1; then
  upstream=$(run_git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null)
  # Guard against literal @{u} leaking through on resolution failure
  if [[ "$upstream" == *"@{u}"* ]]; then
    upstream=""
  fi
fi

# Ahead/behind vs upstream: rev-list --left-right --count outputs "behind<TAB>ahead"
# (left = commits only in @{u}, right = commits only in HEAD)
ahead_behind_display=""
if [ -n "$upstream" ]; then
  counts=$(run_git rev-list --left-right --count '@{u}...HEAD' 2>/dev/null)
  if [ -n "$counts" ]; then
    read -r behind ahead <<< "$counts"
    [ "$ahead" -gt 0 ] 2>/dev/null && ahead_behind_display="${ahead_behind_display} ${MUTED_GREEN}↑${ahead}${RESET}"
    [ "$behind" -gt 0 ] 2>/dev/null && ahead_behind_display="${ahead_behind_display} ${MUTED_RED}↓${behind}${RESET}"
  fi
fi

# --- Sync status ---
# Last commit time -> relative sync status
last_commit_time=$(run_git log -1 --format=%ct 2>/dev/null)
diff_seconds=0
sync_color="$LIGHT_GREY"
if [ -n "$last_commit_time" ]; then
  diff_seconds=$(($(date +%s) - last_commit_time))
  # Clamp: commit timestamps can be ahead of the system clock, which would
  # otherwise render a negative age like "synced -86400s ago"
  [ $diff_seconds -lt 0 ] && diff_seconds=0
  if [ $diff_seconds -lt 60 ]; then
    sync_status="synced ${diff_seconds}s ago"
  elif [ $diff_seconds -lt 3600 ]; then
    sync_status="synced $((diff_seconds / 60))m ago"
  elif [ $diff_seconds -lt 86400 ]; then
    sync_status="synced $((diff_seconds / 3600))h ago"
  else
    sync_status="synced $((diff_seconds / 86400))d ago"
  fi
  if [ $diff_seconds -ge 86400 ]; then
    sync_color="$YELLOW"
  fi
else
  sync_status="no commits"
fi

# --- Change stats ---
# Parse porcelain once for file counts (replaces separate diff --name-only calls)
# Porcelain format: XY filename (X=staged status, Y=unstaged status)
#   Staged: first char is [MADRC]
#   Unstaged: second char is [MD]
#   Untracked: starts with ??
porcelain=$(run_git status --porcelain 2>/dev/null)

change_stats=""
if [ -n "$porcelain" ]; then
  staged_files=$(echo "$porcelain" | grep -c '^[MADRC]')
  unstaged_modified=$(echo "$porcelain" | grep -c '^.[MD]')
  untracked_files=$(echo "$porcelain" | grep -c '^??')
  unstaged_files=$((unstaged_modified + untracked_files))

  # Line-level insertion/deletion counts require shortstat (porcelain doesn't include them)
  # Only run the diff command when the relevant file count is > 0
  if [ "$staged_files" -gt 0 ]; then
    staged_diff=$(run_git diff --cached --shortstat 2>/dev/null)
    staged_ins=$(echo "$staged_diff" | grep -oE '[0-9]+ insertion' | grep -oE '[0-9]+')
    staged_del=$(echo "$staged_diff" | grep -oE '[0-9]+ deletion' | grep -oE '[0-9]+')
    change_stats="Staged: ${LIGHT_GREY} ${staged_files}${RESET} • (${MUTED_GREEN}+${staged_ins:-0}${RESET}/${MUTED_RED}-${staged_del:-0}${RESET})"
  fi

  if [ "$unstaged_files" -gt 0 ]; then
    unstaged_diff=$(run_git diff --shortstat 2>/dev/null)
    unstaged_ins=$(echo "$unstaged_diff" | grep -oE '[0-9]+ insertion' | grep -oE '[0-9]+')
    unstaged_del=$(echo "$unstaged_diff" | grep -oE '[0-9]+ deletion' | grep -oE '[0-9]+')
    unstaged_part="Unstaged: ${LIGHT_GREY} ${unstaged_files}${RESET} • (${MUTED_GREEN}+${unstaged_ins:-0}${RESET}/${MUTED_RED}-${unstaged_del:-0}${RESET})"
    if [ -n "$change_stats" ]; then
      change_stats="${change_stats} | ${unstaged_part}"
    else
      change_stats="${unstaged_part}"
    fi
  fi
fi

# --- Output ---
# Line 1: branch + ahead/behind + sync status
echo -e "${branch_display}${ahead_behind_display} • ${sync_color}${sync_status}${RESET}"

# Line 2: [ticket link |] change stats or "No pending changes"
# Uses printf '%b' for OSC 8 hyperlinks (echo -e unreliable for \e on macOS bash 3.2)
detect_ticket "$branch"
ticket_link=""
if [ -n "$ticket_url" ]; then
  ticket_link="${ticket_label}: ${LINK_BLUE} \033]8;;${ticket_url}\a${ticket_text}\033]8;;\a${RESET} | "
fi
if [ -n "$change_stats" ]; then
  printf '%b\n' "${ticket_link}${change_stats}"
else
  printf '%b\n' "${ticket_link}${LIGHT_GREY}No pending changes${RESET}"
fi
