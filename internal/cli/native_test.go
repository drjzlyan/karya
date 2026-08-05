package cli

import (
	"bufio"
	"strings"
	"testing"
)

func TestAskYesNo(t *testing.T) {
	cases := map[string]bool{
		"y\n":     true,
		"yes\n":   true,
		"Y\n":     true,
		"YES\n":   true,
		"n\n":     false,
		"\n":      false,
		"maybe\n": false,
		"":        false, // EOF with no input → decline
	}
	for in, want := range cases {
		r := bufio.NewReader(strings.NewReader(in))
		if got := askYesNo(r, "ok?"); got != want {
			t.Errorf("askYesNo(%q) = %v, want %v", in, got, want)
		}
	}
}
