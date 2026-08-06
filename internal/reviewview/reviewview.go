// Package reviewview renders a task's assembled review (spec, plan, diff,
// evidence) as a scrollable in-IDE surface, the human's read side of a gate
// (DESIGN.md §6). Approve/reject remain explicit crossings (`karya gate`); this
// view is what the human reads to decide. It is a thin view over
// internal/review + internal/diffview and is snapshot-testable.
package reviewview

import (
	"strings"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/diffview"
	"github.com/drjzlyan/karya/internal/review"
	"github.com/drjzlyan/karya/internal/term"
)

type line struct {
	text  string
	style cellbuf.Style
}

// Crosser crosses the review's gate. The Model supplies it so the human can
// approve/reject without leaving the review (nil makes the view read-only).
type Crosser interface {
	Approve(id string) error
	Reject(id, feedback string) error
}

type mode uint8

const (
	modeNormal mode = iota
	modeReject
)

// Panel is the scrollable review view.
type Panel struct {
	id       string
	hasGate  bool
	lines    []line
	scroll   int
	crosser  Crosser
	mode     mode
	feedback string
	status   string
	closed   bool
}

// New builds a review panel from an assembled review. crosser may be nil, which
// makes the view read-only (approve/reject disabled).
func New(r *review.Review, crosser Crosser) *Panel {
	p := &Panel{id: r.Task.ID, hasGate: r.HasGate, crosser: crosser}
	p.build(r)
	return p
}

// Done reports whether the panel asked to close.
func (p *Panel) Done() bool { return p.closed }

func (p *Panel) build(r *review.Review) {
	bold := cellbuf.Style{Attrs: cellbuf.AttrBold}
	dim := cellbuf.Style{FG: cellbuf.Palette(8)}

	p.add("Task "+r.Task.ID+"  ["+string(r.Task.State)+"]", bold)
	if r.HasGate {
		p.add("Gate: "+string(r.Pending.Gate)+"  (approve → "+string(r.Pending.Approve)+
			", reject → "+string(r.Pending.Reject)+")", dim)
	}
	if r.Spec != nil && strings.TrimSpace(r.Spec.Objective) != "" {
		p.section("Objective")
		p.text(r.Spec.Objective, cellbuf.Style{})
	}
	if r.Spec != nil && len(r.Spec.Criteria) > 0 {
		p.section("Acceptance criteria")
		for _, c := range r.Spec.Criteria {
			mark := "[ ]"
			if c.Checked {
				mark = "[x]"
			}
			p.add("  "+mark+" "+c.Text, cellbuf.Style{})
		}
	}
	if strings.TrimSpace(r.Plan) != "" {
		p.section("Plan")
		p.text(r.Plan, cellbuf.Style{})
	}
	if strings.TrimSpace(r.Diff) != "" {
		p.section("Diff")
		for _, dl := range diffview.Parse(r.Diff) {
			p.add(dl.Text, diffStyle(dl.Kind))
		}
	}
	for i, e := range r.Evidence {
		p.section("Verification " + itoa(i+1))
		p.text(e, cellbuf.Style{})
	}
	if r.HasGate {
		p.add("", cellbuf.Style{})
		p.add("approve: karya gate approve "+r.Task.ID+"   reject: karya gate reject "+r.Task.ID+" <feedback>", dim)
	}
}

func (p *Panel) add(text string, st cellbuf.Style) { p.lines = append(p.lines, line{text, st}) }

func (p *Panel) section(name string) {
	p.add("", cellbuf.Style{})
	p.add("## "+name, cellbuf.Style{Attrs: cellbuf.AttrBold, FG: cellbuf.Palette(6)})
}

func (p *Panel) text(s string, st cellbuf.Style) {
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		p.add(l, st)
	}
}

