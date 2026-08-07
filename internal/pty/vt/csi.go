package vt

import (
	"strconv"
	"strings"

	"github.com/drjzlyan/karya/internal/cellbuf"
)

// csi dispatches a CSI sequence with the given final byte.
func (s *Screen) csi(final byte) {
	if s.private {
		// DEC private modes (cursor keys, alt screen, etc.) are not modeled yet.
		return
	}
	if final != 'm' {
		// Any cursor-affecting sequence cancels a deferred wrap.
		s.wrapNext = false
	}
	nums := parseInts(string(s.params))
	arg := func(i, def int) int {
		if i < len(nums) && nums[i] > 0 {
			return nums[i]
		}
		if i < len(nums) && nums[i] == 0 {
			return 0
		}
		return def
	}

	switch final {
	case 'A':
		s.cy = clamp(s.cy-arg(0, 1), 0, s.h-1)
	case 'B':
		s.cy = clamp(s.cy+arg(0, 1), 0, s.h-1)
	case 'C':
		s.cx = clamp(s.cx+arg(0, 1), 0, s.w-1)
	case 'D':
		s.cx = clamp(s.cx-arg(0, 1), 0, s.w-1)
	case 'G':
		s.cx = clamp(arg(0, 1)-1, 0, s.w-1)
	case 'd':
		s.cy = clamp(arg(0, 1)-1, 0, s.h-1)
	case 'H', 'f':
		row, col := 1, 1
		if len(nums) >= 1 && nums[0] > 0 {
			row = nums[0]
		}
		if len(nums) >= 2 && nums[1] > 0 {
			col = nums[1]
		}
		s.cy = clamp(row-1, 0, s.h-1)
		s.cx = clamp(col-1, 0, s.w-1)
	case 'J':
		s.eraseDisplay(arg(0, 0))
	case 'K':
		s.eraseLine(arg(0, 0))
	case 'm':
		s.sgr(nums)
	}
}

// eraseDisplay clears part or all of the screen (ED).
func (s *Screen) eraseDisplay(mode int) {
	switch mode {
	case 0: // cursor to end of screen
		s.eraseLine(0)
		for y := s.cy + 1; y < s.h; y++ {
			s.blankRow(y)
		}
	case 1: // start of screen to cursor
		for y := 0; y < s.cy; y++ {
			s.blankRow(y)
		}
		s.eraseLine(1)
	case 2, 3: // whole screen
		for y := 0; y < s.h; y++ {
			s.blankRow(y)
		}
	}
}

// eraseLine clears part or all of the cursor's line (EL).
func (s *Screen) eraseLine(mode int) {
	blank := cellbuf.Cell{Rune: ' ', Width: 1, Style: s.style}
	switch mode {
	case 0:
		for x := s.cx; x < s.w; x++ {
			s.buf.Set(x, s.cy, blank)
		}
	case 1:
		for x := 0; x <= s.cx && x < s.w; x++ {
			s.buf.Set(x, s.cy, blank)
		}
	case 2:
		s.blankRow(s.cy)
	}
}

func (s *Screen) blankRow(y int) {
	blank := cellbuf.Cell{Rune: ' ', Width: 1, Style: s.style}
	for x := 0; x < s.w; x++ {
		s.buf.Set(x, y, blank)
	}
}

// sgr applies Select Graphic Rendition parameters to the current style.
func (s *Screen) sgr(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch {
		case p == 0:
			s.style = cellbuf.Style{}
		case p == 1:
			s.style.Attrs |= cellbuf.AttrBold
		case p == 2:
			s.style.Attrs |= cellbuf.AttrDim
		case p == 3:
			s.style.Attrs |= cellbuf.AttrItalic
		case p == 4:
			s.style.Attrs |= cellbuf.AttrUnderline
		case p == 7:
			s.style.Attrs |= cellbuf.AttrReverse
		case p == 9:
			s.style.Attrs |= cellbuf.AttrStrike
		case p == 22:
			s.style.Attrs &^= cellbuf.AttrBold | cellbuf.AttrDim
		case p == 23:
			s.style.Attrs &^= cellbuf.AttrItalic
		case p == 24:
			s.style.Attrs &^= cellbuf.AttrUnderline
		case p == 27:
			s.style.Attrs &^= cellbuf.AttrReverse
		case p == 29:
			s.style.Attrs &^= cellbuf.AttrStrike
		case p >= 30 && p <= 37:
			s.style.FG = cellbuf.Palette(uint8(p - 30))
		case p == 39:
			s.style.FG = cellbuf.Color{}
		case p >= 40 && p <= 47:
			s.style.BG = cellbuf.Palette(uint8(p - 40))
		case p == 49:
			s.style.BG = cellbuf.Color{}
		case p >= 90 && p <= 97:
			s.style.FG = cellbuf.Palette(uint8(p - 90 + 8))
		case p >= 100 && p <= 107:
			s.style.BG = cellbuf.Palette(uint8(p - 100 + 8))
		case p == 38:
			if c, n, ok := extColor(params[i:]); ok {
				s.style.FG = c
				i += n
			}
		case p == 48:
			if c, n, ok := extColor(params[i:]); ok {
				s.style.BG = c
				i += n
			}
		}
	}
}

// extColor parses a 38/48 extended color starting at params[0]. It returns the
// color, the number of extra params consumed, and ok.
func extColor(params []int) (cellbuf.Color, int, bool) {
	if len(params) < 2 {
		return cellbuf.Color{}, 0, false
	}
	switch params[1] {
	case 5: // 38;5;n
		if len(params) < 3 {
			return cellbuf.Color{}, 0, false
		}
		return cellbuf.Palette(uint8(params[2])), 2, true
	case 2: // 38;2;r;g;b
		if len(params) < 5 {
			return cellbuf.Color{}, 0, false
		}
		return cellbuf.RGB(uint8(params[2]), uint8(params[3]), uint8(params[4])), 4, true
	}
	return cellbuf.Color{}, 0, false
}

func parseInts(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]int, len(parts))
	for i, p := range parts {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
