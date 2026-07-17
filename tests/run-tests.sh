#!/bin/bash

# Test suite for statusline scripts. Bash 3.2 compatible.
# Run: ./tests/run-tests.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

PASS=0
FAIL=0

assert_contains() {
  local desc="$1" haystack="$2" needle="$3"
  if [[ "$haystack" == *"$needle"* ]]; then
    PASS=$((PASS + 1))
    echo "PASS: $desc"
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: $desc"
    echo "  expected to contain: $needle"
    echo "  got: $haystack"
  fi
}

assert_not_contains() {
  local desc="$1" haystack="$2" needle="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    PASS=$((PASS + 1))
    echo "PASS: $desc"
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: $desc"
    echo "  expected NOT to contain: $needle"
    echo "  got: $haystack"
  fi
}

assert_equals() {
  local desc="$1" actual="$2" expected="$3"
  if [ "$actual" = "$expected" ]; then
    PASS=$((PASS + 1))
    echo "PASS: $desc"
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: $desc"
    echo "  expected: $expected"
    echo "  got:      $actual"
  fi
}

# Remove ANSI color codes so assertions can check text placement across
# color boundaries (e.g. "origin/main ↑1" spans a color escape)
strip_ansi() {
  sed -E $'s/\x1b\\[[0-9;]*m//g'
}

TMP=$(mktemp -d "${TMPDIR:-/tmp}/statusline-tests.XXXXXX")
trap 'rm -rf "$TMP"' EXIT

# Git helper: commit with identity set, no global config dependency
git_commit() {
  local repo="$1" msg="$2"
  git -C "$repo" -c user.name=test -c user.email=test@test commit -q --allow-empty -m "$msg"
}

# --- statusline-git.sh: sync age ---

# A commit dated in the future must not produce a negative age
git init -q "$TMP/future-repo"
future_epoch=$(($(date +%s) + 86400))
GIT_AUTHOR_DATE="$future_epoch +0000" GIT_COMMITTER_DATE="$future_epoch +0000" \
  git_commit "$TMP/future-repo" "future commit"
out=$("$ROOT_DIR/statusline-git.sh" "$TMP/future-repo")
assert_not_contains "future-dated commit does not render negative sync age" "$out" "synced -"
assert_contains "future-dated commit clamps to 0s ago" "$out" "synced 0s ago"

# --- statusline-git.sh: ahead/behind counts ---

git init -q --bare -b main "$TMP/origin.git"
git clone -q "$TMP/origin.git" "$TMP/clone" 2>/dev/null
git -C "$TMP/clone" symbolic-ref HEAD refs/heads/main
git_commit "$TMP/clone" "base"
git -C "$TMP/clone" push -q -u origin main 2>/dev/null

out=$("$ROOT_DIR/statusline-git.sh" "$TMP/clone")
assert_not_contains "in-sync branch shows no ahead marker" "$out" "↑"
assert_not_contains "in-sync branch shows no behind marker" "$out" "↓"
assert_not_contains "no Upstream label" "$out" "Upstream"
assert_not_contains "no ticket link for a branch without a ticket id" "$out" "Shortcut:"

git_commit "$TMP/clone" "local work"
out=$("$ROOT_DIR/statusline-git.sh" "$TMP/clone" | strip_ansi)
assert_contains "unpushed commit shows ahead count after branch name" "$out" "main ↑1 •"
assert_not_contains "no behind marker when only ahead" "$out" "↓"

git clone -q "$TMP/origin.git" "$TMP/clone2" 2>/dev/null
git_commit "$TMP/clone2" "remote work"
git -C "$TMP/clone2" push -q origin main 2>/dev/null
git -C "$TMP/clone" fetch -q origin
out=$("$ROOT_DIR/statusline-git.sh" "$TMP/clone" | strip_ansi)
assert_contains "diverged branch shows both counts after branch name" "$out" "main ↑1 ↓1 •"

# Exactly-2-lines contract: main.sh splits git.sh output positionally
line_count=$(COLUMNS=60 "$ROOT_DIR/statusline-git.sh" "$TMP/clone" | wc -l | tr -d ' ')
assert_equals "statusline-git.sh outputs exactly 2 lines for a repo with upstream" "$line_count" "2"
line_count=$("$ROOT_DIR/statusline-git.sh" "$TMP" | wc -l | tr -d ' ')
assert_equals "statusline-git.sh outputs exactly 2 lines for a non-repo" "$line_count" "2"

# --- lib.sh: truncate_middle helper ---

