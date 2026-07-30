package cli

import (
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	cases := []struct {
		name, input string
		want        bool
	}{
		{"y", "y\n", true},
		{"yes", "yes\n", true},
		{"uppercase Y", "Y\n", true},
		{"empty defaults no", "\n", false},
		{"n", "n\n", false},
		{"garbage", "maybe\n", false},
		{"eof defaults no", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := confirm(strings.NewReader(c.input), "Continue?"); got != c.want {
				t.Errorf("confirm(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}
