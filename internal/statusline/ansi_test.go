package statusline

import "testing"

func TestTruncateMiddle(t *testing.T) {
	assertEqual(t, "truncate_middle shortens long string with middle ellipsis",
		truncateMiddle("abcdefghijklmnop", 9), "abcd…mnop")
	assertEqual(t, "truncate_middle leaves short string unchanged",
		truncateMiddle("abc", 9), "abc")
}
