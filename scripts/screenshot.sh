#!/usr/bin/env bash
# Regenerates examples/statusline.svg, the screenshot embedded in README.md.
# Pipes examples/stdin-payload-example.json through the built binary and
# renders the ANSI output with charmbracelet/freeze.
#
# The output must be deterministic — CI commits the file only when it changes,
# so the same source must always produce the same bytes. Four things make it
# so: the payload's workspace.current_dir is rewritten to a throwaway fixture
# repo with fixed contents; worktree.name is forced so the location segment
# shows a [wt:...] tag rather than the random mktemp path; the fixture's commit
# is dated one hour in the future, which the sync-age clamp renders as a stable
# "synced 0s ago"; and TZ is pinned so the rate-limit reset badges format
# identically everywhere.
set -euo pipefail

cd "$(dirname "$0")/.."

FREEZE_VERSION=v0.2.2
OUT=examples/statusline.svg

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

repo="$tmp/repo"
mkdir "$repo"
git -C "$repo" init -q -b main
printf 'alpha\nbeta\n' > "$repo/a.txt"
printf 'bravo\n' > "$repo/b.txt"
git -C "$repo" add a.txt b.txt
future="$(($(date +%s) + 3600)) +0000"
GIT_AUTHOR_DATE="$future" GIT_COMMITTER_DATE="$future" \
	git -C "$repo" -c user.name=statusline -c user.email=statusline@example.com \
	-c commit.gpgsign=false commit -q -m fixture
printf 'alpha\ngamma\ndelta\n' > "$repo/a.txt" # staged: +2/-1
git -C "$repo" add a.txt
printf 'charlie\n' > "$repo/b.txt" # unstaged: +1/-1

# worktree.name is forced (not taken from the example file) so the location
# segment always renders as a [wt:...] tag — otherwise the random mktemp path
# would appear in the output and break determinism.
jq --arg dir "$repo" \
	'.workspace.current_dir = $dir | .workspace.project_dir = $dir
	| .worktree.name = "my-feature"' \
	examples/stdin-payload-example.json > "$tmp/payload.json"

TZ=UTC COLUMNS=120 ./statusline < "$tmp/payload.json" > "$tmp/out.ansi"

go run "github.com/charmbracelet/freeze@$FREEZE_VERSION" \
	--config full --language ansi --output "$OUT" "$tmp/out.ansi"
