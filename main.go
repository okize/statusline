package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"statusline/internal/statusline"
)

// columnsFromEnv reads the terminal width Claude Code exports as COLUMNS.
// Returns 0 (meaning "no truncation") when unset, non-numeric, or non-positive.
func columnsFromEnv() int {
	v := os.Getenv("COLUMNS")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func main() {
	cols := columnsFromEnv()

	// `statusline git <dir>` prints the two git lines (parity with the old
	// statusline-git.sh); no args reads the JSON payload from stdin.
	if len(os.Args) > 1 && os.Args[1] == "git" {
		cwd := ""
		if len(os.Args) > 2 {
			cwd = os.Args[2]
		}
		l1, l2 := statusline.RenderGit(cwd, cols)
		_, _ = fmt.Println(l1)
		_, _ = fmt.Println(l2)
		return
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	out, err := statusline.Render(data, cols)
	if err != nil {
		return
	}
	_, _ = fmt.Print(out)
}
