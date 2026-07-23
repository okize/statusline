package statusline

import (
	"strconv"
	"time"
)

// ANSI palette (ported from lib.sh). Written as raw escape bytes so the output
// is byte-identical to what echo -e / printf '%b' produced. Fixed xterm-256
// codes; they do not remap with the terminal theme. (CYAN and WHITE from the
// original palette were unreferenced and are intentionally dropped.)
const (
	blue       = "\x1b[34m"
	green      = "\x1b[32m"
	yellow     = "\x1b[33m"
	orange     = "\x1b[38;5;208m"
	red        = "\x1b[31m"
	lightGrey  = "\x1b[38;5;248m"
	dimGrey    = "\x1b[38;5;242m"
	linkBlue   = "\x1b[94m"
	mutedGreen = "\x1b[38;5;108m"
	mutedRed   = "\x1b[38;5;167m"
	reset      = "\x1b[0m"
)

// contextGradient is the fixed positional blue -> gold -> orange gradient, one
// xterm-256 code per bar segment (modeled on abtop's context meter).
var contextGradient = [20]int{33, 33, 74, 73, 109, 108, 143, 179, 178, 220, 220, 220, 214, 214, 214, 208, 208, 208, 202, 202}

// truncateMiddle shortens s to at most max characters, replacing the middle
// with "…" so both ends stay readable. Counts runes (not display columns), like
// the original bash helper. Must be called on plain text, before ANSI codes.
func truncateMiddle(s string, max int) string {
	r := []rune(s)
	length := len(r)
	if length <= max || max < 5 {
		return s
	}
	keep := max - 1
	head := (keep + 1) / 2
	tail := keep - head
	return string(r[:head]) + "…" + string(r[length-tail:])
}

// formatTokens renders a token count (e.g. 42000 -> "42k"), integer-truncated.
func formatTokens(t int) string {
	if t >= 1000 {
		return strconv.Itoa(t/1000) + "k"
	}
	return strconv.Itoa(t)
}

// rateLimitColor thresholds usage: 0-69% blue, 70-84% yellow, 85%+ red.
func rateLimitColor(pct int) string {
	if pct < 70 {
		return blue
	} else if pct < 85 {
		return yellow
	}
	return red
}

// formatResetTime formats a reset timestamp for display. 5h -> "3:04 PM";
// 7d -> "1/2/06 3:04 PM". Returns "" when the timestamp is absent. Uses local
// time, replacing the BSD-only `date -r` (this is what makes the port portable).
func formatResetTime(epoch *int64, window string) string {
	if epoch == nil {
		return ""
	}
	return time.Unix(*epoch, 0).Local().Format(resetLayout(window))
}

// resetLayout is the Go time layout for each reset window: 5h -> "3:04 PM",
// 7d -> "1/2/06 3:04 PM" (replacing bash's `%-I:%M %p` / `%-m/%-d/%y %-I:%M %p`).
func resetLayout(window string) string {
	if window == "5h" {
		return "3:04 PM"
	}
	return "1/2/06 3:04 PM"
}
