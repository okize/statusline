package statusline

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// pinLocalUTC forces time.Local to UTC for the duration of a test so anything
// rendered through .Local() is timezone-independent. t.Setenv("TZ", ...) is not
// enough — the time package resolves time.Local once, at first use.
func pinLocalUTC(t *testing.T) {
	t.Helper()
	saved := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = saved })
}

// 1788000000 is 2026-08-29T10:40:00Z.
const testResetEpoch = 1788000000

// formatResetTime's non-nil path was uncovered: only resetLayout (the layout
// string) was tested, never the actual formatting of a timestamp.
func TestFormatResetTime(t *testing.T) {
	pinLocalUTC(t)

	assertEqual(t, "absent timestamp renders nothing", formatResetTime(nil, "5h"), "")

	e := int64(testResetEpoch)
	assertEqual(t, "5h reset formats as time only", formatResetTime(&e, "5h"), "10:40 AM")
	assertEqual(t, "7d reset formats with date", formatResetTime(&e, "7d"), "8/29/26 10:40 AM")
}

// No test populated rate_limits.seven_day, so the 7d decode branch, the 7d
// segment, and the " | " join between two windows were all uncovered. The
// parenthesised reset label (rateSegment's label != "" branch) too.
func TestBothRateWindowsWithResets(t *testing.T) {
	pinLocalUTC(t)
	tmp := t.TempDir()

	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"rate_limits":{"five_hour":{"used_percentage":18,"resets_at":%d},"seven_day":{"used_percentage":90,"resets_at":%d}}}`,
		tmp, testResetEpoch, testResetEpoch)

	out := stripANSI(run(t, payload, 0))
	assertContains(t, "5h window with reset label", out, "18% 5h (10:40 AM)")
	assertContains(t, "7d window with dated reset label", out, "90% 7d (8/29/26 10:40 AM)")
	assertContains(t, "the two windows are pipe-joined", out, "5h (10:40 AM) | 90% 7d")

	colored := run(t, payload, 0)
	assertContains(t, "7d colors independently of 5h", colored, red+"90% 7d")

	// A 7d window alone must still render (and still emit the trailing " | ").
	only7d := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"rate_limits":{"seven_day":{"used_percentage":55}}}`, tmp)
	out = stripANSI(run(t, only7d, 0))
	assertContains(t, "7d renders without a 5h window", out, "55% 7d | ")
	assertNotContains(t, "no 5h segment when five_hour absent", out, "5h")
}

// formatTokens only ever saw sub-1000 values, so the "42k" form — the normal
// case for the Out: field — was never rendered.
func TestFormatTokens(t *testing.T) {
	assertEqual(t, "zero", formatTokens(0), "0")
	assertEqual(t, "below 1000 renders raw", formatTokens(999), "999")
	assertEqual(t, "exactly 1000 renders as 1k", formatTokens(1000), "1k")
	assertEqual(t, "truncates toward zero", formatTokens(1999), "1k")
	assertEqual(t, "large count", formatTokens(42000), "42k")

	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":42000}}}`, tmp)
	assertContains(t, "Out: uses the abbreviated form", stripANSI(run(t, payload, 0)), "Out: 42k")
}

// When used_percentage is null/absent, renderMain recomputes it from
// current_usage using the input-only formula. Documented contract, uncovered.
func TestUsedPercentageFallback(t *testing.T) {
	tmp := t.TempDir()

	// 20000 + 5000 + 35000 = 60000 of 200000 -> 30%. output_tokens is excluded.
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"current_usage":{"input_tokens":20000,"cache_creation_input_tokens":5000,"cache_read_input_tokens":35000,"output_tokens":9999}}}`, tmp)
	out := stripANSI(run(t, payload, 0))
	assertContains(t, "percentage computed from current_usage", out, "[30%]")
	assertNotContains(t, "not the uninitialized skeleton", out, "[--%]")

	// An explicit used_percentage wins over the fallback.
	explicit := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":77,"current_usage":{"input_tokens":20000,"cache_creation_input_tokens":5000,"cache_read_input_tokens":35000,"output_tokens":10}}}`, tmp)
	assertContains(t, "explicit percentage is not overridden", stripANSI(run(t, explicit, 0)), "[77%]")
}

