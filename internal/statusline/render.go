package statusline

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// renderMain builds the full multi-line status output: a leading blank line,
// line 1 (model + effort + rate limits + context bar + cache + Out), the
// location+branch line, and the optional PR+stats line. Shells out to the git
// renderer for the git portions. columns == 0 means "no truncation".
func renderMain(in *Input, columns int) string {
	// Reasoning effort badge (absent when the model doesn't support effort).
	effortDisplay := ""
	if in.EffortLevel != "" {
		effortDisplay = " " + lightGrey + "[" + mutedGreen + in.EffortLevel + lightGrey + "]" + reset
	}

	// Current directory, with $HOME collapsed to ~ and width-aware truncation.
	currentFolder := collapseHome(in.CWD)
	if columns > 0 {
		folderMax := columns / 3
		if folderMax < 20 {
			folderMax = 20
		}
		currentFolder = truncateMiddle(currentFolder, folderMax)
	}

	// current_usage is null (all zero) before the first API call and after
	// /compact — render a skeleton in that case.
	initialized := in.CurInput != 0 || in.CurCacheCreate != 0 || in.CurCacheRead != 0 || in.CurOutput != 0

	usedPct := int(in.UsedPct) // truncate toward zero, matching ${used_percentage%.*}
	if usedPct <= 0 && initialized {
		tokensInContext := in.CurInput + in.CurCacheCreate + in.CurCacheRead
		if in.ContextSize > 0 && tokensInContext > 0 {
			usedPct = tokensInContext * 100 / in.ContextSize
		}
	}
	contextDisplay := buildContextDisplay(usedPct, initialized)

	// Cache hit rate of the most recent API call (cache_read / total input).
	totalIn := in.CurInput + in.CurCacheCreate + in.CurCacheRead
	cacheDisplay := " • " + lightGrey + "Cache: --%" + reset
	if totalIn > 0 {
		cachePct := in.CurCacheRead * 100 / totalIn
		cacheDisplay = " • " + lightGrey + "Cache: " + strconv.Itoa(cachePct) + "%" + reset
	}

	tokensOut := "--"
	if initialized {
		tokensOut = formatTokens(in.CurOutput)
	}

	rateDisplay := buildRateDisplay(in, initialized)

	// Inside a worktree the [wt:name] tag replaces the directory.
	locationDisplay := currentFolder
	if in.WorktreeName != "" {
		locationDisplay = orange + "[wt:" + in.WorktreeName + "]" + reset
	}

	prDisplay := buildPRBadge(in)

	gitBranchLine, gitStatsLine := renderGitLines(in.CWD, columns)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(green + in.ModelName + reset + effortDisplay + " | " + rateDisplay +
		contextDisplay + cacheDisplay + " • " + lightGrey + "Out: " + tokensOut + reset + "\n")
	sb.WriteString(locationDisplay + " | " + gitBranchLine + "\n")

	statsLine := gitStatsLine
	if prDisplay != "" {
		if statsLine != "" {
			statsLine = prDisplay + " | " + statsLine
		} else {
			statsLine = prDisplay
		}
	}
	if statsLine != "" {
		sb.WriteString(statsLine + "\n")
	}
	return sb.String()
}

// buildContextDisplay renders the 20-segment context bar. Filled segments form
// the fixed gradient; empty segments are dim grey; the bracketed percentage
// takes the color of the last filled segment. Uninitialized renders a dim
// skeleton with a [--%] placeholder.
func buildContextDisplay(pct int, initialized bool) string {
	const barLength = 20

	if !initialized {
		return dimGrey + strings.Repeat("■", barLength) + reset + " " + lightGrey + "[--%]" + reset
	}

	filled := pct * barLength / 100
	if filled > barLength {
		filled = barLength
	}

	var bar strings.Builder
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar.WriteString(fgRGB(contextGradient[i]) + "■")
		} else {
			bar.WriteString(dimGrey + "■")
		}
	}

	labelIdx := filled - 1
	if labelIdx < 0 {
		labelIdx = 0
	}
	labelColor := fgRGB(contextGradient[labelIdx])

	return bar.String() + reset + " " + lightGrey + "[" + labelColor + strconv.Itoa(pct) + "%" + lightGrey + "]" + reset
}

// buildRateDisplay renders the 5h/7d rate-limit segment. Before the first API
// call (uninitialized and no rate data) it shows the "--% 5h | --% 7d | "
// skeleton; once rate data is present it shows whichever windows exist, colored
// by usage, with a trailing " | ". API-key users (initialized, no rate data)
// get "".
func buildRateDisplay(in *Input, initialized bool) string {
	if !initialized && in.RateFivePct == nil && in.RateSevenPct == nil {
		return lightGrey + "--% 5h" + reset + " | " + lightGrey + "--% 7d" + reset + " | "
	}
	if in.RateFivePct == nil && in.RateSevenPct == nil {
		return ""
	}

	var parts []string
	if in.RateFivePct != nil {
		parts = append(parts, rateSegment(*in.RateFivePct, "5h", in.RateFiveReset))
	}
	if in.RateSevenPct != nil {
		parts = append(parts, rateSegment(*in.RateSevenPct, "7d", in.RateSevenReset))
	}
	return strings.Join(parts, " | ") + " | "
}

func rateSegment(pct float64, window string, resetAt *int64) string {
	pctStr := fmt.Sprintf("%.0f", pct)
	pctInt, _ := strconv.Atoi(pctStr)
	color := rateLimitColor(pctInt)
	label := formatResetTime(resetAt, window)
	if label != "" {
		return color + pctStr + "% " + window + " (" + label + ")" + reset
	}
	return color + pctStr + "% " + window + reset
}

// buildPRBadge renders the "PR #N (state)" badge (clickable when a url is
// present), or "" when there is no open PR. review_state may be absent.
func buildPRBadge(in *Input) string {
	if in.PRNumber == nil {
		return ""
	}
	prText := "PR #" + strconv.FormatInt(*in.PRNumber, 10)
	if in.PRUrl != "" {
		prText = "\x1b]8;;" + in.PRUrl + "\a" + prText + "\x1b]8;;\a"
	}
	disp := linkBlue + prText + reset

	if in.PRState != "" {
		var color string
		switch in.PRState {
		case "approved":
			color = mutedGreen
		case "changes_requested":
			color = mutedRed
		case "draft":
			color = lightGrey
		default:
			color = yellow
		}
		disp += " " + color + "(" + in.PRState + ")" + reset
	}
	return disp
}

// collapseHome replaces a leading $HOME with ~, matching bash ${cwd/#$HOME/~}.
func collapseHome(path string) string {
	home := os.Getenv("HOME")
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
