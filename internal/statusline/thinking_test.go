package statusline

import (
	"fmt"
	"testing"
)

// thinking.enabled is ignored on purpose: a [thinking] badge was removed because
// extended thinking is on for every one of this user's sessions, so it carried no
// signal. The field must not render anything, even when true.
func TestThinkingFieldIgnored(t *testing.T) {
	tmp := t.TempDir()
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":10,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"effort":{"level":"medium"},"fast_mode":true,"thinking":{"enabled":true}}`, tmp)
	out := stripANSI(run(t, payload, 0))
	assertNotContains(t, "no thinking badge even when thinking.enabled is true", out, "[thinking]")
	assertContains(t, "effort and fast badges still render", out, "Test [medium] [fast] •")
}