// The $HOME -> ~ collapse never fired: every existing test used a t.TempDir()
// path outside $HOME.
func TestCollapseHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	proj := filepath.Join(home, "src", "project")
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q}}`, proj)
	out := stripANSI(run(t, payload, 0))
	assertContains(t, "home prefix collapses to ~", out, "~/src/project")
	assertNotContains(t, "absolute home path not shown", out, home+"/src")

	// A path outside $HOME is untouched.
	assertEqual(t, "non-home path unchanged", collapseHome("/opt/elsewhere"), "/opt/elsewhere")
}

// Render and RenderGit — the exported API — had zero coverage; tests called the
// unexported internals directly. This also covers the malformed-JSON path, the
// only guard between bad stdin and a blank status line.
func TestExportedAPI(t *testing.T) {
	tmp := t.TempDir()

	out, err := Render([]byte(contextPayload(tmp, 10)), 0)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertContains(t, "Render produces the model line", out, "Test")
	assertContains(t, "Render output starts with a blank line", out[:1], "\n")

	if _, err := Render([]byte(`{"model":`), 0); err == nil {
		t.Error("Render should return an error on malformed JSON")
	}

	repo := filepath.Join(t.TempDir(), "r")
	gitInit(t, repo)
	gitCommit(t, repo, "base", nil)
	l1, l2 := RenderGit(repo, 0)
	assertContains(t, "RenderGit line 1 has the branch and sync age", l1, "synced")
	assertEqual(t, "RenderGit line 2 is empty on a clean tree", stripANSI(l2), "No pending changes")
}

// Only "approved" and the default (pending) review states were exercised.
func TestPRBadgeReviewStates(t *testing.T) {
	tmp := t.TempDir()
	badge := func(state string) string {
		p := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"pr":{"number":5,"review_state":%q}}`, tmp, state)
		return run(t, p, 0)
	}
	assertContains(t, "approved is muted green", badge("approved"), mutedGreen+"(approved)")
	assertContains(t, "changes_requested is muted red", badge("changes_requested"), mutedRed+"(changes_requested)")
	assertContains(t, "draft is light grey", badge("draft"), lightGrey+"(draft)")
	assertContains(t, "unknown state falls back to yellow", badge("pending"), yellow+"(pending)")
}

// Narrow terminals hit the truncation floors (directory 20, branch 15) rather
// than COLUMNS/3 and COLUMNS/4. Both floor branches were uncovered.
func TestTruncationFloors(t *testing.T) {
	dir := "/Volumes/aaaa/bbbb/cccc/dddd/eeee/ffff"
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q}}`, dir)

	// COLUMNS/3 would be 10; the floor of 20 must win.
	out := stripANSI(run(t, payload, 30))
	assertContains(t, "directory truncated at the floor of 20", out, truncateMiddle(dir, 20))
	assertNotContains(t, "not truncated at COLUMNS/3", out, truncateMiddle(dir, 10))

	repo := filepath.Join(t.TempDir(), "r")
	gitInit(t, repo)
	gitCommit(t, repo, "base", nil)
	branch := "mw/a-very-long-feature-branch-name"
	gitRun(t, repo, nil, "checkout", "-q", "-b", branch)

	// COLUMNS/4 would be 5; the floor of 15 must win.
	l1, _ := renderGitLines(repo, 20)
	assertContains(t, "branch truncated at the floor of 15", stripANSI(l1), truncateMiddle(branch, 15))
	assertNotContains(t, "not truncated at COLUMNS/4", stripANSI(l1), truncateMiddle(branch, 5))
}

// A percentage above 100 must clamp the fill to the bar width instead of
// indexing past the end of the gradient.
func TestContextBarClampsAbove100(t *testing.T) {
	tmp := t.TempDir()
	out := stripANSI(run(t, contextPayload(tmp, 150), 0))
	assertContains(t, "bar still renders 20 segments", out, "■■■■■■■■■■■■■■■■■■■■")
	assertNotContains(t, "bar does not exceed 20 segments", out, "■■■■■■■■■■■■■■■■■■■■■")
	assertContains(t, "percentage still shown", out, "[150%]")
}

// The non-repo path of renderGitLines was uncovered.
func TestGitLinesOutsideRepo(t *testing.T) {
	l1, l2 := renderGitLines(t.TempDir(), 0)
	assertEqual(t, "non-repo line 1", l1, "not a git repo")
	assertEqual(t, "non-repo line 2 is empty", l2, "")

	l1, l2 = renderGitLines("", 0)
	assertEqual(t, "empty cwd line 1", l1, "not a git repo")
	assertEqual(t, "empty cwd line 2 is empty", l2, "")
}
