package gitui

import (
	"fmt"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/diffview"
	"github.com/drjzlyan/karya/internal/git"
)

// View renders the panel into rect: a header, a left file list, a right diff of
// the selected file, and a bottom status/commit line. It satisfies
// layout.PaneContent.
func (p *Panel) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	if r.W < 4 || r.H < 3 {
		return
	}
	// Header.
	header := "  branch: " + p.branch
	buf.SetString(r.X, r.Y, fit(header, r.W), cellbuf.Style{Attrs: cellbuf.AttrBold})

	// Bottom line: commit input or status.
	bottomY := r.Y + r.H - 1
	p.drawBottom(buf, r, bottomY)

	bodyY := r.Y + 1
	bodyH := bottomY - bodyY
	if bodyH < 1 {
		return
	}
	listW := min(40, r.W/2)

	// Left column: Changes on top, Log below — so the panel is informative even
	// when the tree is clean. Give Changes only what it needs (capped at half),
	// and let the Log fill the rest.
	changesH := len(p.files) + 1 // +1 header
	if changesH < 2 {
		changesH = 2 // header + "(clean)"
	}
	if max := bodyH / 2; changesH > max && max >= 2 {
		changesH = max
	}
	if changesH > bodyH {
		changesH = bodyH
	}
	p.drawChanges(buf, cellbuf.Rect{X: r.X, Y: bodyY, W: listW, H: changesH})
	logY := bodyY + changesH
	if logH := bottomY - logY; logH > 0 {
		p.drawLog(buf, cellbuf.Rect{X: r.X, Y: logY, W: listW, H: logH})
	}

	diffX := r.X + listW + 1
	diffW := r.X + r.W - diffX
	if diffW > 0 {
		diffview.Render(buf, cellbuf.Rect{X: diffX, Y: bodyY, W: diffW, H: bodyH}, p.diff, p.diffScroll)
	}
}

func (p *Panel) drawChanges(buf *cellbuf.Buffer, r cellbuf.Rect) {
	hdr := header("Changes", len(p.files), p.focus == focusChanges)
	buf.SetString(r.X, r.Y, fit(hdr, r.W), cellbuf.Style{Attrs: cellbuf.AttrBold})
	rows := cellbuf.Rect{X: r.X, Y: r.Y + 1, W: r.W, H: r.H - 1}
	if rows.H < 1 {
		return
	}
	if len(p.files) == 0 {
		buf.SetString(rows.X, rows.Y, fit("(clean)", rows.W), cellbuf.Style{FG: cellbuf.Palette(8)})
		return
	}
	for i, f := range p.files {
		if i >= rows.H {
			break
		}
		mark, markStyle := fileMark(f)
		st := cellbuf.Style{}
		if i == p.sel && p.focus == focusChanges {
			st.Attrs |= cellbuf.AttrReverse
		}
		buf.Set(rows.X, rows.Y+i, cellbuf.Cell{Rune: mark, Width: 1, Style: markStyle})
		buf.SetString(rows.X+2, rows.Y+i, fit(f.Path, rows.W-2), st)
	}
}

func (p *Panel) drawLog(buf *cellbuf.Buffer, r cellbuf.Rect) {
	hdr := header("Log", len(p.commits), p.focus == focusLog)
	buf.SetString(r.X, r.Y, fit(hdr, r.W), cellbuf.Style{Attrs: cellbuf.AttrBold})
	rows := cellbuf.Rect{X: r.X, Y: r.Y + 1, W: r.W, H: r.H - 1}
	if rows.H < 1 {
		return
	}
	if len(p.commits) == 0 {
		buf.SetString(rows.X, rows.Y, fit("(no commits)", rows.W), cellbuf.Style{FG: cellbuf.Palette(8)})
		return
	}
	// Keep the selected commit in view (simple window that follows the cursor).
	start := 0
	if p.logSel >= rows.H {
		start = p.logSel - rows.H + 1
	}
	for i := start; i < len(p.commits) && i-start < rows.H; i++ {
		c := p.commits[i]
		y := rows.Y + (i - start)
		st := cellbuf.Style{}
		if i == p.logSel && p.focus == focusLog {
			st.Attrs |= cellbuf.AttrReverse
		}
		buf.SetString(rows.X, y, c.Hash+" ", cellbuf.Style{FG: cellbuf.Palette(3)}) // yellow hash
		buf.SetString(rows.X+len(c.Hash)+1, y, fit(c.Subject, rows.W-len(c.Hash)-1), st)
	}
}

// header formats a section title with its count and a focus marker.
func header(name string, n int, focused bool) string {
	marker := "  "
	if focused {
		marker = "▸ "
	}
	return fmt.Sprintf("%s%s (%d)", marker, name, n)
}

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
	if p.mode == modeCommit {
		buf.SetString(r.X, y, fit("commit: "+p.commitBuf+"_", r.W), st)
		return
	}
	text := p.status
	if text == "" {
		text = "Tab changes/log · j/k move · space stage · a/u all · c commit · P push · q close"
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
