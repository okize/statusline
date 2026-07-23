package statusline

import (
	"os"
	"regexp"
)

var scRE = regexp.MustCompile(`sc-[0-9]+`)
var digitsRE = regexp.MustCompile(`[0-9]+`)

// detectTicket maps a branch name to a ticket link, chaining detectTicket<Name>
// functions; first match wins. Returns (label, text, url), or all-empty when no
// tracker matches. To add a tracker (Linear, Asana, ...), write a new detector
// and add it to the chain.
func detectTicket(branch string) (label, text, url string) {
	if l, t, u := detectTicketShortcut(branch); u != "" {
		return l, t, u
	}
	return "", "", ""
}

// detectTicketShortcut links branches containing sc-##### to a Shortcut story.
// Shortcut deep links require an org slug, read from the STATUSLINE_SHORTCUT_ORG
// environment variable; with no org set, no link is produced.
func detectTicketShortcut(branch string) (label, text, url string) {
	match := scRE.FindString(branch)
	if match == "" {
		return "", "", ""
	}
	org := os.Getenv("STATUSLINE_SHORTCUT_ORG")
	if org == "" {
		return "", "", ""
	}
	num := digitsRE.FindString(match)
	return "Shortcut", match, "https://app.shortcut.com/" + org + "/story/" + num
}
