package msgpack

import (
	"bytes"
	"reflect"
	"testing"
)

func decodeBytes(t *testing.T, b []byte) any {
	t.Helper()
	v, err := NewDecoder(bytes.NewReader(b)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}

func roundTrip(t *testing.T, v any) any {
	t.Helper()
	b, err := Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return decodeBytes(t, b)
}

func TestEncodeKnownBytes(t *testing.T) {
	cases := []struct {
		v    any
		want []byte
	}{
		{nil, []byte{0xc0}},
		{true, []byte{0xc3}},
		{false, []byte{0xc2}},
		{0, []byte{0x00}},
		{127, []byte{0x7f}},
		{-1, []byte{0xff}},
		{-32, []byte{0xe0}},
		{128, []byte{0xcc, 0x80}},
		{"", []byte{0xa0}},
		{"hi", []byte{0xa2, 'h', 'i'}},
		{[]any{1, 2}, []byte{0x92, 0x01, 0x02}},
	}
	for _, c := range cases {
		got, err := Marshal(c.v)
		if err != nil {
			t.Fatalf("marshal %v: %v", c.v, err)
		}
		if !bytes.Equal(got, c.want) {
			t.Fatalf("Marshal(%v) = % x want % x", c.v, got, c.want)
		}
	}
}

func TestRoundTripScalars(t *testing.T) {
	cases := []struct {
		in   any
		want any
	}{
		{nil, nil},
		{true, true},
		{false, false},
		{0, int64(0)},
		{42, int64(42)},
		{127, int64(127)},
		{128, int64(128)},
		{300, int64(300)},
		{70000, int64(70000)},
		{5000000000, int64(5000000000)},
		{-1, int64(-1)},
		{-32, int64(-32)},
		{-33, int64(-33)},
		{-200, int64(-200)},
		{-40000, int64(-40000)},
		{"hello world", "hello world"},
		{3.14, 3.14},
	}
	for _, c := range cases {
		got := roundTrip(t, c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("roundtrip(%v) = %v (%T) want %v (%T)", c.in, got, got, c.want, c.want)
		}
	}
}

func TestRoundTripLongString(t *testing.T) {
	s := string(bytes.Repeat([]byte("x"), 5000))
	if got := roundTrip(t, s); got != s {
		t.Fatalf("long string round-trip failed (len got %d)", len(got.(string)))
	}
}

func TestRoundTripArrayAndMap(t *testing.T) {
	in := []any{int64(1), "two", true, []any{int64(3)}}
	got := roundTrip(t, in)
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("array round-trip: %v want %v", got, in)
	}

	m := map[string]any{"foreground": int64(255), "bold": true, "name": "x"}
	gm := roundTrip(t, m)
	if !reflect.DeepEqual(gm, m) {
		t.Fatalf("map round-trip: %v want %v", gm, m)
	}
}

func TestRoundTripBinary(t *testing.T) {
	in := []byte{0, 1, 2, 250}
	got := roundTrip(t, in)
	if !bytes.Equal(got.([]byte), in) {
		t.Fatalf("binary round-trip: %v", got)
	}
}

func TestRoundTripExt(t *testing.T) {
	in := Ext{Type: 0, Data: []byte{0x2a}}
	got := roundTrip(t, in)
	e, ok := got.(Ext)
	if !ok || e.Type != 0 || !bytes.Equal(e.Data, in.Data) {
		t.Fatalf("ext round-trip: %#v", got)
	}
}

func TestDecodeRPCResponseShape(t *testing.T) {
	// A msgpack-RPC response: [1, msgid, err, result].
	msg := []any{int64(1), int64(7), nil, "ok"}
	b, err := Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeBytes(t, b).([]any)
	if len(got) != 4 || got[0] != int64(1) || got[1] != int64(7) || got[2] != nil || got[3] != "ok" {
		t.Fatalf("rpc shape decode wrong: %v", got)
	}
}

func TestStreamingMultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	for _, v := range []any{int64(1), "a", []any{int64(2)}} {
		b, _ := Marshal(v)
		buf.Write(b)
	}
	dec := NewDecoder(&buf)
	if v, _ := dec.Decode(); v != int64(1) {
		t.Fatalf("first = %v", v)
	}
	if v, _ := dec.Decode(); v != "a" {
		t.Fatalf("second = %v", v)
	}
	if v, _ := dec.Decode(); !reflect.DeepEqual(v, []any{int64(2)}) {
		t.Fatalf("third = %v", v)
	}
}

func TestExtFixWidthEncoding(t *testing.T) {
	// 8-byte ext should use fixext8 (0xd7).
	b, _ := Marshal(Ext{Type: 1, Data: make([]byte, 8)})
	if b[0] != 0xd7 {
		t.Fatalf("fixext8 prefix = 0x%02x", b[0])
	}
	// 3-byte ext should use ext8 (0xc7).
	b, _ = Marshal(Ext{Type: 1, Data: make([]byte, 3)})
	if b[0] != 0xc7 || b[1] != 3 {
		t.Fatalf("ext8 header = % x", b[:2])
	}
}
