package layout

import "github.com/drjzlyan/karya/internal/cellbuf"

// FocusDir moves focus to the nearest pane in direction d within the active tab,
// laid out in screen. It returns true if focus moved. Selection prefers panes
// whose perpendicular span overlaps the current pane, then the closest along the
// direction of travel.
func (t *Tree) FocusDir(d Dir, screen cellbuf.Rect) bool {
	c := t.cur()
	if c == nil {
		return false
	}
	places := t.Compute(screen)
	var cur *Placement
	for i := range places {
		if places[i].ID == c.focus {
			cur = &places[i]
		}
	}
	if cur == nil {
		return false
	}
	best := -1
	var bestPrimary, bestSecondary int
	for i := range places {
		p := &places[i]
		if p.ID == cur.ID {
			continue
		}
		ok, primary, secondary := directionScore(cur.Rect, p.Rect, d)
		if !ok {
			continue
		}
		if best == -1 || primary < bestPrimary ||
			(primary == bestPrimary && secondary < bestSecondary) {
			best, bestPrimary, bestSecondary = i, primary, secondary
		}
	}
	if best == -1 {
		return false
	}
	c.focus = places[best].ID
	return true
}

// directionScore reports whether cand lies in direction d from cur and, if so, a
// primary distance (along d) and secondary distance (perpendicular, penalized
// when the spans do not overlap) for choosing the nearest neighbor.
func directionScore(cur, cand cellbuf.Rect, d Dir) (ok bool, primary, secondary int) {
	curR, curB := cur.X+cur.W, cur.Y+cur.H
	candR, candB := cand.X+cand.W, cand.Y+cand.H
	const noOverlapPenalty = 1 << 20

	switch d {
	case DirRight:
		if cand.X < curR {
			return false, 0, 0
		}
		primary = cand.X - curR
		secondary = absInt(center(cur.Y, cur.H) - center(cand.Y, cand.H))
		if overlapLen(cur.Y, curB, cand.Y, candB) <= 0 {
			secondary += noOverlapPenalty
		}
	case DirLeft:
		if candR > cur.X {
			return false, 0, 0
		}
		primary = cur.X - candR
		secondary = absInt(center(cur.Y, cur.H) - center(cand.Y, cand.H))
		if overlapLen(cur.Y, curB, cand.Y, candB) <= 0 {
			secondary += noOverlapPenalty
		}
	case DirDown:
		if cand.Y < curB {
			return false, 0, 0
		}
		primary = cand.Y - curB
		secondary = absInt(center(cur.X, cur.W) - center(cand.X, cand.W))
		if overlapLen(cur.X, curR, cand.X, candR) <= 0 {
			secondary += noOverlapPenalty
		}
	case DirUp:
		if candB > cur.Y {
			return false, 0, 0
		}
		primary = cur.Y - candB
		secondary = absInt(center(cur.X, cur.W) - center(cand.X, cand.W))
		if overlapLen(cur.X, curR, cand.X, candR) <= 0 {
			secondary += noOverlapPenalty
		}
	}
	return true, primary, secondary
}

// ResizeFocused grows (DirRight/DirDown) or shrinks (DirLeft/DirUp) the focused
// pane along the matching axis by adjusting weights against an adjacent sibling,
// keeping the parent's total constant. It is a no-op if no matching split
// ancestor exists or the change would drop a pane below minWeight.
func (t *Tree) ResizeFocused(d Dir) {
	c := t.cur()
	if c == nil {
		return
	}
	target := findLeaf(c.root, c.focus)
	if target == nil {
		return
	}
	axis := SplitH
	if d == DirUp || d == DirDown {
		axis = SplitV
	}
	grow := d == DirRight || d == DirDown

	n := target
	for n.parent != nil {
		p := n.parent
		if p.dir == axis && len(p.children) >= 2 {
			idx := indexOf(p.children, n)
			neighbor := idx + 1
			if neighbor >= len(p.children) {
				neighbor = idx - 1
			}
			delta := resizeStep
			if !grow {
				delta = -resizeStep
			}
			if p.weights[idx]+delta < minWeight || p.weights[neighbor]-delta < minWeight {
				return
			}
			p.weights[idx] += delta
			p.weights[neighbor] -= delta
			return
		}
		n = p
	}
}

func center(pos, size int) int { return pos*2 + size } // 2× center to avoid /2 rounding

func overlapLen(a0, a1, b0, b1 int) int {
	lo := max(a0, b0)
	hi := min(a1, b1)
	return hi - lo
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
