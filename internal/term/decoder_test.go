package term

import (
	"reflect"
	"testing"
)

func keys(ev []Event) []Key {
	var out []Key
	for _, e := range ev {
		if k, ok := e.(KeyEvent); ok {
			out = append(out, k.Key)
		}
	}
	return out
}

func TestDecodePrintable(t *testing.T) {
	d := NewDecoder()
	got := keys(d.Feed([]byte("abc")))
	want := []Key{RuneKey('a'), RuneKey('b'), RuneKey('c')}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDecodeUTF8Multibyte(t *testing.T) {
	d := NewDecoder()
	got := keys(d.Feed([]byte("é世")))
	want := []Key{RuneKey('é'), RuneKey('世')}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDecodeUTF8SplitAcrossFeeds(t *testing.T) {
	d := NewDecoder()
	b := []byte("世") // 3 bytes
	if ev := d.Feed(b[:1]); len(ev) != 0 {
		t.Fatalf("partial rune should yield no events, got %v", ev)
	}
	if ev := d.Feed(b[1:2]); len(ev) != 0 {
		t.Fatalf("still partial, got %v", ev)
	}
	got := keys(d.Feed(b[2:]))
	if !reflect.DeepEqual(got, []Key{RuneKey('世')}) {
		t.Fatalf("got %v", got)
	}
}

func TestDecodeControlKeys(t *testing.T) {
	cases := []struct {
		in   byte
		want Key
	}{
		{0x00, Ctrl(' ')}, // Ctrl+Space — the leader
		{'\r', Named(SymEnter)},
		{'\n', Named(SymEnter)},
		{'\t', Named(SymTab)},
		{0x7f, Named(SymBackspace)},
		{0x08, Named(SymBackspace)},
		{0x01, Ctrl('a')},
		{0x03, Ctrl('c')},
		{0x1a, Ctrl('z')},
	}
	for _, c := range cases {
		d := NewDecoder()
		got := keys(d.Feed([]byte{c.in}))
		if len(got) != 1 || got[0] != c.want {
			t.Fatalf("byte %#x -> %v want %v", c.in, got, c.want)
		}
	}
}

func TestDecodeLoneEscNeedsFlush(t *testing.T) {
	d := NewDecoder()
	if ev := d.Feed([]byte{0x1b}); len(ev) != 0 {
		t.Fatalf("lone ESC should buffer, got %v", ev)
	}
	got := keys(d.Flush())
	if len(got) != 1 || got[0] != Named(SymEsc) {
		t.Fatalf("flush -> %v want Esc", got)
	}
}

func TestDecodeArrowsCSI(t *testing.T) {
	cases := map[string]Key{
		"\x1b[A": Named(SymUp),
		"\x1b[B": Named(SymDown),
		"\x1b[C": Named(SymRight),
		"\x1b[D": Named(SymLeft),
		"\x1b[H": Named(SymHome),
		"\x1b[F": Named(SymEnd),
	}
	for in, want := range cases {
		d := NewDecoder()
		got := keys(d.Feed([]byte(in)))
		if len(got) != 1 || got[0] != want {
			t.Fatalf("%q -> %v want %v", in, got, want)
		}
	}
}

func TestDecodeArrowsSS3(t *testing.T) {
	d := NewDecoder()
	got := keys(d.Feed([]byte("\x1bOA\x1bOP")))
	want := []Key{Named(SymUp), Named(SymF1)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDecodeCSISplitAcrossFeeds(t *testing.T) {
	d := NewDecoder()
	if ev := d.Feed([]byte("\x1b")); len(ev) != 0 {
		t.Fatalf("ESC buffered")
	}
	if ev := d.Feed([]byte("[")); len(ev) != 0 {
		t.Fatalf("ESC[ still incomplete")
	}
	got := keys(d.Feed([]byte("A")))
	if len(got) != 1 || got[0] != Named(SymUp) {
		t.Fatalf("got %v want Up", got)
	}
}

func TestDecodeEditingAndFunctionKeys(t *testing.T) {
	cases := map[string]Key{
		"\x1b[2~":  Named(SymInsert),
		"\x1b[3~":  Named(SymDelete),
		"\x1b[5~":  Named(SymPageUp),
		"\x1b[6~":  Named(SymPageDown),
		"\x1b[15~": Named(SymF5),
		"\x1b[24~": Named(SymF12),
	}
	for in, want := range cases {
		d := NewDecoder()
		got := keys(d.Feed([]byte(in)))
		if len(got) != 1 || got[0] != want {
			t.Fatalf("%q -> %v want %v", in, got, want)
		}
	}
}

func TestDecodeModifiedArrow(t *testing.T) {
	d := NewDecoder()
	// ESC[1;5C = Ctrl+Right (mod param 5 -> 1+4 -> Ctrl)
	got := keys(d.Feed([]byte("\x1b[1;5C")))
	want := Key{Sym: SymRight, Mod: ModCtrl}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDecodeShiftTab(t *testing.T) {
	d := NewDecoder()
	got := keys(d.Feed([]byte("\x1b[Z")))
	want := Key{Sym: SymTab, Mod: ModShift}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDecodeAltRune(t *testing.T) {
	d := NewDecoder()
	got := keys(d.Feed([]byte("\x1bx")))
	if len(got) != 1 || got[0] != Alt('x') {
		t.Fatalf("got %v want Alt+x", got)
	}
}

func TestDecodeBracketedPaste(t *testing.T) {
	d := NewDecoder()
	in := "\x1b[200~hello\nworld\x1b[201~"
	ev := d.Feed([]byte(in))
	if len(ev) != 1 {
		t.Fatalf("expected 1 event, got %v", ev)
	}
	p, ok := ev[0].(PasteEvent)
	if !ok || p.Text != "hello\nworld" {
		t.Fatalf("paste = %+v", ev[0])
	}
}

func TestDecodeBracketedPasteSplit(t *testing.T) {
	d := NewDecoder()
	if ev := d.Feed([]byte("\x1b[200~partial")); len(ev) != 0 {
		t.Fatalf("incomplete paste should buffer, got %v", ev)
	}
	ev := d.Feed([]byte(" more\x1b[201~"))
	if len(ev) != 1 {
		t.Fatalf("expected paste after terminator, got %v", ev)
	}
	if p := ev[0].(PasteEvent); p.Text != "partial more" {
		t.Fatalf("paste text = %q", p.Text)
	}
}

func TestDecodeSGRMouse(t *testing.T) {
	d := NewDecoder()
	// button 0 press at col 10 row 5 (1-based -> 9,4 zero-based)
	ev := d.Feed([]byte("\x1b[<0;10;5M"))
	if len(ev) != 1 {
		t.Fatalf("expected 1 mouse event, got %v", ev)
	}
	m, ok := ev[0].(MouseEvent)
	if !ok {
		t.Fatalf("not a mouse event: %T", ev[0])
	}
	if m.X != 9 || m.Y != 4 || m.Button != MouseLeft || m.Action != MousePress {
		t.Fatalf("mouse = %+v", m)
	}
}

func TestDecodeSGRMouseWheel(t *testing.T) {
	d := NewDecoder()
	ev := d.Feed([]byte("\x1b[<64;1;1M"))
	m := ev[0].(MouseEvent)
	if m.Button != MouseWheelUp {
		t.Fatalf("wheel = %+v", m)
	}
}

func TestDecodeMixedStream(t *testing.T) {
	d := NewDecoder()
	got := keys(d.Feed([]byte("a\x1b[Bz")))
	want := []Key{RuneKey('a'), Named(SymDown), RuneKey('z')}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestKeyString(t *testing.T) {
	cases := []struct {
		k    Key
		want string
	}{
		{Ctrl(' '), "Ctrl+Space"},
		{RuneKey('a'), "a"},
		{Alt('x'), "Alt+x"},
		{Named(SymUp), "Up"},
		{Named(SymEnter), "Enter"},
		{Key{Sym: SymTab, Mod: ModShift}, "Shift+Tab"},
		{Ctrl('c'), "Ctrl+c"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Fatalf("%+v String() = %q want %q", c.k, got, c.want)
		}
	}
}
