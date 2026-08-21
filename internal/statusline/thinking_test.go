package statusline

import (
	"fmt"
	"testing"
)

// thinkingPayload carries an effort level, fast_mode and thinking.enabled, so
// all three line-1 badges render together.
func thinkingPayload(cwd string, enabled bool) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":10,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"effort":{"level":"medium"},"fast_mode":true,"thinking":{"enabled":%t}}`, cwd, enabled)
}

// The [thinking] badge works like the [fast] badge — light-grey brackets,
// mutedGreen fill — and sits after it on line 1.
func TestThinkingBadge(t *testing.T) {
	tmp := t.TempDir()

	out := run(t, thinkingPayload(tmp, true), 0)
	assertContains(t, "thinking badge follows the fast badge", stripANSI(out), "Test [medium] [fast] [thinking]")
	assertContains(t, "thinking badge has light-grey brackets and a mutedGreen fill", out,
		"\x1b[38;5;248m[\x1b[38;5;108mthinking\x1b[38;5;248m]\x1b[0m")

	out2 := stripANSI(run(t, thinkingPayload(tmp, false), 0))
	assertNotContains(t, "no thinking badge when thinking.enabled is false", out2, "[thinking]")

	out3 := stripANSI(run(t, effortPayload(tmp), 0))
	assertNotContains(t, "no thinking badge when thinking is absent", out3, "[thinking]")

	// null vs absent: a null thinking object or a null enabled field also
	// suppresses the badge.
	for _, thinking := range []string{"null", "{}", `{"enabled":null}`} {
		payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":10,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"thinking":%s}`, tmp, thinking)
		out := stripANSI(run(t, payload, 0))
		assertNotContains(t, "no thinking badge when thinking is "+thinking, out, "[thinking]")
	}
}

// thinking.enabled alone still renders the badge right after the model name.
func TestThinkingBadgeAlone(t *testing.T) {
	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":10,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"thinking":{"enabled":true}}`, tmp)
	out := stripANSI(run(t, payload, 0))
	assertContains(t, "thinking badge renders without effort or fast badges", out, "Test [thinking]")
}
