package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// End-to-end tests over the built binary. This is the only layer that exercises
// what Claude Code actually invokes: stdin JSON in, the exact byte stream out.
// The unit tests call the renderer directly and so cannot catch a regression in
// main's own wiring — notably fmt.Print vs fmt.Println, which decides whether
// the output carries a stray trailing newline.

// binPath is the statusline binary, built once for all tests in this package.
var binPath string

func TestMain(m *testing.M) {
	// No defer: os.Exit skips deferred calls, so cleanup is explicit on both
	// the failure and success paths.
	dir, err := os.MkdirTemp("", "statusline-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: mktemp:", err)
		os.Exit(1)
	}

	binPath = filepath.Join(dir, "statusline")
	if out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: go build: %v\n%s", err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// runBin executes the binary with the given stdin, args and COLUMNS value
// (empty means COLUMNS is unset), returning stdout.
func runBin(t *testing.T, stdin, columns string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(), "COLUMNS="+columns)
	if columns == "" {
		// Drop COLUMNS entirely rather than passing it through empty.
		filtered := cmd.Env[:0]
		for _, kv := range cmd.Env {
			if !strings.HasPrefix(kv, "COLUMNS=") {
				filtered = append(filtered, kv)
			}
		}
		cmd.Env = filtered
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running binary %v: %v\nstderr: %s", args, err, stderr.String())
	}
	return string(out)
}

var e2eSGR = regexp.MustCompile("\x1b\\[[0-9;]*m")

func e2eStrip(s string) string { return e2eSGR.ReplaceAllString(s, "") }

func e2ePayload(cwd string) string {
	return fmt.Sprintf(`{
	  "model": {"display_name": "Opus 4.8"},
	  "workspace": {"current_dir": %q},
	  "context_window": {
	    "context_window_size": 200000,
	    "used_percentage": 42,
	    "current_usage": {"input_tokens": 30000, "output_tokens": 12000,
	                      "cache_creation_input_tokens": 5000, "cache_read_input_tokens": 49000}
	  },
	  "rate_limits": {"five_hour": {"used_percentage": 18}, "seven_day": {"used_percentage": 55}}
	}`, cwd)
}

// The output contract: a leading blank line, then the three content lines, with
// exactly one trailing newline and no extra blank line at the end. main uses
// fmt.Print (not Println) for this path; Println would add a fourth newline.
func TestE2EStdinOutputShape(t *testing.T) {
	repo := e2eRepo(t)
	out := runBin(t, e2ePayload(repo), "")

	if !strings.HasPrefix(out, "\n") {
		t.Errorf("output must start with a blank line, got %q", out[:min(10, len(out))])
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Errorf("output must end with exactly one newline, got %q", out[max(0, len(out)-10):])
	}

	// "\n" + 3 lines + trailing "\n" -> 5 fields, first and last empty.
	fields := strings.Split(out, "\n")
	if len(fields) != 5 || fields[0] != "" || fields[4] != "" {
		t.Fatalf("expected a leading blank line and 3 content lines, got %d fields: %q", len(fields), fields)
	}

	l1, l2, l3 := e2eStrip(fields[1]), e2eStrip(fields[2]), e2eStrip(fields[3])
	if !strings.Contains(l1, "Opus 4.8") || !strings.Contains(l1, "[42%]") {
		t.Errorf("line 1 missing model or context percentage: %q", l1)
	}
	if !strings.Contains(l1, "18% 5h") || !strings.Contains(l1, "55% 7d") {
		t.Errorf("line 1 missing rate limits: %q", l1)
	}
	if !strings.Contains(l1, "Cache: 58%") || !strings.Contains(l1, "Out: 12k") {
		t.Errorf("line 1 missing cache/out: %q", l1)
	}
	if !strings.Contains(l2, repo) || !strings.Contains(l2, "synced") {
		t.Errorf("line 2 missing directory or sync age: %q", l2)
	}
	if l3 != "No pending changes" {
		t.Errorf("line 3 = %q, want %q", l3, "No pending changes")
	}
}

// COLUMNS is how Claude Code passes terminal width; the binary must read it
// from the environment and truncate accordingly.
func TestE2EColumnsTruncation(t *testing.T) {
	long := filepath.Join(t.TempDir(), "a-quite-long-directory-name-for-truncation")
	payload := e2ePayload(long)

	wide := e2eStrip(runBin(t, payload, ""))
	if !strings.Contains(wide, long) {
		t.Errorf("with COLUMNS unset the full path should render, got %q", wide)
	}

	narrow := e2eStrip(runBin(t, payload, "60"))
	if strings.Contains(narrow, long) {
		t.Errorf("with COLUMNS=60 the path should be truncated, got %q", narrow)
	}
	if !strings.Contains(narrow, "…") {
		t.Errorf("truncated path should contain an ellipsis, got %q", narrow)
	}
}

// The `git <dir>` subcommand prints its two lines with Println, so both lines
// are newline-terminated regardless of content.
func TestE2EGitSubcommand(t *testing.T) {
	repo := e2eRepo(t)

	out := runBin(t, "", "", "git", repo)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 || lines[2] != "" {
		t.Fatalf("git subcommand should print exactly 2 newline-terminated lines, got %q", out)
	}
	if !strings.Contains(e2eStrip(lines[0]), "synced") {
		t.Errorf("git line 1 missing sync age: %q", lines[0])
	}
	if e2eStrip(lines[1]) != "No pending changes" {
		t.Errorf("git line 2 = %q", lines[1])
	}

	// Missing directory argument: cwd is empty, so the repo check fails.
	out = runBin(t, "", "", "git")
	if got := strings.Split(out, "\n")[0]; got != "not a git repo" {
		t.Errorf("git with no dir = %q, want %q", got, "not a git repo")
	}
}

// Malformed stdin must not produce partial or garbage output — the renderer
// errors and main prints nothing rather than a broken status line.
func TestE2EMalformedStdin(t *testing.T) {
	if out := runBin(t, `{"model":`, ""); out != "" {
		t.Errorf("malformed JSON should produce no output, got %q", out)
	}
	if out := runBin(t, "", ""); out != "" {
		t.Errorf("empty stdin should produce no output, got %q", out)
	}
}

// e2eRepo builds a throwaway repo with one commit and a clean tree.
func e2eRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("-c", "user.name=test", "-c", "user.email=test@test",
		"commit", "-q", "--allow-empty", "-m", "base")
	return dir
}
