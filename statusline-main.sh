#!/bin/bash

# Main status line for Claude Code
# Receives JSON via stdin with session context
# Calls statusline-git.sh which outputs 2 lines:
#   Line 1: branch [↑N ↓M] • synced Xd ago
#   Line 2: [ticket link |] Staged/Unstaged stats [or "No pending changes"]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib.sh"

# --- Display functions ---

# Render the context bar as 20 square segments (■, each = 5%); filled and
# empty segments share the glyph and differ only by color. Filled segments
# form a fixed blue -> gold -> orange positional gradient (modeled on abtop's
# context meter): the fill reveals the gradient, and the percentage value
# takes the color of the last filled segment, in grey brackets. One xterm-256
# code per segment; fixed colors that do not remap with the terminal theme.
CONTEXT_GRADIENT=(33 33 74 67 109 108 143 179 178 220 220 220 214 214 214 208 208 208 202 202)

build_context_display() {
  local pct="$1" initialized="$2"
  local bar_length=20

  # Skeleton: same structure as the live display, with -- placeholders
  if [ "$initialized" = false ]; then
    local bar=$(printf "%${bar_length}s" | tr ' ' '■')
    echo -e "${DIM_GREY}${bar}${RESET} ${LIGHT_GREY}[--%]${RESET}"
    return
  fi

  local pct_int=${pct%.*}
  local filled=$((pct_int * bar_length / 100))
  [ "$filled" -gt "$bar_length" ] && filled=$bar_length

  local bar="" i
  for ((i = 0; i < bar_length; i++)); do
    if [ "$i" -lt "$filled" ]; then
      bar+="\033[38;5;${CONTEXT_GRADIENT[$i]}m■"
    else
      bar+="${DIM_GREY}■"
    fi
  done

  local label_idx=$((filled - 1))
  [ "$label_idx" -lt 0 ] && label_idx=0
  local label_color="\033[38;5;${CONTEXT_GRADIENT[$label_idx]}m"

  echo -e "${bar}${RESET} ${LIGHT_GREY}[${label_color}${pct_int}%${LIGHT_GREY}]${RESET}"
}

# Format token counts for display (e.g. 42000 -> "42k")
format_tokens() {
  local t="$1"
  if [ "$t" -ge 1000 ]; then
    echo "$((t / 1000))k"
  else
    echo "$t"
  fi
}

# Rate limit color thresholds: 0-70% blue, 70-85% yellow, 85%+ red
rate_limit_color() {
  local pct_int="$1"
  if [ "$pct_int" -lt 70 ]; then
    echo "$BLUE"
  elif [ "$pct_int" -lt 85 ]; then
    echo "$YELLOW"
  else
    echo "$RED"
  fi
}

# Format a reset timestamp (Unix epoch seconds) for display
# For 5h window: "2:50 PM"
# For 7d window: "4/3/25 5:50 PM"
format_reset_time() {
  local epoch="$1"
  local window="$2"  # "5h" or "7d"
  [ -z "$epoch" ] && return

  if [ "$window" = "5h" ]; then
    date -r "$epoch" +"%-I:%M %p" 2>/dev/null
  else
    date -r "$epoch" +"%-m/%-d/%y %-I:%M %p" 2>/dev/null
  fi
}

