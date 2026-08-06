// Package term is karya's stdlib-only terminal layer: the input Decoder that
// turns a raw byte stream into Key/Mouse/Paste/Resize events, the ANSI Output
// writer that flushes cellbuf spans to the screen, terminal capability
// detection, and a thin build-tagged raw-mode wrapper.
//
// The Decoder and Output are pure and unit-testable (bytes in, events or bytes
// out); only raw-mode and size queries touch a real terminal (DESIGN.md §6.1,
// §8.1).
package term

import "fmt"

// Mod is a bitmask of modifier keys held with a key press.
type Mod uint8

// Modifier bits.
const (
	ModShift Mod = 1 << iota
	ModAlt
	ModCtrl
)

// Has reports whether all bits in m are set.
func (m Mod) Has(bits Mod) bool { return m&bits == bits }

// Sym names a key. KeyRune means the press carries a printable rune in
// Key.Rune; every other value is a named non-printable key.
type Sym uint16

// Key symbols.
const (
	SymNone Sym = iota
	SymRune
	SymEnter
	SymTab
	SymEsc
	SymBackspace
	SymDelete
	SymInsert
	SymUp
	SymDown
	SymLeft
	SymRight
	SymHome
	SymEnd
	SymPageUp
	SymPageDown
	SymF1
	SymF2
	SymF3
	SymF4
	SymF5
	SymF6
	SymF7
	SymF8
	SymF9
	SymF10
	SymF11
	SymF12
)

// Key is a single, canonical key press. It is the unit the keymap engine
// matches against, so its String form is stable and used in binding tables.
type Key struct {
	Sym  Sym
	Rune rune // valid when Sym == SymRune
	Mod  Mod
}

// Rune builds a printable-rune key with no modifiers.
func RuneKey(r rune) Key { return Key{Sym: SymRune, Rune: r} }

// Ctrl builds a Ctrl+<rune> key (e.g. Ctrl(' ') is the karya leader).
func Ctrl(r rune) Key { return Key{Sym: SymRune, Rune: r, Mod: ModCtrl} }

// Alt builds an Alt+<rune> key.
func Alt(r rune) Key { return Key{Sym: SymRune, Rune: r, Mod: ModAlt} }

// Named builds a named key (e.g. Named(SymEnter)).
func Named(s Sym) Key { return Key{Sym: s} }

// IsRune reports whether the key is a printable rune press.
func (k Key) IsRune() bool { return k.Sym == SymRune }

var symNames = map[Sym]string{
	SymEnter: "Enter", SymTab: "Tab", SymEsc: "Esc", SymBackspace: "Backspace",
	SymDelete: "Delete", SymInsert: "Insert", SymUp: "Up", SymDown: "Down",
	SymLeft: "Left", SymRight: "Right", SymHome: "Home", SymEnd: "End",
	SymPageUp: "PageUp", SymPageDown: "PageDown",
	SymF1: "F1", SymF2: "F2", SymF3: "F3", SymF4: "F4", SymF5: "F5", SymF6: "F6",
	SymF7: "F7", SymF8: "F8", SymF9: "F9", SymF10: "F10", SymF11: "F11", SymF12: "F12",
}

// String returns a stable, human-readable form such as "Ctrl+Space", "Alt+x",
// "Up", or "a". It is used in keymap tables and which-key popups.
func (k Key) String() string {
	prefix := ""
	if k.Mod.Has(ModCtrl) {
		prefix += "Ctrl+"
	}
	if k.Mod.Has(ModAlt) {
		prefix += "Alt+"
	}
	if k.Mod.Has(ModShift) && k.Sym != SymRune {
		prefix += "Shift+"
	}
	if k.Sym == SymRune {
		if k.Rune == ' ' {
			return prefix + "Space"
		}
		return prefix + string(k.Rune)
	}
	if name, ok := symNames[k.Sym]; ok {
		return prefix + name
	}
	return fmt.Sprintf("%sSym(%d)", prefix, k.Sym)
}
