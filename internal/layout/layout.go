// Package layout is karya's in-process window/pane/tab manager — the job tmux
// used to do, now owned by karya (DESIGN.md §6.1). A Tree holds tabs; each tab
// is a binary-ish tree of splits and leaf panes. layout computes each pane's
// rectangle, moves focus by spatial adjacency, resizes panes, and splits/closes
// them.
//
// Geometry and focus are pure functions of the tree and the screen rectangle, so
// they are unit- and snapshot-testable without a terminal. Pane contents are
// supplied via the PaneContent interface and rendered by the caller.
package layout

import (
	"math"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

// PaneID uniquely identifies a pane within a Tree.
type PaneID int

// PaneContent renders itself into buf within rect. focused indicates whether the
// pane currently has keyboard focus (for chrome such as an active border).
type PaneContent interface {
	View(buf *cellbuf.Buffer, rect cellbuf.Rect, focused bool)
}

// SplitDir is how a split arranges its children.
type SplitDir uint8

// Split directions.
const (
	SplitH SplitDir = iota // children arranged left-to-right (a vertical divider)
	SplitV                 // children stacked top-to-bottom (a horizontal divider)
)

// Dir is a movement/resize direction.
type Dir uint8

// Directions.
const (
	DirLeft Dir = iota
	DirRight
	DirUp
	DirDown
)

// resizeStep is the weight delta applied per resize action.
const resizeStep = 0.25

// minWeight keeps a pane from being resized out of existence.
const minWeight = 0.1

// node is either a leaf pane or a split with children.
type node struct {
	leaf     bool
	id       PaneID
	content  PaneContent
	dir      SplitDir
	children []*node
	weights  []float64
	parent   *node
}

func newLeaf(id PaneID, c PaneContent) *node {
	return &node{leaf: true, id: id, content: c}
}

type tab struct {
	root  *node
	focus PaneID
	title string
}

// Tree is the whole window layout: an ordered set of tabs with one active.
type Tree struct {
	tabs   []*tab
	active int
	nextID PaneID
}

// NewTree returns an empty Tree.
func NewTree() *Tree { return &Tree{} }

// AddTab appends a new tab whose sole pane holds content, makes it active, and
// returns the new pane's id.
func (t *Tree) AddTab(title string, content PaneContent) PaneID {
	id := t.newID()
	leaf := newLeaf(id, content)
	t.tabs = append(t.tabs, &tab{root: leaf, focus: id, title: title})
	t.active = len(t.tabs) - 1
	return id
}

func (t *Tree) newID() PaneID {
	t.nextID++
	return t.nextID
}

// TabCount returns the number of tabs.
func (t *Tree) TabCount() int { return len(t.tabs) }

// ActiveTab returns the index of the active tab.
func (t *Tree) ActiveTab() int { return t.active }

// TabTitles returns the tab titles in order.
func (t *Tree) TabTitles() []string {
	out := make([]string, len(t.tabs))
	for i, tb := range t.tabs {
		out[i] = tb.title
	}
	return out
}

func (t *Tree) cur() *tab {
	if len(t.tabs) == 0 {
		return nil
	}
	return t.tabs[t.active]
}

// FocusedID returns the focused pane id of the active tab (0 if none).
func (t *Tree) FocusedID() PaneID {
	c := t.cur()
	if c == nil {
		return 0
	}
	return c.focus
}

// SplitFocused splits the focused pane in the given direction, placing content
// in the new pane, focusing it, and returning its id. If the focused pane's
// parent is already a split of the same direction, the new pane is added as a
// sibling instead of nesting.
func (t *Tree) SplitFocused(dir SplitDir, content PaneContent) PaneID {
	c := t.cur()
	if c == nil {
		return t.AddTab("", content)
	}
	target := findLeaf(c.root, c.focus)
	if target == nil {
		return 0
	}
	id := t.newID()
	leaf := newLeaf(id, content)

	parent := target.parent
	if parent != nil && !parent.leaf && parent.dir == dir {
		// Add as a sibling right after the target.
		idx := indexOf(parent.children, target)
		w := parent.weights[idx]
		parent.children = insertNode(parent.children, idx+1, leaf)
		parent.weights = insertWeight(parent.weights, idx+1, w)
		leaf.parent = parent
	} else {
		// Replace target with a new split [target, leaf].
		split := &node{dir: dir, children: []*node{target, leaf}, weights: []float64{1, 1}, parent: parent}
		target.parent = split
		leaf.parent = split
		if parent == nil {
			c.root = split
		} else {
			idx := indexOf(parent.children, target)
			parent.children[idx] = split
		}
	}
	c.focus = id
	return id
}

// CloseFocused removes the focused pane. If it was the last pane in its tab, the
// tab is removed; the final tab is never removed (an empty Tree is avoided by
// the caller seeding at least one pane).
func (t *Tree) CloseFocused() {
	c := t.cur()
	if c == nil {
		return
	}
	target := findLeaf(c.root, c.focus)
	if target == nil {
		return
	}
	parent := target.parent
	if parent == nil {
		// Sole pane in the tab.
		if len(t.tabs) > 1 {
			t.tabs = append(t.tabs[:t.active], t.tabs[t.active+1:]...)
			if t.active >= len(t.tabs) {
				t.active = len(t.tabs) - 1
			}
		}
		return
	}
	idx := indexOf(parent.children, target)
	parent.children = append(parent.children[:idx], parent.children[idx+1:]...)
	parent.weights = append(parent.weights[:idx], parent.weights[idx+1:]...)
	// Collapse a now-single-child split into its child.
	if len(parent.children) == 1 {
		only := parent.children[0]
		gp := parent.parent
		only.parent = gp
		if gp == nil {
			c.root = only
		} else {
			gi := indexOf(gp.children, parent)
			gp.children[gi] = only
		}
	}
	// Focus the first remaining leaf.
	c.focus = firstLeaf(c.root).id
}

// NextTab / PrevTab / GotoTab switch the active tab (GotoTab is 1-based).
func (t *Tree) NextTab() {
	if len(t.tabs) > 0 {
		t.active = (t.active + 1) % len(t.tabs)
	}
}

func (t *Tree) PrevTab() {
	if len(t.tabs) > 0 {
		t.active = (t.active - 1 + len(t.tabs)) % len(t.tabs)
	}
}

// GotoTab activates the nth tab (1-based); out-of-range is ignored.
func (t *Tree) GotoTab(n int) {
	if n >= 1 && n <= len(t.tabs) {
		t.active = n - 1
	}
}

// Placement is a computed pane rectangle for the active tab.
type Placement struct {
	ID      PaneID
	Rect    cellbuf.Rect
	Content PaneContent
	Focused bool
}

// Compute returns the placements of every pane in the active tab within screen.
func (t *Tree) Compute(screen cellbuf.Rect) []Placement {
	c := t.cur()
	if c == nil {
		return nil
	}
	var out []Placement
	computeNode(c.root, screen, c.focus, &out)
	return out
}

func computeNode(n *node, rect cellbuf.Rect, focus PaneID, out *[]Placement) {
	if n == nil {
		return
	}
	if n.leaf {
		*out = append(*out, Placement{ID: n.id, Rect: rect, Content: n.content, Focused: n.id == focus})
		return
	}
	if n.dir == SplitH {
		sizes := distribute(rect.W, n.weights)
		x := rect.X
		for i, ch := range n.children {
			computeNode(ch, cellbuf.Rect{X: x, Y: rect.Y, W: sizes[i], H: rect.H}, focus, out)
			x += sizes[i]
		}
	} else {
		sizes := distribute(rect.H, n.weights)
		y := rect.Y
		for i, ch := range n.children {
			computeNode(ch, cellbuf.Rect{X: rect.X, Y: y, W: rect.W, H: sizes[i]}, focus, out)
			y += sizes[i]
		}
	}
}

// distribute splits total across weighted children, giving the remainder to the
// last child so the sizes sum exactly to total.
func distribute(total int, weights []float64) []int {
	n := len(weights)
	sizes := make([]int, n)
	if n == 0 {
		return sizes
	}
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	if sum <= 0 {
		sum = float64(n)
		for i := range weights {
			weights[i] = 1
		}
	}
	used := 0
	for i := 0; i < n-1; i++ {
		s := int(math.Round(float64(total) * weights[i] / sum))
		if s < 0 {
			s = 0
		}
		sizes[i] = s
		used += s
	}
	last := total - used
	if last < 0 {
		last = 0
	}
	sizes[n-1] = last
	return sizes
}

// Render computes placements and asks each pane's content to draw itself.
func (t *Tree) Render(buf *cellbuf.Buffer, screen cellbuf.Rect) {
	for _, p := range t.Compute(screen) {
		if p.Content != nil {
			p.Content.View(buf, p.Rect, p.Focused)
		}
	}
}

// --- tree helpers ---

func findLeaf(n *node, id PaneID) *node {
	if n == nil {
		return nil
	}
	if n.leaf {
		if n.id == id {
			return n
		}
		return nil
	}
	for _, ch := range n.children {
		if got := findLeaf(ch, id); got != nil {
			return got
		}
	}
	return nil
}

func firstLeaf(n *node) *node {
	for !n.leaf {
		n = n.children[0]
	}
	return n
}

func indexOf(nodes []*node, target *node) int {
	for i, n := range nodes {
		if n == target {
			return i
		}
	}
	return -1
}

func insertNode(s []*node, i int, n *node) []*node {
	s = append(s, nil)
	copy(s[i+1:], s[i:])
	s[i] = n
	return s
}

func insertWeight(s []float64, i int, w float64) []float64 {
	s = append(s, 0)
	copy(s[i+1:], s[i:])
	s[i] = w
	return s
}