# --- Parse JSON input (single jq call, one value per line) ---
input=$(cat)
{
  read -r model_name
  read -r cwd
  read -r context_size
  read -r used_percentage
  read -r current_input
  read -r current_cache_create
  read -r current_cache_read
  read -r current_output
  read -r rate_five_pct
  read -r rate_seven_pct
  read -r rate_five_reset
  read -r rate_seven_reset
  read -r pr_number
  read -r pr_url
  read -r pr_state
  read -r worktree_name
} < <(echo "$input" | jq -r '
  .model.display_name,
  .workspace.current_dir,
  (.context_window.context_window_size // 200000),
  (.context_window.used_percentage // 0),
  (.context_window.current_usage.input_tokens // 0),
  (.context_window.current_usage.cache_creation_input_tokens // 0),
  (.context_window.current_usage.cache_read_input_tokens // 0),
  (.context_window.current_usage.output_tokens // 0),
  (.rate_limits.five_hour.used_percentage // ""),
  (.rate_limits.seven_day.used_percentage // ""),
  (.rate_limits.five_hour.resets_at // ""),
  (.rate_limits.seven_day.resets_at // ""),
  (.pr.number // ""),
  (.pr.url // ""),
  (.pr.review_state // ""),
  (.worktree.name // .workspace.git_worktree // "")
')

# --- Current directory ---
current_folder="${cwd/#$HOME/~}"

# Width-aware truncation: Claude Code (>= 2.1.153) sets COLUMNS to the terminal
# width before running the script (tput cols does not work; output is captured).
# Without COLUMNS the path is shown untruncated.
if [ -n "$COLUMNS" ] && [ "$COLUMNS" -gt 0 ] 2>/dev/null; then
  folder_max=$((COLUMNS / 3))
  [ "$folder_max" -lt 20 ] && folder_max=20
  current_folder=$(truncate_middle "$current_folder" "$folder_max")
fi

# --- Context window display ---
# Determine if context has been initialized (current_usage is null before first API call)
context_initialized=true
if [ "$current_input" -eq 0 ] && [ "$current_cache_create" -eq 0 ] && [ "$current_cache_read" -eq 0 ] && [ "$current_output" -eq 0 ]; then
  context_initialized=false
fi

# Ensure percentage is integer for bash arithmetic
used_percentage=${used_percentage%.*}

# Fallback: calculate percentage from current_usage (input-only, matching how used_percentage is calculated)
if [ "$used_percentage" -le 0 ] 2>/dev/null && [ "$context_initialized" = true ]; then
  tokens_in_context=$((current_input + current_cache_create + current_cache_read))
  if [ "$context_size" -gt 0 ] && [ "$tokens_in_context" -gt 0 ]; then
    used_percentage=$((tokens_in_context * 100 / context_size))
  fi
fi

context_display=$(build_context_display "$used_percentage" "$context_initialized")

# --- Cache hit rate (most recent API call) ---
# cache_read / total input tokens. Drops sharply when the prompt cache went
# cold (idle past the ~5min TTL), flagging a slower, pricier turn. Shows a
# -- placeholder before the first API call (no usage data).
total_in=$((current_input + current_cache_create + current_cache_read))
if [ "$total_in" -gt 0 ]; then
  cache_pct=$((current_cache_read * 100 / total_in))
  cache_display=" • ${LIGHT_GREY}Cache: ${cache_pct}%${RESET}"
else
  cache_display=" • ${LIGHT_GREY}Cache: --%${RESET}"
fi

# Output tokens generated by the most recent API call
if [ "$context_initialized" = false ]; then
  tokens_out_display="--"
else
  tokens_out_display=$(format_tokens "$current_output")
fi

# --- Subscription rate limit display ---
# Shows 5-hour and/or 7-day usage when available (subscribers only, after first
# API call). Before the first call, a skeleton with -- placeholders; once
# initialized, absent rate data (API-key users) renders nothing.
rate_limits_display=""
if [ "$context_initialized" = false ] && [ -z "$rate_five_pct" ] && [ -z "$rate_seven_pct" ]; then
  rate_limits_display="${LIGHT_GREY}--% 5h${RESET} | ${LIGHT_GREY}--% 7d${RESET} | "
elif [ -n "$rate_five_pct" ] || [ -n "$rate_seven_pct" ]; then
  rate_parts=()
  if [ -n "$rate_five_pct" ]; then
    pct_int=$(printf "%.0f" "$rate_five_pct")
    color=$(rate_limit_color "$pct_int")
    reset_label=$(format_reset_time "$rate_five_reset" "5h")
    if [ -n "$reset_label" ]; then
      rate_parts+=("${color}${pct_int}% 5h (${reset_label})${RESET}")
    else
      rate_parts+=("${color}${pct_int}% 5h${RESET}")
    fi
  fi
  if [ -n "$rate_seven_pct" ]; then
    pct_int=$(printf "%.0f" "$rate_seven_pct")
    color=$(rate_limit_color "$pct_int")
    reset_label=$(format_reset_time "$rate_seven_reset" "7d")
    if [ -n "$reset_label" ]; then
      rate_parts+=("${color}${pct_int}% 7d (${reset_label})${RESET}")
    else
      rate_parts+=("${color}${pct_int}% 7d${RESET}")
    fi
  fi
  # Join parts with " | "
  rate_limits_display="${rate_parts[0]}"
  if [ "${#rate_parts[@]}" -gt 1 ]; then
    rate_limits_display="${rate_limits_display} | ${rate_parts[1]}"
  fi
  rate_limits_display="${rate_limits_display} | "
fi

# --- Location (directory or worktree tag) ---
# Inside a worktree the [wt:name] tag replaces the directory: the two are
# redundant and together eat too much horizontal space.
# worktree.name is set for --worktree sessions; workspace.git_worktree for any
# linked worktree (absent in the main working tree)
location_display="$current_folder"
if [ -n "$worktree_name" ]; then
  location_display="${ORANGE}[wt:${worktree_name}]${RESET}"
fi

# --- PR badge ---
# pr.* mirrors the open PR for the current branch (absent until a PR is found,
# and removed once it merges or closes). review_state may be independently absent.
pr_display=""
if [ -n "$pr_number" ]; then
  pr_text="PR #${pr_number}"
  if [ -n "$pr_url" ]; then
    # OSC 8 hyperlink; rendered with printf '%b' below
    pr_text="\033]8;;${pr_url}\a${pr_text}\033]8;;\a"
  fi
  pr_display="${LINK_BLUE}${pr_text}${RESET}"
  if [ -n "$pr_state" ]; then
    case "$pr_state" in
      approved) pr_state_color="$MUTED_GREEN" ;;
      changes_requested) pr_state_color="$MUTED_RED" ;;
      draft) pr_state_color="$LIGHT_GREY" ;;
      *) pr_state_color="$YELLOW" ;;
    esac
    pr_display="${pr_display} ${pr_state_color}(${pr_state})${RESET}"
  fi
fi

# --- Git info (2 lines: branch+sync, stats) ---
git_output=$("$SCRIPT_DIR/statusline-git.sh" "$cwd")
git_branch_line=$(echo "$git_output" | sed -n '1p')
git_stats_line=$(echo "$git_output" | sed -n '2p')

# --- Output ---
echo ""
echo -e "${CYAN}${model_name}${RESET} | ${rate_limits_display}${context_display}${cache_display} • ${LIGHT_GREY}Out: ${tokens_out_display}${RESET}"
echo -e "${location_display} | ${git_branch_line}"
# Line 3: [PR badge |] git stats. printf '%b' for OSC 8 (echo -e unreliable here)
stats_line="$git_stats_line"
if [ -n "$pr_display" ]; then
  if [ -n "$stats_line" ]; then
    stats_line="${pr_display} | ${stats_line}"
  else
    stats_line="$pr_display"
  fi
fi
if [ -n "$stats_line" ]; then
  printf '%b\n' "$stats_line"
fi
