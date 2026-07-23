package statusline

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// A commit dated in the future must not produce a negative age.
func TestSyncAgeFutureCommit(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "future-repo")
	gitInit(t, repo)
	future := time.Now().Add(24 * time.Hour).Unix()
	dateEnv := fmt.Sprintf("%d +0000", future)
	gitCommit(t, repo, "future commit", []string{
		"GIT_AUTHOR_DATE=" + dateEnv,
		"GIT_COMMITTER_DATE=" + dateEnv,
	})
	l1, _ := renderGitLines(repo, 0)
	assertNotContains(t, "future-dated commit does not render negative sync age", l1, "synced -")
	assertContains(t, "future-dated commit clamps to 0s ago", l1, "synced 0s ago")
}

func TestAheadBehind(t *testing.T) {
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	clone := filepath.Join(base, "clone")
	clone2 := filepath.Join(base, "clone2")

	gitRun(t, "", nil, "init", "-q", "--bare", "-b", "main", origin)
	gitRun(t, "", nil, "clone", "-q", origin, clone)
	gitRun(t, clone, nil, "symbolic-ref", "HEAD", "refs/heads/main")
	gitCommit(t, clone, "base", nil)
	gitRun(t, clone, nil, "push", "-q", "-u", "origin", "main")

	out := gitLines(clone, 0)
	assertNotContains(t, "in-sync branch shows no ahead marker", out, "↑")
	assertNotContains(t, "in-sync branch shows no behind marker", out, "↓")
	assertNotContains(t, "no Upstream label", out, "Upstream")
	assertNotContains(t, "no ticket link for a branch without a ticket id", out, "Shortcut:")

	gitCommit(t, clone, "local work", nil)
	out = stripANSI(gitLines(clone, 0))
	assertContains(t, "unpushed commit shows ahead count after branch name", out, "main ↑1 •")
	assertNotContains(t, "no behind marker when only ahead", out, "↓")

	gitRun(t, "", nil, "clone", "-q", origin, clone2)
	gitCommit(t, clone2, "remote work", nil)
	gitRun(t, clone2, nil, "push", "-q", "origin", "main")
	gitRun(t, clone, nil, "fetch", "-q", "origin")
	out = stripANSI(gitLines(clone, 0))
	assertContains(t, "diverged branch shows both counts after branch name", out, "main ↑1 ↓1 •")
}

// The git renderer must always yield exactly two lines with no embedded
// newlines (main.sh split them positionally); a non-repo yields the notice + "".
func TestTwoLineContract(t *testing.T) {
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	clone := filepath.Join(base, "clone")
	gitRun(t, "", nil, "init", "-q", "--bare", "-b", "main", origin)
	gitRun(t, "", nil, "clone", "-q", origin, clone)
	gitRun(t, clone, nil, "symbolic-ref", "HEAD", "refs/heads/main")
	gitCommit(t, clone, "base", nil)
	gitRun(t, clone, nil, "push", "-q", "-u", "origin", "main")

	l1, l2 := renderGitLines(clone, 60)
	assertNotContains(t, "git line 1 must not embed a newline", l1, "\n")
	assertNotContains(t, "git line 2 must not embed a newline", l2, "\n")

	nl1, nl2 := renderGitLines(t.TempDir(), 0)
	assertEqual(t, "non-repo line 1 is the not-a-git-repo notice", nl1, "not a git repo")
	assertEqual(t, "non-repo line 2 is empty", nl2, "")
}

func TestBranchTruncation(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	gitInit(t, repo)
	gitCommit(t, repo, "base", nil)
	longBranch := "feature/very-long-branch-name-for-truncation-testing"
	gitRun(t, repo, nil, "checkout", "-q", "-b", longBranch)

	l1, _ := renderGitLines(repo, 60)
	assertContains(t, "long branch truncated when COLUMNS set", l1, "…")
	assertNotContains(t, "full branch absent when COLUMNS set", l1, longBranch)

	l1, _ = renderGitLines(repo, 0)
	assertContains(t, "full branch shown when COLUMNS unset", l1, longBranch)
}

// Shortcut detection must use the full branch name, not the truncated display,
// and build the link from the configured org.
func TestShortcutDetection(t *testing.T) {
	t.Setenv("STATUSLINE_SHORTCUT_ORG", "acme")
	repo := filepath.Join(t.TempDir(), "repo")
	gitInit(t, repo)
	gitCommit(t, repo, "base", nil)
	scBranch := "feature/sc-54321-very-long-description-of-the-work"
	gitRun(t, repo, nil, "checkout", "-q", "-b", scBranch)

	_, l2 := renderGitLines(repo, 60)
	assertContains(t, "Shortcut link built from full branch when display truncated",
		l2, "app.shortcut.com/acme/story/54321")
}

// With no org configured, an sc- branch produces no ticket link.
func TestShortcutNoOrgNoLink(t *testing.T) {
	t.Setenv("STATUSLINE_SHORTCUT_ORG", "")
	repo := filepath.Join(t.TempDir(), "repo")
	gitInit(t, repo)
	gitCommit(t, repo, "base", nil)
	gitRun(t, repo, nil, "checkout", "-q", "-b", "feature/sc-54321-work")

	_, l2 := renderGitLines(repo, 0)
	assertNotContains(t, "no Shortcut link when org unset", l2, "app.shortcut.com")
	assertContains(t, "clean repo shows no pending changes", l2, "No pending changes")
}
