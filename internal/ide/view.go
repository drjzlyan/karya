package ide

import (
	"fmt"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

// drawStatus paints the bottom status line: the leader hint, tab position, and
// the current status message.
func (m *Model) drawStatus(buf *cellbuf.Buffer) {
	y := m.rows - 1
	if y < 0 {
		return
	}
	st := cellbuf.Style{Attrs: cellbuf.AttrReverse}
	buf.Fill(cellbuf.Rect{X: 0, Y: y, W: m.cols, H: 1}, cellbuf.Cell{Rune: ' ', Width: 1, Style: st})
	tabs := fmt.Sprintf(" tab %d/%d ", m.tree.ActiveTab()+1, m.tree.TabCount())
	buf.SetString(0, y, " karya "+tabs+"│ "+m.status, st)
}

// drawWhichKey overlays the pending-chord continuations near the bottom-left,
// mirroring which-key discovery.
func (m *Model) drawWhichKey(buf *cellbuf.Buffer) {
	lines := make([]string, 0, len(m.whichkey))
	width := 0
	for _, c := range m.whichkey {
		marker := "  "
		if c.IsGroup {
			marker = " ▸"
		}
		desc := c.Desc
		if desc == "" {
			desc = c.Group
		}
		ln := fmt.Sprintf(" %-7s%s%s", c.Key.String(), marker, desc)
		lines = append(lines, ln)
		if len(ln) > width {
			width = len(ln)
		}
	}
	if width < 10 {
		width = 10
	}
	boxW := width + 2
	boxH := len(lines) + 2
	if boxW > m.cols {
		boxW = m.cols
	}
	// Place above the status line.
	x := 0
	y := m.rows - 1 - boxH
	if y < 0 {
		y = 0
	}
	rect := cellbuf.Rect{X: x, Y: y, W: boxW, H: boxH}
	// Clear the box area, draw a frame, then the lines.
	buf.Fill(rect, cellbuf.Cell{Rune: ' ', Width: 1})
	inner := drawFrame(buf, rect, "leader", true)
	for i, ln := range lines {
		if i >= inner.H {
			break
		}
		buf.SetString(inner.X, inner.Y+i, ln, cellbuf.Style{})
	}
}
