package term

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Decoder turns a raw terminal byte stream into Events. It buffers partial
// escape sequences across Feed calls, so a sequence split over two reads decodes
// correctly. A lone ESC is ambiguous (it may begin a sequence), so it is held
// until either more bytes arrive or Flush is called after the read timeout.
//
// Decoder is pure: Feed and Flush are functions of the bytes seen so far, with
// no terminal or clock involved, which is what makes input handling testable.
type Decoder struct {
	buf []byte
}

// NewDecoder returns an empty Decoder.
func NewDecoder() *Decoder { return &Decoder{} }

// Feed appends p to the internal buffer and returns every complete event that
// can now be decoded. Incomplete trailing sequences stay buffered.
func (d *Decoder) Feed(p []byte) []Event {
	d.buf = append(d.buf, p...)
	return d.drain(false)
}

// Flush interprets any buffered bytes as complete input (e.g. a lone ESC becomes
// the Esc key). Call it after an input read times out with bytes still pending.
func (d *Decoder) Flush() []Event { return d.drain(true) }

// Buffered reports how many bytes are held awaiting more input. When it is
// non-zero after Feed, the caller should arm a short timer and call Flush if no
// further bytes arrive (this disambiguates a lone ESC from an escape sequence).
func (d *Decoder) Buffered() int { return len(d.buf) }

func (d *Decoder) drain(final bool) []Event {
	var events []Event
	for len(d.buf) > 0 {
		ev, n, ok := decodeOne(d.buf, final)
		if !ok {
			break // incomplete; wait for more bytes
		}
		if n == 0 {
			break // safety against non-advancing parse
		}
		if ev != nil {
			events = append(events, ev)
		}
		d.buf = d.buf[n:]
	}
	if len(d.buf) == 0 {
		d.buf = d.buf[:0]
	}
	return events
}

// decodeOne attempts to decode a single event from the front of b. It returns
// the event (may be nil for consumed-but-empty), the number of bytes consumed,
// and whether a complete event was decoded. When ok is false the caller should
// wait for more input (unless final is set).
func decodeOne(b []byte, final bool) (Event, int, bool) {
	c := b[0]
	switch {
	case c == 0x1b:
		return decodeEsc(b, final)
	case c < 0x20 || c == 0x7f:
		return KeyEvent{Key: controlKey(c)}, 1, true
	default:
		return decodeRune(b, final)
	}
}

// decodeRune decodes a single UTF-8 printable rune.
func decodeRune(b []byte, final bool) (Event, int, bool) {
	r, size := utf8.DecodeRune(b)
	if r == utf8.RuneError && size <= 1 {
		if !final && !utf8.FullRune(b) {
			return nil, 0, false // partial multibyte rune; wait
		}
		return nil, 1, true // invalid byte; drop it
	}
	return KeyEvent{Key: RuneKey(r)}, size, true
}

// controlKey maps a C0/DEL control byte to a Key.
func controlKey(c byte) Key {
	switch c {
	case 0x00:
		return Ctrl(' ') // Ctrl+Space (also Ctrl+@): the karya leader
	case '\r', '\n':
		return Named(SymEnter)
	case '\t':
		return Named(SymTab)
	case 0x7f, 0x08:
		return Named(SymBackspace)
	case 0x1c:
		return Ctrl('\\')
	case 0x1d:
		return Ctrl(']')
	case 0x1e:
		return Ctrl('^')
	case 0x1f:
		return Ctrl('_')
	default:
		// 0x01–0x1a → Ctrl+a … Ctrl+z
		return Ctrl(rune('a' + c - 1))
	}
}

// decodeEsc handles anything beginning with ESC: CSI, SS3, Alt+rune, or a lone
// Esc.
func decodeEsc(b []byte, final bool) (Event, int, bool) {
	if len(b) == 1 {
		if final {
			return KeyEvent{Key: Named(SymEsc)}, 1, true
		}
		return nil, 0, false // maybe the start of a sequence
	}
	switch b[1] {
	case '[':
		return decodeCSI(b, final)
	case 'O':
		return decodeSS3(b, final)
	case 0x1b:
		// ESC ESC: emit one Esc, keep the second for the next round.
		return KeyEvent{Key: Named(SymEsc)}, 1, true
	default:
		// Alt + <rune>
		r, size := utf8.DecodeRune(b[1:])
		if r == utf8.RuneError && size <= 1 {
			if !final && !utf8.FullRune(b[1:]) {
				return nil, 0, false
			}
			return KeyEvent{Key: Named(SymEsc)}, 1, true
		}
		return KeyEvent{Key: Alt(r)}, 1 + size, true
	}
}

// decodeSS3 handles ESC O <final> (application cursor / F1–F4).
func decodeSS3(b []byte, final bool) (Event, int, bool) {
	if len(b) < 3 {
		if final {
			return KeyEvent{Key: Named(SymEsc)}, 1, true
		}
		return nil, 0, false
	}
	if k, ok := ss3Final(b[2]); ok {
		return KeyEvent{Key: k}, 3, true
	}
	return nil, 3, true // unknown SS3; drop
}

func ss3Final(c byte) (Key, bool) {
	switch c {
	case 'A':
		return Named(SymUp), true
	case 'B':
		return Named(SymDown), true
	case 'C':
		return Named(SymRight), true
	case 'D':
		return Named(SymLeft), true
	case 'H':
		return Named(SymHome), true
	case 'F':
		return Named(SymEnd), true
	case 'P':
		return Named(SymF1), true
	case 'Q':
		return Named(SymF2), true
	case 'R':
		return Named(SymF3), true
	case 'S':
		return Named(SymF4), true
	}
	return Key{}, false
}

