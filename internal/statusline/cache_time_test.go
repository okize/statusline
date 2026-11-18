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
// the clock time the cached prefix goes cold. The whole badge is light grey,
// like the percentage it follows and the cache/out pair.
func TestCacheBadgeWarm(t *testing.T) {
	pinLocalUTC(t)
	tmp := t.TempDir()
	warm := fmt.Sprintf(`{"warm":true,"caching_observed":true,"ttl":"5m","expires_at":%d,"hit_ratio":0.93}`, testResetEpoch)

	lines := renderLines(t, promptCachePayload(tmp, warm), 0)
	assertContains(t, "warm badge follows the context percentage", stripANSI(lines[1]), "[50%] [cache 10:40 AM]")
	assertContains(t, "warm badge is light grey", lines[1], "\x1b[38;5;248m[cache 10:40 AM]\x1b[0m")
	assertNotContains(t, "badge stays off the model line", stripANSI(lines[0]), "cache")

	// numeric tolerance: a float-typed expires_at still renders
	float := fmt.Sprintf(`{"warm":true,"caching_observed":true,"expires_at":%d.0}`, testResetEpoch)
	assertContains(t, "float expires_at renders", stripANSI(renderLines(t, promptCachePayload(tmp, float), 0)[1]), "[cache 10:40 AM]")
}

// A cold cache keeps the badge, so the user sees that the next message pays to
// re-read the whole context. Same light grey as the warm badge.
func TestCacheBadgeCold(t *testing.T) {
	tmp := t.TempDir()
	cold := `{"warm":false,"caching_observed":true,"ttl":"5m","expires_at":null,"hit_ratio":0.93}`

	line2 := renderLines(t, promptCachePayload(tmp, cold), 0)[1]
	assertContains(t, "cold badge follows the context percentage", stripANSI(line2), "[50%] [cache cold]")
	assertContains(t, "cold badge is light grey", line2, "\x1b[38;5;248m[cache cold]\x1b[0m")

	// warm absent reads as not warm
	noWarm := renderLines(t, promptCachePayload(tmp, `{"caching_observed":true}`), 0)[1]
	assertContains(t, "absent warm renders cold", stripANSI(noWarm), "[50%] [cache cold]")
}

// warm with no expires_at has no clock time to show, so the badge says warm.
func TestCacheBadgeWarmWithoutExpiry(t *testing.T) {
	tmp := t.TempDir()
	warm := `{"warm":true,"caching_observed":true,"expires_at":null}`

	line2 := renderLines(t, promptCachePayload(tmp, warm), 0)[1]
	assertContains(t, "warm badge without a time", stripANSI(line2), "[50%] [cache warm]")
	assertContains(t, "warm-without-time is light grey", line2, "\x1b[38;5;248m[cache warm]\x1b[0m")
}

// No badge when the session has never seen cache tokens (caching off, or a
// provider that doesn't report them) or before the first API response, when
// prompt_cache is absent or null. The percentage is then followed by the next
// segment's bullet.
func TestCacheBadgeOmitted(t *testing.T) {
	tmp := t.TempDir()

	cases := map[string]string{
		"caching_observed false":  promptCachePayload(tmp, `{"warm":false,"caching_observed":false,"expires_at":null}`),
		"caching_observed absent": promptCachePayload(tmp, `{"warm":true,"expires_at":1788000000}`),
		"prompt_cache null":       promptCachePayload(tmp, `null`),
		"prompt_cache absent":     contextPayload(tmp, 50),
	}
	for name, payload := range cases {
		line2 := stripANSI(renderLines(t, payload, 0)[1])
		assertNotContains(t, "no cache badge when "+name, line2, "cache")
		assertContains(t, "the percentage meets the next bullet when "+name, line2, "[50%] • [")
	}
}

// After /compact current_usage is null again (skeleton bar) while prompt_cache
// still describes the session; the badge follows the [--%] placeholder.
func TestCacheBadgeOnSkeleton(t *testing.T) {
	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":0},"prompt_cache":{"warm":false,"caching_observed":true,"expires_at":null}}`, tmp)

	line2 := stripANSI(renderLines(t, payload, 0)[1])
	assertContains(t, "badge follows the skeleton percentage", line2, "[--%] [cache cold]")
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
// context percentage: `[50%] • 1h 12m • (+156 −23) • [...]`.
func TestSessionTime(t *testing.T) {
	tmp := t.TempDir()

	line2 := stripANSI(renderLines(t, durationPayload(tmp, "4320000"), 0)[1])
	assertContains(t, "session time sits between context and lines", line2, "[50%] • 1h 12m • (+156 −23) • [90% / 10]")

	colored := run(t, durationPayload(tmp, "4320000"), 0)
	assertContains(t, "session time is light grey", colored, "\x1b[38;5;248m1h 12m\x1b[0m")

	// numeric tolerance: a float-typed duration still renders
	assertContains(t, "float duration renders", stripANSI(run(t, durationPayload(tmp, "4320000.0"), 0)), "[50%] • 1h 12m •")

	// null drops the segment without a dangling separator
	assertContains(t, "null duration drops the segment", stripANSI(run(t, durationPayload(tmp, "null"), 0)), "[50%] • (+156 −23) •")
}

// The session time does not depend on a cost figure.
func TestSessionTimeWithoutCost(t *testing.T) {
	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":9000,"cache_read_input_tokens":90000,"output_tokens":10}},"cost":{"total_duration_ms":754000}}`, tmp)

	lines := renderLines(t, payload, 0)
	assertEqual(t, "line 1 has no cost", stripANSI(lines[0]), "Test")
	assertContains(t, "session time follows the context percentage", stripANSI(lines[1]), "[50%] • 12m • [90% / 10]")
}
