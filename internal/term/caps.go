package term

import "strings"

// Caps describes what the terminal can render. karya keeps a small built-in
// notion of capability rather than parsing terminfo: it needs only color depth
// and truecolor support to emit correct SGR.
type Caps struct {
	TrueColor bool // 24-bit color via COLORTERM
	Palette   int  // 256, 16, or 8
}

// DetectCaps derives Caps from the environment. getenv is injected so detection
// is a pure function and testable (pass os.Getenv in production).
func DetectCaps(getenv func(string) string) Caps {
	term := getenv("TERM")
	colorterm := strings.ToLower(getenv("COLORTERM"))

	caps := Caps{Palette: 8}
	switch {
	case term == "":
		caps.Palette = 8
	case strings.Contains(term, "256color"), strings.Contains(term, "direct"):
		caps.Palette = 256
	case term == "dumb":
		caps.Palette = 8
	default:
		caps.Palette = 16
	}
	if colorterm == "truecolor" || colorterm == "24bit" {
		caps.TrueColor = true
		caps.Palette = 256
	}
	return caps
}
