// Package statusline renders the Claude Code status line from the stdin JSON
// payload. It is the implementation behind the statusline binary; see the repo
// README and CLAUDE.md for the output contract.
package statusline

// Render decodes the stdin JSON payload and returns the full multi-line status
// output (leading blank line, model/context line, location+branch line, and the
// optional PR+stats line). columns is the terminal width; 0 means no truncation.
func Render(data []byte, columns int) (string, error) {
	in, err := parseInput(data)
	if err != nil {
		return "", err
	}
	return renderMain(in, columns), nil
}

// RenderGit returns the two git lines for cwd — line 1 (branch + ahead/behind +
// sync) and line 2 (ticket link + change stats) — backing the `statusline git
// <dir>` subcommand. columns is the terminal width; 0 means no truncation.
func RenderGit(cwd string, columns int) (line1, line2 string) {
	return renderGitLines(cwd, columns)
}
