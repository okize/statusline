#!/bin/bash

# Shared library for the statusline scripts: ANSI palette and display helpers.
# Sourced by statusline-main.sh and statusline-git.sh.

# --- ANSI palette ---

CYAN='\033[36m'
BLUE='\033[34m'
GREEN='\033[32m'
YELLOW='\033[33m'
ORANGE='\033[38;5;208m'
RED='\033[31m'
LIGHT_GREY='\033[38;5;248m'
WHITE='\033[97m'
LINK_BLUE='\033[94m'
MUTED_GREEN='\033[38;5;108m'
MUTED_RED='\033[38;5;167m'
RESET='\033[0m'

# --- Display helpers ---

# Truncate a string to at most $2 characters, replacing the middle with "…"
# so both the start and end stay readable. Must be called on plain text
# (before ANSI codes are added), otherwise escape sequences get cut mid-code.
# Assumes a UTF-8 locale: under LC_ALL=C, ${#s} and substring expansion are
# byte-based and can split multibyte characters. Counts characters, not
# display columns, so wide (e.g. CJK) characters may exceed the target width.
truncate_middle() {
  local s="$1" max="$2"
  local len=${#s}
  if [ "$len" -le "$max" ] || [ "$max" -lt 5 ]; then
    printf '%s' "$s"
    return
  fi
  local keep=$((max - 1))
  local head=$(((keep + 1) / 2))
  local tail=$((keep - head))
  printf '%s…%s' "${s:0:head}" "${s:len - tail}"
}
