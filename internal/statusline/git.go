package statusline

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// runGit runs `git --no-optional-locks -C cwd <args>` and returns its stdout
// and whether it exited 0. Mirrors run_git() in statusline-git.sh (stderr is
// discarded, as the bash callers did with 2>/dev/null).
func runGit(cwd string, args ...string) (string, bool) {
	full := append([]string{"--no-optional-locks", "-C", cwd}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// renderGitLines produces the two git lines for cwd. Line 1 is branch +
// ahead/behind + sync status; line 2 is the ticket link + change stats, or
// "No pending changes". A non-repo yields ("not a git repo", "").
func renderGitLines(cwd string, columns int) (string, string) {
	if !gitIsRepo(cwd) {
		return "not a git repo", ""
	}

	branch := gitBranch(cwd)

	// Width-aware truncation of the DISPLAYED branch; detection below keeps the
	// full branch name.
	branchDisplay := branch
	if columns > 0 {
		branchMax := columns / 4
		if branchMax < 15 {
			branchMax = 15
		}
		branchDisplay = truncateMiddle(branch, branchMax)
	}

	ab := gitAheadBehind(cwd)
	syncColor, sync := gitSyncStatus(cwd)
	line1 := branchDisplay + ab + " • " + syncColor + sync + reset

	cs := gitChangeStats(cwd)

	label, text, url := detectTicket(branch)
	ticketLink := ""
	if url != "" {
		ticketLink = label + ": " + linkBlue + " \x1b]8;;" + url + "\a" + text + "\x1b]8;;\a" + reset + " | "
	}

	var line2 string
	if cs != "" {
		line2 = ticketLink + cs
	} else {
		line2 = ticketLink + lightGrey + "No pending changes" + reset
	}
	return line1, line2
}

func gitIsRepo(cwd string) bool {
	if cwd == "" {
		return false
	}
	if fi, err := os.Stat(filepath.Join(cwd, ".git")); err == nil && fi.IsDir() {
		return true
	}
	_, ok := runGit(cwd, "rev-parse", "--git-dir")
	return ok
}

func gitBranch(cwd string) string {
	out, _ := runGit(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out)
}

// gitAheadBehind renders " ↑N ↓M" (each only when non-zero) vs the upstream, or
// "" when there is no upstream. Guards against a deleted remote branch causing
// git to echo the literal "@{u}".
func gitAheadBehind(cwd string) string {
	up, ok := runGit(cwd, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if !ok {
		return ""
	}
	upstream := strings.TrimSpace(up)
	if upstream == "" || strings.Contains(upstream, "@{u}") {
		return ""
	}

	// rev-list --left-right --count '@{u}...HEAD' outputs "behind<TAB>ahead".
	counts, ok := runGit(cwd, "rev-list", "--left-right", "--count", "@{u}...HEAD")
	if !ok {
		return ""
	}
	fields := strings.Fields(counts)
	if len(fields) != 2 {
		return ""
	}
	behind, _ := strconv.Atoi(fields[0])
	ahead, _ := strconv.Atoi(fields[1])

	disp := ""
	if ahead > 0 {
		disp += " " + mutedGreen + "↑" + strconv.Itoa(ahead) + reset
	}
	if behind > 0 {
		disp += " " + mutedRed + "↓" + strconv.Itoa(behind) + reset
	}
	return disp
}

// gitSyncStatus turns the last commit time into a relative "synced X ago"
// string plus its color. Negative diffs (commit dated ahead of the clock) clamp
// to 0. Color escalates to yellow at >= 1 day stale.
func gitSyncStatus(cwd string) (color, status string) {
	out, ok := runGit(cwd, "log", "-1", "--format=%ct")
	last := strings.TrimSpace(out)
	if !ok || last == "" {
		return lightGrey, "no commits"
	}
	commitTime, err := strconv.ParseInt(last, 10, 64)
	if err != nil {
		return lightGrey, "no commits"
	}

	diff := time.Now().Unix() - commitTime
	if diff < 0 {
		diff = 0
	}

	switch {
	case diff < 60:
		status = "synced " + strconv.FormatInt(diff, 10) + "s ago"
	case diff < 3600:
		status = "synced " + strconv.FormatInt(diff/60, 10) + "m ago"
	case diff < 86400:
		status = "synced " + strconv.FormatInt(diff/3600, 10) + "h ago"
	default:
		status = "synced " + strconv.FormatInt(diff/86400, 10) + "d ago"
	}

	color = lightGrey
	if diff >= 86400 {
		color = yellow
	}
	return color, status
}

var insertionRE = regexp.MustCompile(`([0-9]+) insertion`)
var deletionRE = regexp.MustCompile(`([0-9]+) deletion`)

// gitChangeStats parses `status --porcelain` once for staged/unstaged counts,
// then runs `diff --shortstat` only when a count is > 0, and renders the
// "Staged: N • (+I/-D) | Unstaged: N • (+I/-D)" summary (or "" when clean).
func gitChangeStats(cwd string) string {
	porcelain, _ := runGit(cwd, "status", "--porcelain")
	if strings.TrimSpace(porcelain) == "" {
		return ""
	}

	staged, unstagedMod, untracked := 0, 0, 0
	for _, l := range strings.Split(strings.TrimRight(porcelain, "\n"), "\n") {
		if l == "" {
			continue
		}
		if strings.IndexByte("MADRC", l[0]) >= 0 { // staged: first char [MADRC]
			staged++
		}
		if len(l) >= 2 && (l[1] == 'M' || l[1] == 'D') { // unstaged-modified: second char [MD]
			unstagedMod++
		}
		if strings.HasPrefix(l, "??") { // untracked
			untracked++
		}
	}
	unstaged := unstagedMod + untracked

	stats := ""
	if staged > 0 {
		ins, del := gitShortstat(cwd, true)
		stats = "Staged: " + lightGrey + " " + strconv.Itoa(staged) + reset +
			" • (" + mutedGreen + "+" + ins + reset + "/" + mutedRed + "-" + del + reset + ")"
	}
	if unstaged > 0 {
		ins, del := gitShortstat(cwd, false)
		part := "Unstaged: " + lightGrey + " " + strconv.Itoa(unstaged) + reset +
			" • (" + mutedGreen + "+" + ins + reset + "/" + mutedRed + "-" + del + reset + ")"
		if stats != "" {
			stats = stats + " | " + part
		} else {
			stats = part
		}
	}
	return stats
}

// gitShortstat extracts insertion/deletion counts from `diff [--cached]
// --shortstat`, defaulting each to "0" when absent.
func gitShortstat(cwd string, cached bool) (ins, del string) {
	args := []string{"diff", "--shortstat"}
	if cached {
		args = []string{"diff", "--cached", "--shortstat"}
	}
	out, _ := runGit(cwd, args...)
	ins, del = "0", "0"
	if m := insertionRE.FindStringSubmatch(out); m != nil {
		ins = m[1]
	}
	if m := deletionRE.FindStringSubmatch(out); m != nil {
		del = m[1]
	}
	return ins, del
}
