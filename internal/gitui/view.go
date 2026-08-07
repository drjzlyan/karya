package gitui

import (
	"fmt"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/diffview"
	"github.com/drjzlyan/karya/internal/git"
)

// minBox is the smallest useful box height (top border + 1 content row + bottom).
const minBox = 3

// View renders the panel as a set of bordered panes — Changes, Branches,
// Stashes, and Log stacked in a left column, with the selected item's diff in a
// large pane on the right — plus a bottom status/input line. Discrete panes
// (rather than one continuous list) make it obvious where each section starts
// and which one has focus. It satisfies layout.PaneContent.
func (p *Panel) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W < 8 || r.H < 4 {
		return
	}
	// Top header: the current branch. Bottom line: status or an input prompt.
	buf.SetString(r.X, r.Y, fit("  on "+p.branch, r.W), cellbuf.Style{Attrs: cellbuf.AttrBold})
	bottomY := r.Y + r.H - 1
	p.drawBottom(buf, r, bottomY)

	bodyY := r.Y + 1
	bodyH := bottomY - bodyY
	if bodyH < minBox {
		return
	}
	leftW := clamp(r.W*2/5, 22, 44)

	// Left column: four stacked panes. Give each its natural height, capped so the
	// Log (last) still gets room; the Log absorbs any remainder.
	natural := []int{
		len(p.files) + 2,
		min(len(p.branches), 4) + 2,
		min(len(p.stashes), 3) + 2,
		bodyH, // the log wants whatever is left
	}
	h := splitColumn(bodyH, natural)
	y := bodyY
	p.drawChanges(buf, cellbuf.Rect{X: r.X, Y: y, W: leftW, H: h[0]})
	y += h[0]
	p.drawBranches(buf, cellbuf.Rect{X: r.X, Y: y, W: leftW, H: h[1]})
	y += h[1]
	p.drawStashes(buf, cellbuf.Rect{X: r.X, Y: y, W: leftW, H: h[2]})
	y += h[2]
	p.drawLog(buf, cellbuf.Rect{X: r.X, Y: y, W: leftW, H: h[3]})

	// Right column: the diff of whatever is selected.
	diffRect := cellbuf.Rect{X: r.X + leftW, Y: bodyY, W: r.W - leftW, H: bodyH}
	inner := cellbuf.Box(buf, diffRect, p.diffTitle(), p.focus == focusLog || p.focus == focusStashes)
	diffview.Render(buf, inner, p.diff, p.diffScroll)
}

// splitColumn distributes bodyH across len(natural) stacked boxes: each takes its
// natural height but never so much that a later box drops below minBox, and the
// last box absorbs any remainder. Boxes that cannot fit get 0 (skipped).
func splitColumn(bodyH int, natural []int) []int {
	n := len(natural)
	out := make([]int, n)
	remaining := bodyH
	for i := 0; i < n; i++ {
		if remaining < minBox {
			out[i] = 0
			continue
		}
		reserve := minBox * (n - i - 1) // keep room for the boxes below
		avail := remaining - reserve
		want := natural[i]
		if want > avail {
			want = avail
		}
		if want < minBox {
			want = minBox
		}
		if want > remaining {
			want = remaining
		}
		out[i] = want
		remaining -= want
	}
	if remaining > 0 { // hand leftover to the last (log) box
		out[n-1] += remaining
	}
	return out
}

func (p *Panel) drawChanges(buf *cellbuf.Buffer, r cellbuf.Rect) {
	inner, ok := boxInner(buf, r, title("Changes", len(p.files)), p.focus == focusChanges)
	if !ok {
		return
	}
	if len(p.files) == 0 {
		buf.SetString(inner.X, inner.Y, fit("clean", inner.W), dimStyle())
		return
	}
	start := windowStart(p.sel, inner.H, len(p.files))
	for i := start; i < len(p.files) && i-start < inner.H; i++ {
		f := p.files[i]
		y := inner.Y + (i - start)
		mark, markStyle := fileMark(f)
		st := rowStyle(i == p.sel && p.focus == focusChanges)
		buf.Set(inner.X, y, cellbuf.Cell{Rune: mark, Width: 1, Style: markStyle})
		buf.SetString(inner.X+2, y, fit(f.Path, inner.W-2), st)
	}
}

func (p *Panel) drawBranches(buf *cellbuf.Buffer, r cellbuf.Rect) {
	inner, ok := boxInner(buf, r, title("Branches", len(p.branches)), p.focus == focusBranches)
	if !ok {
		return
	}
	if len(p.branches) == 0 {
		buf.SetString(inner.X, inner.Y, fit("(none)", inner.W), dimStyle())
		return
	}
	start := windowStart(p.branchSel, inner.H, len(p.branches))
	for i := start; i < len(p.branches) && i-start < inner.H; i++ {
		name := p.branches[i]
		y := inner.Y + (i - start)
		st := rowStyle(i == p.branchSel && p.focus == focusBranches)
		marker := "  "
		if name == p.branch {
			marker = "* "
			st.FG = cellbuf.Palette(2) // green for the current branch
		}
		buf.SetString(inner.X, y, marker, st)
		buf.SetString(inner.X+2, y, fit(name, inner.W-2), st)
	}
}