source "$ROOT_DIR/lib.sh"
assert_equals "truncate_middle shortens long string with middle ellipsis" \
  "$(truncate_middle "abcdefghijklmnop" 9)" "abcd…mnop"
assert_equals "truncate_middle leaves short string unchanged" \
  "$(truncate_middle "abc" 9)" "abc"

# --- statusline-main.sh: cwd truncation via COLUMNS ---

long_cwd="/Volumes/test/some/extremely/long/path/to/a/deeply/nested/project/directory"
payload="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$long_cwd\"}}"

out=$(echo "$payload" | COLUMNS=60 "$ROOT_DIR/statusline-main.sh")
assert_contains "long cwd truncated when COLUMNS set" "$out" "…"
assert_not_contains "full cwd absent when COLUMNS set" "$out" "$long_cwd"

out=$(echo "$payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "full cwd shown when COLUMNS unset" "$out" "$long_cwd"

# --- statusline-main.sh: context bar ---

# 20-char bar (each char = 5%) whose filled chars form a fixed blue->gold->
# orange gradient; the percentage text takes the fill's leading-edge color
context_payload() {
  echo "{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\"},\"context_window\":{\"context_window_size\":200000,\"used_percentage\":$1,\"current_usage\":{\"input_tokens\":1000,\"cache_creation_input_tokens\":0,\"cache_read_input_tokens\":0,\"output_tokens\":10}}}"
}

out=$(context_payload 10 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh" | strip_ansi)
assert_contains "bar renders 20 square segments" "$out" "■■■■■■■■■■■■■■■■■■■■"
assert_not_contains "bar is not wider than 20 segments" "$out" "■■■■■■■■■■■■■■■■■■■■■"
assert_contains "percentage shown without token fraction" "$out" "Context: 10%"
assert_not_contains "token fraction no longer shown" "$out" "/200k)"

# Filled vs empty segments share the ■ glyph and differ only by color, so
# fill level is asserted via the per-segment color codes
out=$(context_payload 10 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "10% fills exactly 2 segments" "$out" $'\033[38;5;33m■\033[38;5;33m■\033[38;5;248m■'

uninit_payload="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\"},\"context_window\":{\"context_window_size\":200000,\"used_percentage\":0}}"
out=$(echo "$uninit_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "uninitialized bar is 20 grey segments" "$out" $'\033[38;5;248m■■■■■■■■■■■■■■■■■■■■'
assert_not_contains "uninitialized bar is not wider than 20 segments" "$out" "■■■■■■■■■■■■■■■■■■■■■"

# --- statusline-main.sh: cache hit rate ---

cache_payload="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\"},\"context_window\":{\"context_window_size\":200000,\"used_percentage\":50,\"current_usage\":{\"input_tokens\":1000,\"cache_creation_input_tokens\":9000,\"cache_read_input_tokens\":90000,\"output_tokens\":10}}}"
out=$(echo "$cache_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "cache hit rate computed from current_usage" "$out" "Cache: 90%"
assert_not_contains "In tokens no longer shown" "$out" "In:"
assert_not_contains "no placeholders once usage data arrives" "$out" "--%"

# --- statusline-main.sh: rate limit colors ---

rate_payload() {
  echo "{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\"},\"context_window\":{\"context_window_size\":200000,\"used_percentage\":50,\"current_usage\":{\"input_tokens\":1000,\"cache_creation_input_tokens\":0,\"cache_read_input_tokens\":0,\"output_tokens\":10}},\"rate_limits\":{\"five_hour\":{\"used_percentage\":$1}}}"
}

out=$(rate_payload 8 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "rate limit usage below 70% is blue" "$out" $'\033[34m8% 5h'
out=$(rate_payload 75 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "rate limit usage 70-84% is yellow" "$out" $'\033[33m75% 5h'
out=$(rate_payload 90 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "rate limit usage 85%+ is red" "$out" $'\033[31m90% 5h'

# --- statusline-main.sh: pre-first-call skeleton ---

out=$(echo "$uninit_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh" | strip_ansi)
assert_contains "skeleton shows 5h rate placeholder" "$out" "--% 5h"
assert_contains "skeleton shows 7d rate placeholder" "$out" "--% 7d"
assert_contains "skeleton shows context placeholder" "$out" "Context: --%"
assert_not_contains "skeleton drops the token fraction" "$out" "(--/"
assert_contains "skeleton shows cache placeholder" "$out" "Cache: --%"
assert_contains "skeleton shows out placeholder" "$out" "Out: --"
assert_not_contains "skeleton does not show Out: 0" "$out" "Out: 0"

# Gradient assertions anchor on the escape code immediately before a filled
# bar char (or before the percentage text for leading-edge label colors)
out=$(context_payload 100 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "gradient starts bright blue" "$out" $'\033[38;5;33m■'
assert_contains "gradient midpoint is gold" "$out" $'\033[38;5;220m■'
assert_contains "gradient ends deep orange" "$out" $'\033[38;5;202m■'
assert_not_contains "no tier-green bar chars" "$out" $'\033[32m■'

out=$(context_payload 18 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "18% label is steel blue" "$out" $'\033[38;5;67m18%'
out=$(context_payload 42 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "42% label is gold" "$out" $'\033[38;5;178m42%'
out=$(context_payload 72 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "72% label is orange" "$out" $'\033[38;5;214m72%'
out=$(context_payload 91 | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "91% label is deep orange" "$out" $'\033[38;5;202m91%'

# --- statusline-git.sh: branch truncation via COLUMNS ---

long_branch="feature/very-long-branch-name-for-truncation-testing"
git -C "$TMP/future-repo" checkout -q -b "$long_branch"

out=$(COLUMNS=60 "$ROOT_DIR/statusline-git.sh" "$TMP/future-repo")
assert_contains "long branch truncated when COLUMNS set" "$out" "…"
assert_not_contains "full branch absent when COLUMNS set" "$out" "$long_branch"

out=$(env -u COLUMNS "$ROOT_DIR/statusline-git.sh" "$TMP/future-repo")
assert_contains "full branch shown when COLUMNS unset" "$out" "$long_branch"

# Shortcut detection must use the full branch name, not the truncated display
sc_branch="feature/sc-54321-very-long-description-of-the-work"
git -C "$TMP/future-repo" checkout -q -b "$sc_branch"
out=$(COLUMNS=60 "$ROOT_DIR/statusline-git.sh" "$TMP/future-repo")
assert_contains "Shortcut link built from full branch when display truncated" "$out" "app.shortcut.com/wistia-pde/story/54321"

# --- statusline-main.sh: PR badge ---

pr_payload="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\"},\"pr\":{\"number\":123,\"url\":\"https://github.com/okize/statusline/pull/123\",\"review_state\":\"approved\"}}"
out=$(echo "$pr_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "PR badge shows number" "$out" "PR #123"
assert_contains "PR badge shows review state" "$out" "(approved)"
assert_contains "PR badge embeds link url" "$out" "pull/123"

pr_payload_no_state="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\"},\"pr\":{\"number\":7,\"url\":\"https://github.com/okize/statusline/pull/7\"}}"
out=$(echo "$pr_payload_no_state" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "PR badge renders without review_state" "$out" "PR #7"
assert_not_contains "no empty parens when review_state absent" "$out" "()"

no_pr_payload="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\"}}"
out=$(echo "$no_pr_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_not_contains "no PR badge when pr absent" "$out" "PR #"

# PR badge joins with git stats when both are present (dirty repo cwd)
echo "dirty" > "$TMP/clone/untracked.txt"
pr_dirty_payload="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP/clone\"},\"pr\":{\"number\":9,\"url\":\"https://github.com/okize/statusline/pull/9\",\"review_state\":\"pending\"}}"
out=$(echo "$pr_dirty_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "PR badge present alongside git stats" "$out" "PR #9"
assert_contains "git stats present alongside PR badge" "$out" "Unstaged:"

# --- statusline-main.sh: worktree indicator ---

linked_wt_payload="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\",\"git_worktree\":\"feature-xyz\"}}"
out=$(echo "$linked_wt_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "linked worktree shows tag from workspace.git_worktree" "$out" "wt:feature-xyz"
assert_not_contains "worktree tag replaces the directory" "$out" "$TMP"

session_wt_payload="{\"model\":{\"display_name\":\"Test\"},\"workspace\":{\"current_dir\":\"$TMP\"},\"worktree\":{\"name\":\"my-feature\",\"path\":\"$TMP\"}}"
out=$(echo "$session_wt_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_contains "--worktree session shows tag from worktree.name" "$out" "wt:my-feature"
assert_not_contains "worktree tag replaces the directory in --worktree session" "$out" "$TMP"

out=$(echo "$no_pr_payload" | env -u COLUMNS "$ROOT_DIR/statusline-main.sh")
assert_not_contains "no worktree tag outside a worktree" "$out" "wt:"

# --- Summary ---
echo ""
echo "$PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
