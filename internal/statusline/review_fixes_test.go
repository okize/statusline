package statusline

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// jq tolerates a JSON number arriving as a float in an integer-ish field; the
// old bash pipeline kept rendering. The Go port must not blank the whole line
// when e.g. context_window_size or pr.number arrives as 200000.0 / 123.0.
func TestTolerantNumericJSON(t *testing.T) {
	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000.0,"used_percentage":50,"current_usage":{"input_tokens":1000.0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10.0}},"pr":{"number":123.0}}`, tmp)
	out := run(t, payload, 0)
	assertContains(t, "renders model despite float-typed int fields", out, "Test")
	assertContains(t, "renders PR number from a float value", out, "PR #123")
}

// Sync-age minute/hour/day buckets and the >= 1 day yellow escalation (only the
// 0s future-clamp was previously covered).
func TestSyncAgeBuckets(t *testing.T) {
	cases := []struct {
		ago    time.Duration
		want   string
		yellow bool
	}{
		{5 * time.Minute, "synced 5m ago", false},
		{3 * time.Hour, "synced 3h ago", false},
		{50 * time.Hour, "synced 2d ago", true},
	}
	for _, c := range cases {
		repo := filepath.Join(t.TempDir(), "r")
		gitInit(t, repo)
		d := time.Now().Add(-c.ago).Unix()
		env := fmt.Sprintf("%d +0000", d)
		gitCommit(t, repo, "c", []string{"GIT_AUTHOR_DATE=" + env, "GIT_COMMITTER_DATE=" + env})
		l1, _ := renderGitLines(repo, 0)
		assertContains(t, c.want, l1, c.want)
		if c.yellow {
			assertContains(t, "stale commit (>= 1 day) is yellow", l1, yellow+"synced")
		} else {
			assertContains(t, "recent commit is light grey", l1, lightGrey+"synced")
		}
	}
}

// Insertion/deletion counts in the change-stats line (only the "Unstaged:"
// label was previously asserted).
func TestChangeStatsCounts(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "r")
	gitInit(t, repo)
	gitCommit(t, repo, "base", nil)

	// Commit a tracked file (1 line), leaving a clean tree.
	if err := os.WriteFile(filepath.Join(repo, "t.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, nil, "add", "t.txt")
	gitCommit(t, repo, "add t", nil)

	// Stage a new file (+2) and modify the tracked file unstaged (+2).
	if err := os.WriteFile(filepath.Join(repo, "s.txt"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, nil, "add", "s.txt")
	if err := os.WriteFile(filepath.Join(repo, "t.txt"), []byte("x\ny\nz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, l2 := renderGitLines(repo, 0)
	s := stripANSI(l2)
	assertContains(t, "staged insertion count", s, "Staged:  1 • (+2/-0)")
	assertContains(t, "unstaged insertion count", s, "Unstaged:  1 • (+2/-0)")
}

// Exact 70 / 85 threshold boundaries for rate-limit color.
func TestRateLimitColorBoundaries(t *testing.T) {
	assertEqual(t, "69 is blue", rateLimitColor(69), blue)
	assertEqual(t, "70 is yellow", rateLimitColor(70), yellow)
	assertEqual(t, "84 is yellow", rateLimitColor(84), yellow)
	assertEqual(t, "85 is red", rateLimitColor(85), red)
}

// Reset-time layouts, pinned deterministically via a UTC time.Time so the
// assertion is timezone-independent.
func TestResetLayout(t *testing.T) {
	ts := time.Date(2026, 4, 3, 17, 50, 0, 0, time.UTC)
	assertEqual(t, "5h layout", ts.Format(resetLayout("5h")), "5:50 PM")
	assertEqual(t, "7d layout", ts.Format(resetLayout("7d")), "4/3/26 5:50 PM")
}
