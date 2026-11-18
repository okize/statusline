package statusline

import (
	"fmt"
	"testing"
)

// promptCachePayload is contextPayload (50%) plus a prompt_cache object. The
// promptCache argument is spliced in raw, so a test can send any field mix,
// including null.
func promptCachePayload(cwd, promptCache string) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"prompt_cache":%s}`, cwd, promptCache)
}

// durationPayload is costPayload plus cost.total_duration_ms and both lines
// counts, so the session time renders between the cost and the lines changed.
func durationPayload(cwd, durationMS string) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":9000,"cache_read_input_tokens":90000,"output_tokens":10}},"cost":{"total_cost_usd":12.34,"total_duration_ms":%s,"total_lines_added":156,"total_lines_removed":23}}`, cwd, durationMS)
}

// A warm cache renders a bracketed badge after the context percentage, holding
// the clock time the cached prefix goes cold. Same shape as the rate window's
// reset badge: light-grey brackets around a fixed mutedBlue fill.
func TestCacheBadgeWarm(t *testing.T) {
	pinLocalUTC(t)
	tmp := t.TempDir()
	warm := fmt.Sprintf(`{"warm":true,"caching_observed":true,"ttl":"5m","expires_at":%d,"hit_ratio":0.93}`, testResetEpoch)

	lines := renderLines(t, promptCachePayload(tmp, warm), 0)
	assertContains(t, "warm badge follows the context percentage", stripANSI(lines[0]), "[50%] [cache 10:40 AM]")
	assertContains(t, "warm badge has light-grey brackets and a mutedBlue fill", lines[0],
		"\x1b[38;5;248m[\x1b[38;5;68mcache 10:40 AM\x1b[38;5;248m]\x1b[0m")
	assertNotContains(t, "badge stays off the usage line", stripANSI(lines[1]), "cache")

	// numeric tolerance: a float-typed expires_at still renders
	float := fmt.Sprintf(`{"warm":true,"caching_observed":true,"expires_at":%d.0}`, testResetEpoch)
	assertContains(t, "float expires_at renders", stripANSI(renderLines(t, promptCachePayload(tmp, float), 0)[0]), "[cache 10:40 AM]")
}

// A cold cache keeps the badge but dims it: the next message pays to re-read
// the whole context, which is worth seeing.
func TestCacheBadgeCold(t *testing.T) {
	tmp := t.TempDir()
	cold := `{"warm":false,"caching_observed":true,"ttl":"5m","expires_at":null,"hit_ratio":0.93}`

	line1 := renderLines(t, promptCachePayload(tmp, cold), 0)[0]
	assertContains(t, "cold badge follows the context percentage", stripANSI(line1), "[50%] [cache cold]")
	assertContains(t, "cold badge has a dimGrey fill", line1,
		"\x1b[38;5;248m[\x1b[38;5;242mcache cold\x1b[38;5;248m]\x1b[0m")

	// warm absent reads as not warm
	noWarm := renderLines(t, promptCachePayload(tmp, `{"caching_observed":true}`), 0)[0]
	assertContains(t, "absent warm renders cold", stripANSI(noWarm), "[50%] [cache cold]")
}

// warm with no expires_at has no clock time to show, so the badge says warm.
func TestCacheBadgeWarmWithoutExpiry(t *testing.T) {
	tmp := t.TempDir()
	warm := `{"warm":true,"caching_observed":true,"expires_at":null}`

	line1 := renderLines(t, promptCachePayload(tmp, warm), 0)[0]
	assertContains(t, "warm badge without a time", stripANSI(line1), "[50%] [cache warm]")
	assertContains(t, "warm-without-time keeps the mutedBlue fill", line1, "\x1b[38;5;68mcache warm")
}

// No badge when the session has never seen cache tokens (caching off, or a
// provider that doesn't report them) or before the first API response, when
// prompt_cache is absent or null. Line 1 then ends at the percentage.
func TestCacheBadgeOmitted(t *testing.T) {
	tmp := t.TempDir()

	cases := map[string]string{
		"caching_observed false":  promptCachePayload(tmp, `{"warm":false,"caching_observed":false,"expires_at":null}`),
		"caching_observed absent": promptCachePayload(tmp, `{"warm":true,"expires_at":1788000000}`),
		"prompt_cache null":       promptCachePayload(tmp, `null`),
		"prompt_cache absent":     contextPayload(tmp, 50),
	}
	for name, payload := range cases {
		line1 := stripANSI(renderLines(t, payload, 0)[0])
		assertNotContains(t, "no cache badge when "+name, line1, "cache")
		assertContains(t, "line 1 ends at the percentage when "+name, line1+"\n", "[50%]\n")
	}
}

// After /compact current_usage is null again (skeleton bar) while prompt_cache
// still describes the session; the badge follows the [--%] placeholder.
func TestCacheBadgeOnSkeleton(t *testing.T) {
	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":0},"prompt_cache":{"warm":false,"caching_observed":true,"expires_at":null}}`, tmp)

	line1 := stripANSI(renderLines(t, payload, 0)[0])
	assertContains(t, "badge follows the skeleton percentage", line1, "[--%] [cache cold]")
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{-5000, "0s"}, // clamped, like the sync age
		{0, "0s"},
		{45000, "45s"},
		{59999, "59s"},
		{60000, "1m"},
		{754000, "12m"},
		{3599999, "59m"},
		{3600000, "1h 0m"},
		{4320000, "1h 12m"},
		{93784000, "26h 3m"},
	}
	for _, c := range cases {
		assertEqual(t, fmt.Sprintf("formatDuration(%d)", c.ms), formatDuration(c.ms), c.want)
	}
}

// The session's wall-clock time is its own light-grey segment right after the
// cost: `$12.34 • 1h 12m • (+156 −23) • [...]`.
func TestSessionTime(t *testing.T) {
	tmp := t.TempDir()

	out := stripANSI(run(t, durationPayload(tmp, "4320000"), 0))
	assertContains(t, "session time sits between cost and lines", out, "$12.34 • 1h 12m • (+156 −23) • [90% / 10]")

	colored := run(t, durationPayload(tmp, "4320000"), 0)
	assertContains(t, "session time is light grey", colored, "\x1b[38;5;248m1h 12m\x1b[0m")

	// numeric tolerance: a float-typed duration still renders
	assertContains(t, "float duration renders", stripANSI(run(t, durationPayload(tmp, "4320000.0"), 0)), "$12.34 • 1h 12m •")

	// null drops the segment without a dangling separator
	assertContains(t, "null duration drops the segment", stripANSI(run(t, durationPayload(tmp, "null"), 0)), "$12.34 • (+156 −23) •")
}

// Without a cost figure the usage line starts at the session time.
func TestSessionTimeWithoutCost(t *testing.T) {
	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":9000,"cache_read_input_tokens":90000,"output_tokens":10}},"cost":{"total_duration_ms":754000}}`, tmp)

	line2 := stripANSI(renderLines(t, payload, 0)[1])
	assertEqual(t, "usage line leads with the session time", line2, "12m • [90% / 10]")
}
