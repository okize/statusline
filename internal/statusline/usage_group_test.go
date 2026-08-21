package statusline

import (
	"fmt"
	"testing"
)

// usageGroupPayload reproduces the target example exactly: $23.12 session
// cost, +156 −23 lines, 99% cache hit, 528 output tokens.
func usageGroupPayload(cwd string) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":500,"cache_creation_input_tokens":500,"cache_read_input_tokens":99000,"output_tokens":528}},"cost":{"total_cost_usd":23.12,"total_lines_added":156,"total_lines_removed":23}}`, cwd)
}

// Line 2's usage area is three segments — cost, lines changed, and a
// bracketed cache/out pair joined by " / ". No pipes anywhere.
func TestUsageGroup(t *testing.T) {
	tmp := t.TempDir()

	out := stripANSI(run(t, usageGroupPayload(tmp), 0))
	assertContains(t, "full usage area matches the target shape", out, "$23.12 • (+156 −23) • [99% / 528]")
	assertNotContains(t, "Cache label is gone", out, "Cache:")
	assertNotContains(t, "Out label is gone", out, "Out:")

	colored := run(t, usageGroupPayload(tmp), 0)
	assertContains(t, "cost is white", colored, "\x1b[97m$23.12\x1b[0m")
	assertContains(t, "added lines are mutedGreen", colored, "\x1b[38;5;108m+156")
	assertContains(t, "removed lines are mutedRed", colored, "\x1b[38;5;167m−23")
}

// Without lines data the group drops the (+N −M) part; without cost it drops
// the dollar amount too.
func TestUsageGroupPartsDropCleanly(t *testing.T) {
	tmp := t.TempDir()

	// cost present, lines absent
	out := stripANSI(run(t, costPayload(tmp, 0.42), 0))
	assertContains(t, "no lines segment without lines data", out, "$0.42 • [90% / 10]")

	// one lines field alone is not enough — the part needs both
	half := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":9000,"cache_read_input_tokens":90000,"output_tokens":10}},"cost":{"total_cost_usd":0.42,"total_lines_added":5}}`, tmp)
	outHalf := stripANSI(run(t, half, 0))
	assertContains(t, "no lines segment when total_lines_removed is absent", outHalf, "$0.42 • [90% / 10]")

	// cost object absent entirely
	out2 := stripANSI(run(t, cachePayload(tmp), 0))
	assertContains(t, "bare group without cost data", out2, "\n[90% / 10]")
	assertNotContains(t, "no dollar sign without cost data", out2, "$")

	// large output counts keep the abbreviated form
	big := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":42000}}}`, tmp)
	assertContains(t, "out keeps the k abbreviation", stripANSI(run(t, big, 0)), "/ 42k]")
}

// The skeleton keeps the group shape with placeholders.
func TestUsageGroupSkeleton(t *testing.T) {
	tmp := t.TempDir()
	out := stripANSI(run(t, uninitPayload(tmp), 0))
	assertContains(t, "skeleton group holds placeholders", out, "--% 5h • --% 7d • [--% / --]")
}
