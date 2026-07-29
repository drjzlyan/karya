package editor

import "testing"

func TestEscapeForVim(t *testing.T) {
	cases := map[string]string{
		"file.go":    "file.go",
		"my file.go": `my\ file.go`,
		"a#b.go":     `a\#b.go`,
		"100%.txt":   `100\%.txt`,
		"a b#c%d":    `a\ b\#c\%d`,
	}
	for in, want := range cases {
		if got := escapeForVim(in); got != want {
			t.Errorf("escapeForVim(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"/tmp/dir": "'/tmp/dir'",
		"a b":      "'a b'",
		"it's":     `'it'\''s'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
