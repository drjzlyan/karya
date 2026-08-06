package cellbuf

import (
	"sort"
	"unicode"
)

// RuneWidth returns the number of terminal cells a rune occupies: 0 for
// combining/zero-width and non-printing control runes, 2 for East Asian wide and
// fullwidth runes and common emoji, and 1 otherwise.
//
// It is a compact, stdlib-only approximation (no golang.org/x/text): it covers
// the wide ranges that matter in practice and treats everything else as narrow.
// Ranges can be extended as needed; correctness of the common cases (ASCII, CJK,
// emoji, combining marks) is what the tests pin.
func RuneWidth(r rune) int {
	if r == 0 {
		return 0
	}
	// C0/C1 control and DEL: non-printing.
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	// Combining marks and zero-width format characters.
	if unicode.In(r, unicode.Mn, unicode.Me) {
		return 0
	}
	if r == 0x200B || r == 0x200D || r == 0xFEFF { // ZWSP, ZWJ, BOM/ZWNBSP
		return 0
	}
	if inRanges(r, wideRanges) {
		return 2
	}
	return 1
}

// rng is an inclusive rune range.
type rng struct{ lo, hi rune }

// inRanges reports whether r falls within any range in the sorted table.
func inRanges(r rune, table []rng) bool {
	i := sort.Search(len(table), func(i int) bool { return table[i].hi >= r })
	return i < len(table) && r >= table[i].lo
}

// wideRanges lists East Asian Wide/Fullwidth blocks and common wide emoji,
// sorted by lo (and non-overlapping) so inRanges can binary-search.
var wideRanges = []rng{
	{0x1100, 0x115F},   // Hangul Jamo
	{0x2329, 0x232A},   // angle brackets
	{0x2E80, 0x303E},   // CJK radicals, Kangxi, CJK symbols
	{0x3041, 0x33FF},   // Hiragana .. CJK compatibility
	{0x3400, 0x4DBF},   // CJK Unified Ext A
	{0x4E00, 0x9FFF},   // CJK Unified Ideographs
	{0xA000, 0xA4CF},   // Yi
	{0xA960, 0xA97F},   // Hangul Jamo Ext-A
	{0xAC00, 0xD7A3},   // Hangul Syllables
	{0xF900, 0xFAFF},   // CJK Compatibility Ideographs
	{0xFE10, 0xFE19},   // Vertical forms
	{0xFE30, 0xFE6F},   // CJK Compatibility / small forms
	{0xFF00, 0xFF60},   // Fullwidth forms
	{0xFFE0, 0xFFE6},   // Fullwidth signs
	{0x1F000, 0x1F02F}, // Mahjong tiles
	{0x1F0A0, 0x1F0FF}, // Playing cards
	{0x1F100, 0x1F1FF}, // Enclosed alphanumeric supplement (incl. regional ind.)
	{0x1F200, 0x1F2FF}, // Enclosed ideographic supplement
	{0x1F300, 0x1F64F}, // Misc symbols, pictographs, emoticons
	{0x1F680, 0x1F6FF}, // Transport and map symbols
	{0x1F900, 0x1F9FF}, // Supplemental symbols and pictographs
	{0x1FA70, 0x1FAFF}, // Symbols and pictographs extended-A
	{0x20000, 0x3FFFD}, // CJK Unified Ext B+ (supplementary ideographic)
}
