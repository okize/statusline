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

// contextGradient is the fixed positional context-bar gradient: one 24-bit RGB
// triple per segment, a smooth blue -> yellow -> orange ramp. These are the exact
// values the gradient-string demo emits across four stops (#3a85f7, yellow,
// orange, #ed6a2c). Rendered as truecolor SGR (\x1b[38;2;R;G;Bm) so the 20
// segments interpolate smoothly instead of banding into xterm-256 steps.
var contextGradient = [20][3]int{
	{58, 133, 247}, {86, 150, 212}, {114, 168, 176}, {142, 185, 141},
	{171, 203, 106}, {199, 220, 71}, {227, 238, 35}, {255, 255, 0},
	{255, 240, 0}, {255, 225, 0}, {255, 210, 0}, {255, 195, 0},
	{255, 180, 0}, {255, 165, 0}, {252, 155, 7}, {249, 145, 15},
	{246, 136, 22}, {243, 126, 29}, {240, 116, 37}, {237, 106, 44},
}

// fgRGB returns the 24-bit truecolor foreground SGR sequence for an RGB triple.
func fgRGB(c [3]int) string {
	return "\x1b[38;2;" + strconv.Itoa(c[0]) + ";" + strconv.Itoa(c[1]) + ";" + strconv.Itoa(c[2]) + "m"
}

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
