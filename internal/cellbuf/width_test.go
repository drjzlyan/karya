package cellbuf

import "testing"

func TestRuneWidth(t *testing.T) {
	cases := []struct {
		name string
		r    rune
		want int
	}{
		{"ascii", 'a', 1},
		{"space", ' ', 1},
		{"digit", '9', 1},
		{"latin1", 'ñ', 1},
		{"null", 0, 0},
		{"control-tab", '\t', 0},
		{"control-newline", '\n', 0},
		{"del", 0x7f, 0},
		{"c1", 0x85, 0},
		{"combining-acute", 0x0301, 0},
		{"zwj", 0x200D, 0},
		{"zwsp", 0x200B, 0},
		{"bom", 0xFEFF, 0},
		{"cjk-han", '世', 2},
		{"hiragana", 'あ', 2},
		{"hangul", '한', 2},
		{"fullwidth-A", 'Ａ', 2},
		{"emoji-smile", 0x1F600, 2},
		{"emoji-rocket", 0x1F680, 2},
		{"cjk-ext-b", 0x20000, 2},
		{"greek", 'λ', 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RuneWidth(c.r); got != c.want {
				t.Fatalf("RuneWidth(%U) = %d want %d", c.r, got, c.want)
			}
		})
	}
}

func TestWideRangesSortedNonOverlapping(t *testing.T) {
	for i := 1; i < len(wideRanges); i++ {
		if wideRanges[i].lo <= wideRanges[i-1].hi {
			t.Fatalf("wideRanges not sorted/non-overlapping at %d: %+v after %+v",
				i, wideRanges[i], wideRanges[i-1])
		}
		if wideRanges[i].lo > wideRanges[i].hi {
			t.Fatalf("range %d has lo>hi: %+v", i, wideRanges[i])
		}
	}
}
