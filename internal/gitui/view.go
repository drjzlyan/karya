package gitui

import (
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
	p.drawFiles(buf, cellbuf.Rect{X: r.X, Y: bodyY, W: listW, H: bodyH})
	diffX := r.X + listW + 1
	diffW := r.X + r.W - diffX
	if diffW > 0 {
		diffview.Render(buf, cellbuf.Rect{X: diffX, Y: bodyY, W: diffW, H: bodyH}, p.diff, p.diffScroll)
	}
}

func (p *Panel) drawFiles(buf *cellbuf.Buffer, r cellbuf.Rect) {
	if len(p.files) == 0 {
		buf.SetString(r.X, r.Y, fit("(clean)", r.W), cellbuf.Style{FG: cellbuf.Palette(8)})
		return
	}
	for i, f := range p.files {
		if i >= r.H {
			break
		}
		mark, markStyle := fileMark(f)
		st := cellbuf.Style{}
		if i == p.sel {
			st.Attrs |= cellbuf.AttrReverse
		}
		buf.Set(r.X, r.Y+i, cellbuf.Cell{Rune: mark, Width: 1, Style: markStyle})
		buf.SetString(r.X+2, r.Y+i, fit(f.Path, r.W-2), st)
	}
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
		text = "j/k move · space stage · a/u all · c commit · P push · q close"
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
