package main

import (
	"os"
	"testing"
)

// columnsFromEnv is the only branching logic in package main: COLUMNS unset,
// non-numeric, or non-positive must all mean "no truncation" (0). Anything else
// silently disables truncation or passes a bogus width to the renderer.
func TestColumnsFromEnv(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		val  string
		want int
	}{
		{name: "unset", set: false, want: 0},
		{name: "empty", set: true, val: "", want: 0},
		{name: "non-numeric", set: true, val: "wide", want: 0},
		{name: "float is not an int", set: true, val: "120.5", want: 0},
		{name: "zero", set: true, val: "0", want: 0},
		{name: "negative", set: true, val: "-80", want: 0},
		{name: "valid width", set: true, val: "120", want: 120},
		{name: "leading plus", set: true, val: "+80", want: 80},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// t.Setenv cannot unset, but it does register a cleanup that
			// restores the original value — so setting then unsetting is safe.
			t.Setenv("COLUMNS", c.val)
			if !c.set {
				if err := os.Unsetenv("COLUMNS"); err != nil {
					t.Fatal(err)
				}
			}
			if got := columnsFromEnv(); got != c.want {
				t.Errorf("columnsFromEnv() with COLUMNS=%q = %d, want %d", c.val, got, c.want)
			}
		})
	}
}