// HandleKey scrolls, crosses the gate, or closes the panel.
func (p *Panel) HandleKey(k term.Key) {
	if p.mode == modeReject {
		p.handleRejectKey(k)
		return
	}
	switch {
	case k == term.RuneKey('j') || k == term.Named(term.SymDown):
		p.scrollBy(1)
	case k == term.RuneKey('k') || k == term.Named(term.SymUp):
		p.scrollBy(-1)
	case k == term.Ctrl('d') || k == term.Named(term.SymPageDown):
		p.scrollBy(10)
	case k == term.Ctrl('u') || k == term.Named(term.SymPageUp):
		p.scrollBy(-10)
	case k == term.RuneKey('a') && p.canCross():
		p.approve()
	case k == term.RuneKey('x') && p.canCross():
		p.mode = modeReject
		p.feedback = ""
	case k == term.RuneKey('q') || k == term.Named(term.SymEsc):
		p.closed = true
	}
}

func (p *Panel) canCross() bool { return p.crosser != nil && p.hasGate }

func (p *Panel) approve() {
	if err := p.crosser.Approve(p.id); err != nil {
		p.status = "approve failed: " + err.Error()
		return
	}
	p.status = "approved " + p.id
	p.closed = true // the review is now stale
}

func (p *Panel) handleRejectKey(k term.Key) {
	switch {
	case k == term.Named(term.SymEsc):
		p.mode = modeNormal
		p.feedback = ""
	case k == term.Named(term.SymEnter):
		if strings.TrimSpace(p.feedback) == "" {
			p.status = "reject needs feedback (Esc to cancel)"
			return
		}
		if err := p.crosser.Reject(p.id, p.feedback); err != nil {
			p.status = "reject failed: " + err.Error()
			p.mode = modeNormal
			return
		}
		p.status = "rejected " + p.id
		p.closed = true
	case k == term.Named(term.SymBackspace):
		if n := len(p.feedback); n > 0 {
			p.feedback = p.feedback[:n-1]
		}
	case k.Sym == term.SymRune && k.Mod == 0:
		p.feedback += string(k.Rune)
	}
}

func (p *Panel) scrollBy(delta int) {
	p.scroll += delta
	if p.scroll < 0 {
		p.scroll = 0
	}
	if p.scroll >= len(p.lines) {
		p.scroll = max(0, len(p.lines)-1)
	}
}

// View renders the review into rect. It satisfies layout.PaneContent.
func (p *Panel) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W < 2 || r.H < 2 {
		return
	}
	bottomY := r.Y + r.H - 1
	for row := 0; row < r.H-1; row++ {
		idx := p.scroll + row
		if idx >= len(p.lines) {
			break
		}
		ln := p.lines[idx]
		buf.SetString(r.X, r.Y+row, fit(ln.text, r.W), ln.style)
	}
	st := cellbuf.Style{Attrs: cellbuf.AttrReverse}
	buf.Fill(cellbuf.Rect{X: r.X, Y: bottomY, W: r.W, H: 1}, cellbuf.Cell{Rune: ' ', Width: 1, Style: st})
	buf.SetString(r.X, bottomY, fit(p.bottomText(), r.W), st)
}

func (p *Panel) bottomText() string {
	if p.mode == modeReject {
		return "reject feedback: " + p.feedback + "_"
	}
	if p.status != "" {
		return p.status
	}
	if p.canCross() {
		return "review " + p.id + " · j/k scroll · a approve · x reject · q close"
	}
	return "review " + p.id + " · j/k scroll · q close"
}

func diffStyle(k diffview.LineKind) cellbuf.Style {
	switch k {
	case diffview.LineAdd:
		return cellbuf.Style{FG: cellbuf.Palette(2)}
	case diffview.LineDel:
		return cellbuf.Style{FG: cellbuf.Palette(1)}
	case diffview.LineHunk:
		return cellbuf.Style{FG: cellbuf.Palette(6), Attrs: cellbuf.AttrBold}
	case diffview.LineFileHeader:
		return cellbuf.Style{Attrs: cellbuf.AttrBold}
	case diffview.LineMeta:
		return cellbuf.Style{FG: cellbuf.Palette(8)}
	default:
		return cellbuf.Style{}
	}
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
