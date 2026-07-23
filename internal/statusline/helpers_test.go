package statusline

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// --- assertions (mirror tests/run-tests.sh) ---

func assertContains(t *testing.T, desc, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("%s\n  expected to contain: %q\n  got: %q", desc, needle, haystack)
	}
}

func assertNotContains(t *testing.T, desc, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("%s\n  expected NOT to contain: %q\n  got: %q", desc, needle, haystack)
	}
}

func assertEqual(t *testing.T, desc, actual, expected string) {
	t.Helper()
	if actual != expected {
		t.Errorf("%s\n  expected: %q\n  got:      %q", desc, expected, actual)
	}
}

// stripANSI removes SGR color escapes so assertions can check text placement
// across color boundaries. Matches the sed in run-tests.sh (only \x1b[...m).
var sgrRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return sgrRE.ReplaceAllString(s, "") }

// --- statusline-main.sh invocation ---

// run parses a JSON payload and renders the full status output, mirroring
// `echo "$payload" | statusline-main.sh` with COLUMNS set (columns>0) or unset
// (columns==0).
func run(t *testing.T, jsonStr string, columns int) string {
	t.Helper()
	in, err := parseInput([]byte(jsonStr))
	if err != nil {
		t.Fatalf("parseInput: %v", err)
	}
	return renderMain(in, columns)
}

// gitLines renders the two git lines joined with a newline, so contains-checks
// that span both lines work like the bash `out` variable did.
func gitLines(cwd string, columns int) string {
	l1, l2 := renderGitLines(cwd, columns)
	return l1 + "\n" + l2
}

// --- git fixtures (mirror run-tests.sh helpers) ---

func gitRun(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, nil, "init", "-q")
}

// gitCommit commits with a fixed identity and no dependency on global config,
// matching git_commit() in run-tests.sh. env may carry GIT_AUTHOR_DATE /
// GIT_COMMITTER_DATE for dated commits.
func gitCommit(t *testing.T, dir, msg string, env []string) {
	t.Helper()
	gitRun(t, dir, env,
		"-c", "user.name=test", "-c", "user.email=test@test",
		"commit", "-q", "--allow-empty", "-m", msg)
}

// --- payload builders (mirror the payload functions in run-tests.sh) ---

func contextPayload(cwd string, pct int) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":%d,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}}}`, cwd, pct)
}

func uninitPayload(cwd string) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":0}}`, cwd)
}

func cachePayload(cwd string) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":9000,"cache_read_input_tokens":90000,"output_tokens":10}}}`, cwd)
}

func effortPayload(cwd string) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":10,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"effort":{"level":"xhigh"}}`, cwd)
}

func ratePayload(cwd string, pct int) string {
	return fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"context_window":{"context_window_size":200000,"used_percentage":50,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"rate_limits":{"five_hour":{"used_percentage":%d}}}`, cwd, pct)
}
