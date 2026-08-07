// Package companionview is karya's read-only Companion agent pane (DESIGN.md §6):
// a code companion the human asks questions of from the Human-in-Control view. It
// answers about the task, repo, and code but never edits files — file-changing
// agents run headlessly from the Multi-Agent view. It is a thin view: the answer
// is produced by an injected asker (the IDE wires it to a headless agent runner),
// so the pane is decoupled from the agent backend and hermetically testable.
package companionview

import (
	"strings"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

// Companion is the companion Q&A pane.
type Companion struct {
	agent   string   // the backing agent's name (for the header), or "" (none)
	lines   []string // the running transcript (questions + answers)
	input   string   // the question being typed
	pending bool     // true while awaiting an answer
	scroll  int      // number of lines scrolled up from the bottom
	ask     string   // set when a question is submitted, drained by AskRequest
	closed  bool
}

// New builds a companion pane backed by the named agent ("" when none detected).
func New(agent string) *Companion {
	c := &Companion{agent: agent}
	if agent == "" {
		c.lines = []string{"No coding agent detected — install one to ask the companion."}
	} else {
		c.lines = []string{"Companion ready (" + agent + "). Ask a question about the task or repo."}
	}
	return c
}

// Done reports whether the pane asked to close.
func (c *Companion) Done() bool { return c.closed }

// AskRequest returns (once) a submitted question, or "". The IDE runs it through
// a headless agent and returns the reply via Answer.
func (c *Companion) AskRequest() string {
	q := c.ask
	c.ask = ""
	return q
}

// Answer records the agent's reply (or an error) and clears the pending state.
func (c *Companion) Answer(text string, err error) {
	c.pending = false
	if err != nil {
		c.lines = append(c.lines, "! "+err.Error())
		return
	}
	c.lines = append(c.lines, strings.Split(strings.TrimRight(text, "\n"), "\n")...)
	c.scroll = 0
}

// HandleKey processes a forwarded key.
func (c *Companion) HandleKey(k term.Key) {
	switch {
	case k == term.Named(term.SymEsc):
		if c.input != "" {
			c.input = ""
			return
		}
		c.closed = true
	case k == term.Named(term.SymEnter):
		if c.pending || strings.TrimSpace(c.input) == "" || c.agent == "" {
			return
		}
		c.lines = append(c.lines, "› "+c.input)
		c.ask = c.input
		c.input = ""
		c.pending = true
		c.scroll = 0
	case k == term.Named(term.SymBackspace):
		if c.input != "" {
			c.input = c.input[:len(c.input)-1]
		}
	case k == term.Ctrl('u'):
		c.scroll += 3
	case k == term.Ctrl('d'):
		c.scroll = max(0, c.scroll-3)
	case k.IsRune() && k.Mod == 0:
		c.input += string(k.Rune)
	}
}

// View renders the pane into rect. It satisfies layout.PaneContent.
func (c *Companion) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W < 4 || r.H < 2 {
		return
	}
	title := "  Companion"
	if c.agent != "" {
		title += " · " + c.agent
	}
	buf.SetString(r.X, r.Y, fit(title, r.W), cellbuf.Style{Attrs: cellbuf.AttrBold})

	bottomY := r.Y + r.H - 1
	listY := r.Y + 1
	listH := bottomY - listY
	if listH < 1 {
		return
	}

	// Show a window of the transcript ending c.scroll lines above the bottom.
	end := len(c.lines) - c.scroll
	if end < 0 {
		end = 0
	}
	start := end - listH
	if start < 0 {
		start = 0
	}
	for i, ln := range c.lines[start:end] {
		buf.SetString(r.X, listY+i, fit(ln, r.W), lineStyle(ln))
	}

	// Bottom input / status line.
	prompt := "› " + c.input + "_"
	if c.pending {
		prompt = "… thinking"
	} else if c.agent == "" {
		prompt = "(no agent)"
	}
	st := cellbuf.Style{Attrs: cellbuf.AttrReverse}
	buf.Fill(cellbuf.Rect{X: r.X, Y: bottomY, W: r.W, H: 1}, cellbuf.Cell{Rune: ' ', Width: 1, Style: st})
	buf.SetString(r.X, bottomY, fit(prompt, r.W), st)
}

// lineStyle dims agent/system lines and highlights the user's questions.
func lineStyle(ln string) cellbuf.Style {
	if strings.HasPrefix(ln, "› ") {
		return cellbuf.Style{FG: cellbuf.Palette(6)} // cyan question
	}
	if strings.HasPrefix(ln, "! ") {
		return cellbuf.Style{FG: cellbuf.Palette(1)} // red error
	}
	return cellbuf.Style{}
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) > w {
		return s[:w]
	}
	return s
}
