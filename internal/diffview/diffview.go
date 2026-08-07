// Package diffview parses a unified diff and renders it into a cellbuf with
// syntax colors (additions, deletions, hunk headers). It is pure — text in,
// cells out — and shared by the git panel and task review (DESIGN.md §6, §7).
package diffview

import (
	"strings"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

// LineKind classifies a diff line for styling.
type LineKind uint8

// Diff line kinds.
const (
	LineContext LineKind = iota
	LineAdd
	LineDel
	LineHunk
	LineFileHeader
	LineMeta
)

// Line is one classified line of a unified diff.
type Line struct {
	Kind LineKind
	Text string
}

// Parse classifies each line of a unified diff. An empty diff yields no lines.
func Parse(diff string) []Line {
	if diff == "" {
		return nil
	}
	raw := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	lines := make([]Line, 0, len(raw))
	for _, l := range raw {
		lines = append(lines, Line{Kind: classify(l), Text: l})
	}
	return lines
}

func classify(l string) LineKind {
	switch {
	case strings.HasPrefix(l, "diff --git"),
		strings.HasPrefix(l, "--- "),
		strings.HasPrefix(l, "+++ "):
		return LineFileHeader
	case strings.HasPrefix(l, "@@"):
		return LineHunk
	case strings.HasPrefix(l, "index "),
		strings.HasPrefix(l, "new file"),
		strings.HasPrefix(l, "deleted file"),
		strings.HasPrefix(l, "old mode"),
		strings.HasPrefix(l, "new mode"),
		strings.HasPrefix(l, "similarity"),
		strings.HasPrefix(l, "rename "),
		strings.HasPrefix(l, "\\ No newline"):
		return LineMeta
	case strings.HasPrefix(l, "+"):
		return LineAdd
	case strings.HasPrefix(l, "-"):
		return LineDel
	default:
		return LineContext
	}
}

// styleFor returns the cell style for a diff line kind.
func styleFor(k LineKind) cellbuf.Style {
	switch k {
	case LineAdd:
		return cellbuf.Style{FG: cellbuf.Palette(2)} // green
	case LineDel:
		return cellbuf.Style{FG: cellbuf.Palette(1)} // red
	case LineHunk:
		return cellbuf.Style{FG: cellbuf.Palette(6), Attrs: cellbuf.AttrBold} // cyan
	case LineFileHeader:
		return cellbuf.Style{Attrs: cellbuf.AttrBold}
	case LineMeta:
		return cellbuf.Style{FG: cellbuf.Palette(8)} // dim
	default:
		return cellbuf.Style{}
	}
}

// Render draws lines into rect starting at the given scroll offset (the index of
// the first line to show), truncating each to the rect width.
func Render(buf *cellbuf.Buffer, rect cellbuf.Rect, lines []Line, scroll int) {
	if scroll < 0 {
		scroll = 0
	}
	for row := 0; row < rect.H; row++ {
		idx := scroll + row
		if idx >= len(lines) {
			break
		}
		ln := lines[idx]
		text := ln.Text
		if len(text) > rect.W {
			text = text[:rect.W]
		}
		buf.SetString(rect.X, rect.Y+row, text, styleFor(ln.Kind))
	}
}

// MaxScroll returns the largest useful scroll offset for lines in a rect of the
// given height (so the last line stays visible).
func MaxScroll(lines []Line, height int) int {
	m := len(lines) - height
	if m < 0 {
		return 0
	}
	return m
}
