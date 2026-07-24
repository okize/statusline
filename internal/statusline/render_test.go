package statusline

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCwdTruncation(t *testing.T) {
	longCwd := "/Volumes/test/some/extremely/long/path/to/a/deeply/nested/project/directory"
	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q}}`, longCwd)

	out := run(t, payload, 60)
	assertContains(t, "long cwd truncated when COLUMNS set", out, "…")
	assertNotContains(t, "full cwd absent when COLUMNS set", out, longCwd)

	out = run(t, payload, 0)
	assertContains(t, "full cwd shown when COLUMNS unset", out, longCwd)
}

func TestContextBar(t *testing.T) {
	tmp := t.TempDir()

	out := stripANSI(run(t, contextPayload(tmp, 10), 0))
	assertContains(t, "bar renders 20 square segments", out, "■■■■■■■■■■■■■■■■■■■■")
	assertNotContains(t, "bar is not wider than 20 segments", out, "■■■■■■■■■■■■■■■■■■■■■")
	assertContains(t, "percentage bracketed after the bar", out, "■ [10%]")
	assertNotContains(t, "no Context label", out, "Context:")
	assertNotContains(t, "token fraction no longer shown", out, "/200k)")

	out2 := run(t, contextPayload(tmp, 10), 0)
	assertContains(t, "10% fills exactly 2 segments", out2, "\x1b[38;2;58;133;247m■\x1b[38;2;86;150;212m■\x1b[38;5;242m■")

	out3 := run(t, uninitPayload(tmp), 0)
	assertContains(t, "uninitialized bar is 20 dim segments", out3, "\x1b[38;5;242m■■■■■■■■■■■■■■■■■■■■")
	assertNotContains(t, "uninitialized bar is not wider than 20 segments", out3, "■■■■■■■■■■■■■■■■■■■■■")
}

func TestCacheHitRate(t *testing.T) {
	tmp := t.TempDir()
	out := run(t, cachePayload(tmp), 0)
	assertContains(t, "cache hit rate computed from current_usage", out, "Cache: 90%")
	assertNotContains(t, "In tokens no longer shown", out, "In:")
	assertNotContains(t, "no placeholders once usage data arrives", out, "--%")
}

func TestModelColor(t *testing.T) {
	tmp := t.TempDir()
	out := run(t, contextPayload(tmp, 10), 0)
	assertContains(t, "model name is green", out, "\x1b[32mTest")
}

func TestEffortBadge(t *testing.T) {
	tmp := t.TempDir()
	out := run(t, effortPayload(tmp), 0)
	assertContains(t, "effort level bracketed after the model name", out,
		"\x1b[32mTest\x1b[0m \x1b[38;5;248m[\x1b[38;5;108mxhigh\x1b[38;5;248m]")

	out2 := stripANSI(run(t, contextPayload(tmp, 10), 0))
	assertNotContains(t, "no effort bracket when effort is absent", out2, "Test [")
}

func TestRateLimitColors(t *testing.T) {
	tmp := t.TempDir()
	assertContains(t, "rate limit usage below 70% is blue", run(t, ratePayload(tmp, 8), 0), "\x1b[34m8% 5h")
	assertContains(t, "rate limit usage 70-84% is yellow", run(t, ratePayload(tmp, 75), 0), "\x1b[33m75% 5h")
	assertContains(t, "rate limit usage 85%+ is red", run(t, ratePayload(tmp, 90), 0), "\x1b[31m90% 5h")
}

func TestPreFirstCallSkeleton(t *testing.T) {
	tmp := t.TempDir()
	out := stripANSI(run(t, uninitPayload(tmp), 0))
	assertContains(t, "skeleton shows 5h rate placeholder", out, "--% 5h")
	assertContains(t, "skeleton shows 7d rate placeholder", out, "--% 7d")
	assertContains(t, "skeleton shows context placeholder", out, "[--%]")
	assertNotContains(t, "skeleton drops the token fraction", out, "(--/")
	assertContains(t, "skeleton shows cache placeholder", out, "Cache: --%")
	assertContains(t, "skeleton shows out placeholder", out, "Out: --")
	assertNotContains(t, "skeleton does not show Out: 0", out, "Out: 0")
}

func TestGradient(t *testing.T) {
	tmp := t.TempDir()
	out := run(t, contextPayload(tmp, 100), 0)
	assertContains(t, "gradient starts blue (truecolor)", out, "\x1b[38;2;58;133;247m■")
	assertContains(t, "gradient passes through yellow", out, "\x1b[38;2;255;255;0m■")
	assertContains(t, "gradient ends orange-red", out, "\x1b[38;2;237;106;44m■")
	assertNotContains(t, "gradient no longer emits xterm-256 codes", out, "\x1b[38;5;33m■")
}

func TestPercentLabelColor(t *testing.T) {
	tmp := t.TempDir()
	assertContains(t, "18% value matches its last filled segment",
		run(t, contextPayload(tmp, 18), 0), "\x1b[38;5;248m[\x1b[38;2;114;168;176m18%\x1b[38;5;248m]")
	assertContains(t, "42% value matches its last filled segment",
		run(t, contextPayload(tmp, 42), 0), "\x1b[38;5;248m[\x1b[38;2;255;255;0m42%\x1b[38;5;248m]")
	assertContains(t, "72% value matches its last filled segment",
		run(t, contextPayload(tmp, 72), 0), "\x1b[38;5;248m[\x1b[38;2;255;165;0m72%\x1b[38;5;248m]")
	assertContains(t, "91% value matches its last filled segment",
		run(t, contextPayload(tmp, 91), 0), "\x1b[38;5;248m[\x1b[38;2;243;126;29m91%\x1b[38;5;248m]")
	assertContains(t, "3% value is blue before any segment fills",
		run(t, contextPayload(tmp, 3), 0), "\x1b[38;5;248m[\x1b[38;2;58;133;247m3%\x1b[38;5;248m]")
}

func TestPRBadge(t *testing.T) {
	tmp := t.TempDir()

	prPayload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"pr":{"number":123,"url":"https://github.com/okize/statusline/pull/123","review_state":"approved"}}`, tmp)
	out := run(t, prPayload, 0)
	assertContains(t, "PR badge shows number", out, "PR #123")
	assertContains(t, "PR badge shows review state", out, "(approved)")
	assertContains(t, "PR badge embeds link url", out, "pull/123")

	noState := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"pr":{"number":7,"url":"https://github.com/okize/statusline/pull/7"}}`, tmp)
	out = run(t, noState, 0)
	assertContains(t, "PR badge renders without review_state", out, "PR #7")
	assertNotContains(t, "no empty parens when review_state absent", out, "()")

	noPR := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q}}`, tmp)
	out = run(t, noPR, 0)
	assertNotContains(t, "no PR badge when pr absent", out, "PR #")
}

