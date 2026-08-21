package statusline

import (
	"fmt"
	"testing"
)

// fastPayload is effortPayload plus fast_mode, so the [fast] badge renders
// next to the effort badge.
func fastPayload(cwd string, fast bool) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":10,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"effort":{"level":"medium"},"fast_mode":%t}`, cwd, fast)
}

func costPayload(cwd string, cost float64) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":9000,"cache_read_input_tokens":90000,"output_tokens":10}},"cost":{"total_cost_usd":%g}}`, cwd, cost)
}

// The [fast] badge mirrors the effort badge's shape — light-grey brackets
// around a muted fill — and sits right after the effort badge on line 1.
func TestFastBadge(t *testing.T) {
	tmp := t.TempDir()

	out := run(t, fastPayload(tmp, true), 0)
	assertContains(t, "fast badge follows the effort badge", stripANSI(out), "Test [medium] [fast]")
	assertContains(t, "fast badge has light-grey brackets and a muted fill", out,
		"\x1b[38;5;248m[\x1b[38;5;108mfast\x1b[38;5;248m]\x1b[0m")

	out2 := stripANSI(run(t, fastPayload(tmp, false), 0))
	assertNotContains(t, "no fast badge when fast_mode is false", out2, "[fast]")

	out3 := stripANSI(run(t, effortPayload(tmp), 0))
	assertNotContains(t, "no fast badge when fast_mode is absent", out3, "[fast]")
}

// fast_mode without an effort level still renders the badge after the model name.
func TestFastBadgeWithoutEffort(t *testing.T) {
	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":10,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"fast_mode":true}`, tmp)
	out := stripANSI(run(t, payload, 0))
	assertContains(t, "fast badge renders without an effort badge", out, "Test [fast]")
}

// The session cost sits on line 2, before the cache segment.
func TestSessionCost(t *testing.T) {
	tmp := t.TempDir()

	out := stripANSI(run(t, costPayload(tmp, 0.42), 0))
	assertContains(t, "cost renders before the usage group", out, "$0.42 • [90% /")

	out2 := stripANSI(run(t, costPayload(tmp, 12.3456), 0))
	assertContains(t, "cost rounds to cents", out2, "$12.35 •")

	out3 := stripANSI(run(t, cachePayload(tmp), 0))
	assertNotContains(t, "no cost segment when cost is absent", out3, "$0")

	out4 := run(t, costPayload(tmp, 0.42), 0)
	assertContains(t, "cost is white", out4, "\x1b[97m$0.42\x1b[0m")

	// null vs absent: an empty cost object (or a null total_cost_usd) also
	// drops the segment, matching the codebase's jq-style // defaulting.
	empty := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":9000,"cache_read_input_tokens":90000,"output_tokens":10}},"cost":{"total_cost_usd":null}}`, tmp)
	out5 := stripANSI(run(t, empty, 0))
	assertNotContains(t, "no cost segment when total_cost_usd is null", out5, "$0")
	assertContains(t, "line 2 still starts at the usage group", out5, "\n[90% /")
}