// decodeCSI handles ESC [ ... <final>, including arrows, editing keys, function
// keys, modified keys, bracketed paste, and SGR mouse.
func decodeCSI(b []byte, final bool) (Event, int, bool) {
	// Find the final byte (0x40–0x7e) after "ESC[".
	end := -1
	for i := 2; i < len(b); i++ {
		if b[i] >= 0x40 && b[i] <= 0x7e {
			end = i
			break
		}
	}
	if end == -1 {
		if final {
			return KeyEvent{Key: Named(SymEsc)}, 1, true // give up; re-parse rest
		}
		return nil, 0, false // incomplete CSI
	}
	params := string(b[2:end])
	fin := b[end]
	seqLen := end + 1

	// Bracketed paste: ESC[200~ <text> ESC[201~
	if fin == '~' && params == "200" {
		return decodePaste(b, seqLen, final)
	}
	// SGR mouse: ESC[<...(M|m)
	if strings.HasPrefix(params, "<") && (fin == 'M' || fin == 'm') {
		if ev, ok := decodeSGRMouse(params[1:], fin); ok {
			return ev, seqLen, true
		}
		return nil, seqLen, true
	}

	nums := parseParams(params)
	mod := Mod(0)
	if len(nums) >= 2 {
		mod = decodeMod(nums[1])
	}

	switch fin {
	case 'A', 'B', 'C', 'D', 'H', 'F':
		if k, ok := csiLetter(fin); ok {
			k.Mod = mod
			return KeyEvent{Key: k}, seqLen, true
		}
	case 'Z':
		return KeyEvent{Key: Key{Sym: SymTab, Mod: ModShift}}, seqLen, true
	case '~':
		n := 0
		if len(nums) >= 1 {
			n = nums[0]
		}
		if k, ok := csiTilde(n); ok {
			k.Mod = mod
			return KeyEvent{Key: k}, seqLen, true
		}
	}
	return nil, seqLen, true // unknown CSI; drop
}

func csiLetter(fin byte) (Key, bool) {
	switch fin {
	case 'A':
		return Named(SymUp), true
	case 'B':
		return Named(SymDown), true
	case 'C':
		return Named(SymRight), true
	case 'D':
		return Named(SymLeft), true
	case 'H':
		return Named(SymHome), true
	case 'F':
		return Named(SymEnd), true
	}
	return Key{}, false
}

func csiTilde(n int) (Key, bool) {
	switch n {
	case 1, 7:
		return Named(SymHome), true
	case 2:
		return Named(SymInsert), true
	case 3:
		return Named(SymDelete), true
	case 4, 8:
		return Named(SymEnd), true
	case 5:
		return Named(SymPageUp), true
	case 6:
		return Named(SymPageDown), true
	case 11:
		return Named(SymF1), true
	case 12:
		return Named(SymF2), true
	case 13:
		return Named(SymF3), true
	case 14:
		return Named(SymF4), true
	case 15:
		return Named(SymF5), true
	case 17:
		return Named(SymF6), true
	case 18:
		return Named(SymF7), true
	case 19:
		return Named(SymF8), true
	case 20:
		return Named(SymF9), true
	case 21:
		return Named(SymF10), true
	case 23:
		return Named(SymF11), true
	case 24:
		return Named(SymF12), true
	}
	return Key{}, false
}

// decodePaste consumes a bracketed-paste block, returning the pasted text.
func decodePaste(b []byte, afterStart int, final bool) (Event, int, bool) {
	const term = "\x1b[201~"
	rest := b[afterStart:]
	idx := strings.Index(string(rest), term)
	if idx == -1 {
		if final {
			return PasteEvent{Text: string(rest)}, len(b), true
		}
		return nil, 0, false // wait for the terminator
	}
	text := string(rest[:idx])
	return PasteEvent{Text: text}, afterStart + idx + len(term), true
}

// decodeSGRMouse parses the body of an SGR mouse sequence (btn;x;y) with the
// given final byte ('M' press, 'm' release).
func decodeSGRMouse(body string, fin byte) (Event, bool) {
	nums := parseParams(body)
	if len(nums) != 3 {
		return nil, false
	}
	btn, x, y := nums[0], nums[1], nums[2]
	ev := MouseEvent{X: x - 1, Y: y - 1}
	if btn&4 != 0 {
		ev.Mod |= ModShift
	}
	if btn&8 != 0 {
		ev.Mod |= ModAlt
	}
	if btn&16 != 0 {
		ev.Mod |= ModCtrl
	}
	switch {
	case btn&64 != 0:
		if btn&1 != 0 {
			ev.Button = MouseWheelDown
		} else {
			ev.Button = MouseWheelUp
		}
		ev.Action = MousePress
	default:
		switch btn & 3 {
		case 0:
			ev.Button = MouseLeft
		case 1:
			ev.Button = MouseMiddle
		case 2:
			ev.Button = MouseRight
		}
		if fin == 'm' {
			ev.Action = MouseRelease
		} else if btn&32 != 0 {
			ev.Action = MouseMotion
		} else {
			ev.Action = MousePress
		}
	}
	return ev, true
}

// parseParams splits a ';'-separated CSI parameter string into ints.
func parseParams(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		nums = append(nums, n)
	}
	return nums
}

// decodeMod converts a CSI modifier parameter (1 + bitmask) into a Mod.
func decodeMod(n int) Mod {
	if n <= 1 {
		return 0
	}
	bits := n - 1
	var m Mod
	if bits&1 != 0 {
		m |= ModShift
	}
	if bits&2 != 0 {
		m |= ModAlt
	}
	if bits&4 != 0 {
		m |= ModCtrl
	}
	return m
}