// PR badge joins with git stats when both are present (dirty repo cwd).
func TestPRBadgeWithGitStats(t *testing.T) {
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	clone := filepath.Join(base, "clone")
	gitRun(t, "", nil, "init", "-q", "--bare", "-b", "main", origin)
	gitRun(t, "", nil, "clone", "-q", origin, clone)
	gitRun(t, clone, nil, "symbolic-ref", "HEAD", "refs/heads/main")
	gitCommit(t, clone, "base", nil)
	gitRun(t, clone, nil, "push", "-q", "-u", "origin", "main")
	if err := os.WriteFile(filepath.Join(clone, "untracked.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	payload := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"pr":{"number":9,"url":"https://github.com/okize/statusline/pull/9","review_state":"pending"}}`, clone)
	out := run(t, payload, 0)
	assertContains(t, "PR badge present alongside git stats", out, "PR #9")
	assertContains(t, "git stats present alongside PR badge", out, "Unstaged:")
}

func TestWorktreeIndicator(t *testing.T) {
	tmp := t.TempDir()

	linked := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q,"git_worktree":"feature-xyz"}}`, tmp)
	out := run(t, linked, 0)
	assertContains(t, "linked worktree shows tag from workspace.git_worktree", out, "wt:feature-xyz")
	assertNotContains(t, "worktree tag replaces the directory", out, tmp)

	session := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q},"worktree":{"name":"my-feature","path":%q}}`, tmp, tmp)
	out = run(t, session, 0)
	assertContains(t, "--worktree session shows tag from worktree.name", out, "wt:my-feature")
	assertNotContains(t, "worktree tag replaces the directory in --worktree session", out, tmp)

	noWT := fmt.Sprintf(`{"model":{"display_name":"Test"},"workspace":{"current_dir":%q}}`, tmp)
	out = run(t, noWT, 0)
	assertNotContains(t, "no worktree tag outside a worktree", out, "wt:")
}