func (p *Panel) drawStashes(buf *cellbuf.Buffer, r cellbuf.Rect) {
	inner, ok := boxInner(buf, r, title("Stashes", len(p.stashes)), p.focus == focusStashes)
	if !ok {
		return
	}
	if len(p.stashes) == 0 {
		buf.SetString(inner.X, inner.Y, fit("(none)", inner.W), dimStyle())
		return
	}
	start := windowStart(p.stashSel, inner.H, len(p.stashes))
	for i := start; i < len(p.stashes) && i-start < inner.H; i++ {
		s := p.stashes[i]
		y := inner.Y + (i - start)
		st := rowStyle(i == p.stashSel && p.focus == focusStashes)
		buf.SetString(inner.X, y, fit(s.Desc, inner.W), st)
	}
}

func (p *Panel) drawLog(buf *cellbuf.Buffer, r cellbuf.Rect) {
	inner, ok := boxInner(buf, r, title("Log", len(p.commits)), p.focus == focusLog)
	if !ok {
		return
	}
	if len(p.commits) == 0 {
		buf.SetString(inner.X, inner.Y, fit("(no commits)", inner.W), dimStyle())
		return
	}
	start := windowStart(p.logSel, inner.H, len(p.commits))
	for i := start; i < len(p.commits) && i-start < inner.H; i++ {
		c := p.commits[i]
		y := inner.Y + (i - start)
		st := rowStyle(i == p.logSel && p.focus == focusLog)
		buf.SetString(inner.X, y, c.Hash+" ", cellbuf.Style{FG: cellbuf.Palette(3)}) // yellow hash
		buf.SetString(inner.X+len(c.Hash)+1, y, fit(c.Subject, inner.W-len(c.Hash)-1), st)
	}
}

// diffTitle labels the diff pane with what it is showing.
func (p *Panel) diffTitle() string {
	switch p.focus {
	case focusLog:
		if len(p.commits) > 0 {
			return "Diff · " + p.commits[p.logSel].Hash
		}
	case focusStashes:
		if len(p.stashes) > 0 {
			return "Diff · " + p.stashes[p.stashSel].Ref
		}
	case focusBranches:
		return "Branches"
	default:
		if len(p.files) > 0 {
			return "Diff · " + p.files[p.sel].Path
		}
	}
	return "Diff"
}

// boxInner draws a section's border and returns its content rect. ok is false
// when the box is too small to render into (so the caller skips its content).
func boxInner(buf *cellbuf.Buffer, r cellbuf.Rect, name string, focused bool) (cellbuf.Rect, bool) {
	if r.H < minBox || r.W < 4 {
		return cellbuf.Rect{}, false
	}
	inner := cellbuf.Box(buf, r, name, focused)
	return inner, inner.H >= 1 && inner.W >= 1
}

// windowStart returns the first index to render so that sel stays visible in a
// viewport of height h over n items.
func windowStart(sel, h, n int) int {
	if h <= 0 || sel < h {
		return 0
	}
	start := sel - h + 1
	if start > n-h {
		start = n - h
	}
	if start < 0 {
		start = 0
	}
	return start
}

// title formats a pane title with its item count.
func title(name string, n int) string { return fmt.Sprintf("%s (%d)", name, n) }

// rowStyle returns the style for a list row, reversed when it is the selected
// row of the focused pane.
func rowStyle(selected bool) cellbuf.Style {
	st := cellbuf.Style{}
	if selected {
		st.Attrs |= cellbuf.AttrReverse
	}
	return st
}

func dimStyle() cellbuf.Style { return cellbuf.Style{FG: cellbuf.Palette(8)} }

// fileMark returns a status glyph and its color for a file.
func fileMark(f git.FileStatus) (rune, cellbuf.Style) {
	switch {
	case f.Untracked:
		return '?', cellbuf.Style{FG: cellbuf.Palette(8)}
	case f.Staged() && f.Unstaged():
		return '±', cellbuf.Style{FG: cellbuf.Palette(3)} // partially staged
	case f.Staged():
		return '●', cellbuf.Style{FG: cellbuf.Palette(2)} // staged (green)
	default:
		return '○', cellbuf.Style{FG: cellbuf.Palette(1)} // unstaged (red)
	}
}

func (p *Panel) drawBottom(buf *cellbuf.Buffer, r cellbuf.Rect, y int) {
	st := cellbuf.Style{Attrs: cellbuf.AttrReverse}
	buf.Fill(cellbuf.Rect{X: r.X, Y: y, W: r.W, H: 1}, cellbuf.Cell{Rune: ' ', Width: 1, Style: st})
	switch p.mode {
	case modeCommit:
		buf.SetString(r.X, y, fit("commit: "+p.commitBuf+"_", r.W), st)
		return
	case modeBranch:
		buf.SetString(r.X, y, fit("new branch: "+p.branchBuf+"_", r.W), st)
		return
	}
	text := p.status
	if text == "" {
		text = "Tab pane · j/k move · Enter act · space stage · c commit · P push · s stash · b branch · q close"
	}
	buf.SetString(r.X, y, fit(text, r.W), st)
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
