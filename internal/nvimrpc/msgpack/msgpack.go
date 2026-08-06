// Package msgpack is a minimal MessagePack codec covering the subset Neovim's
// msgpack-RPC uses: nil, bool, ints, floats, strings, binary, arrays, maps, and
// ext (Neovim's Buffer/Window/Tabpage handles). It is stdlib-only (no external
// msgpack library, per AGENTS.md) and deliberately small — just what the
// protocol emits — with a streaming Decoder so the RPC reader can pull one
// message at a time off nvim's stdout (DESIGN.md §6.3).
package msgpack

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Ext is a MessagePack extension value. Neovim uses ext types for API object
// handles (Buffer=0, Window=1, Tabpage=2); the payload is a packed integer id.
type Ext struct {
	Type int8
	Data []byte
}

// Marshal encodes v into MessagePack bytes. Supported Go types: nil, bool,
// int/int8..64, uint/uint8..64, float32/64, string, []byte, []any,
// map[string]any, and Ext. Other types are an error.
func Marshal(v any) ([]byte, error) {
	var b []byte
	b, err := appendValue(b, v)
	return b, err
}

func appendValue(b []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(b, 0xc0), nil
	case bool:
		if x {
			return append(b, 0xc3), nil
		}
		return append(b, 0xc2), nil
	case int:
		return appendInt(b, int64(x)), nil
	case int8:
		return appendInt(b, int64(x)), nil
	case int16:
		return appendInt(b, int64(x)), nil
	case int32:
		return appendInt(b, int64(x)), nil
	case int64:
		return appendInt(b, x), nil
	case uint:
		return appendUint(b, uint64(x)), nil
	case uint8:
		return appendUint(b, uint64(x)), nil
	case uint16:
		return appendUint(b, uint64(x)), nil
	case uint32:
		return appendUint(b, uint64(x)), nil
	case uint64:
		return appendUint(b, x), nil
	case float32:
		return appendFloat64(b, float64(x)), nil
	case float64:
		return appendFloat64(b, x), nil
	case string:
		return appendString(b, x), nil
	case []byte:
		return appendBinary(b, x), nil
	case []any:
		return appendArray(b, x)
	case map[string]any:
		return appendMap(b, x)
	case Ext:
		return appendExt(b, x), nil
	default:
		return b, fmt.Errorf("msgpack: unsupported type %T", v)
	}
}

func appendInt(b []byte, n int64) []byte {
	switch {
	case n >= 0:
		return appendUint(b, uint64(n))
	case n >= -32:
		return append(b, byte(0xe0|(n+32)))
	case n >= math.MinInt8:
		return append(b, 0xd0, byte(n))
	case n >= math.MinInt16:
		b = append(b, 0xd1)
		return binary.BigEndian.AppendUint16(b, uint16(n))
	case n >= math.MinInt32:
		b = append(b, 0xd2)
		return binary.BigEndian.AppendUint32(b, uint32(n))
	default:
		b = append(b, 0xd3)
		return binary.BigEndian.AppendUint64(b, uint64(n))
	}
}

func appendUint(b []byte, n uint64) []byte {
	switch {
	case n < 0x80:
		return append(b, byte(n))
	case n <= math.MaxUint8:
		return append(b, 0xcc, byte(n))
	case n <= math.MaxUint16:
		b = append(b, 0xcd)
		return binary.BigEndian.AppendUint16(b, uint16(n))
	case n <= math.MaxUint32:
		b = append(b, 0xce)
		return binary.BigEndian.AppendUint32(b, uint32(n))
	default:
		b = append(b, 0xcf)
		return binary.BigEndian.AppendUint64(b, n)
	}
}

func appendFloat64(b []byte, f float64) []byte {
	b = append(b, 0xcb)
	return binary.BigEndian.AppendUint64(b, math.Float64bits(f))
}

func appendString(b []byte, s string) []byte {
	n := len(s)
	switch {
	case n < 32:
		b = append(b, 0xa0|byte(n))
	case n <= math.MaxUint8:
		b = append(b, 0xd9, byte(n))
	case n <= math.MaxUint16:
		b = append(b, 0xda)
		b = binary.BigEndian.AppendUint16(b, uint16(n))
	default:
		b = append(b, 0xdb)
		b = binary.BigEndian.AppendUint32(b, uint32(n))
	}
	return append(b, s...)
}

func appendBinary(b, data []byte) []byte {
	n := len(data)
	switch {
	case n <= math.MaxUint8:
		b = append(b, 0xc4, byte(n))
	case n <= math.MaxUint16:
		b = append(b, 0xc5)
		b = binary.BigEndian.AppendUint16(b, uint16(n))
	default:
		b = append(b, 0xc6)
		b = binary.BigEndian.AppendUint32(b, uint32(n))
	}
	return append(b, data...)
}

func appendArray(b []byte, a []any) ([]byte, error) {
	n := len(a)
	switch {
	case n < 16:
		b = append(b, 0x90|byte(n))
	case n <= math.MaxUint16:
		b = append(b, 0xdc)
		b = binary.BigEndian.AppendUint16(b, uint16(n))
	default:
		b = append(b, 0xdd)
		b = binary.BigEndian.AppendUint32(b, uint32(n))
	}
	var err error
	for _, e := range a {
		if b, err = appendValue(b, e); err != nil {
			return b, err
		}
	}
	return b, nil
}

func appendMap(b []byte, m map[string]any) ([]byte, error) {
	n := len(m)
	switch {
	case n < 16:
		b = append(b, 0x80|byte(n))
	case n <= math.MaxUint16:
		b = append(b, 0xde)
		b = binary.BigEndian.AppendUint16(b, uint16(n))
	default:
		b = append(b, 0xdf)
		b = binary.BigEndian.AppendUint32(b, uint32(n))
	}
	var err error
	for k, v := range m {
		b = appendString(b, k)
		if b, err = appendValue(b, v); err != nil {
			return b, err
		}
	}
	return b, nil
}

func appendExt(b []byte, e Ext) []byte {
	n := len(e.Data)
	switch n {
	case 1:
		b = append(b, 0xd4)
	case 2:
		b = append(b, 0xd5)
	case 4:
		b = append(b, 0xd6)
	case 8:
		b = append(b, 0xd7)
	case 16:
		b = append(b, 0xd8)
	default:
		switch {
		case n <= math.MaxUint8:
			b = append(b, 0xc7, byte(n))
		case n <= math.MaxUint16:
			b = append(b, 0xc8)
			b = binary.BigEndian.AppendUint16(b, uint16(n))
		default:
			b = append(b, 0xc9)
			b = binary.BigEndian.AppendUint32(b, uint32(n))
		}
	}
	b = append(b, byte(e.Type))
	return append(b, e.Data...)
}

// Decoder reads MessagePack values from a stream, one at a time.
type Decoder struct {
	r *bufio.Reader
}

// NewDecoder returns a Decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

// Decode reads and returns the next value. Integers decode to int64, floats to
// float64, strings to string, binary to []byte, arrays to []any, maps to
// map[string]any (Neovim uses string keys), and ext to Ext. nil decodes to nil.
func (d *Decoder) Decode() (any, error) {
	c, err := d.r.ReadByte()
	if err != nil {
		return nil, err
	}
	switch {
	case c <= 0x7f: // positive fixint
		return int64(c), nil
	case c >= 0xe0: // negative fixint
		return int64(int8(c)), nil
	case c >= 0xa0 && c <= 0xbf: // fixstr
		return d.readString(int(c & 0x1f))
	case c >= 0x90 && c <= 0x9f: // fixarray
		return d.readArray(int(c & 0x0f))
	case c >= 0x80 && c <= 0x8f: // fixmap
		return d.readMap(int(c & 0x0f))
	}
	switch c {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xcc:
		return d.readUint(1)
	case 0xcd:
		return d.readUint(2)
	case 0xce:
		return d.readUint(4)
	case 0xcf:
		return d.readUint(8)
	case 0xd0:
		return d.readInt(1)
	case 0xd1:
		return d.readInt(2)
	case 0xd2:
		return d.readInt(4)
	case 0xd3:
		return d.readInt(8)
	case 0xca:
		return d.readFloat32()
	case 0xcb:
		return d.readFloat64()
	case 0xd9:
		return d.readStringN(1)
	case 0xda:
		return d.readStringN(2)
	case 0xdb:
		return d.readStringN(4)
	case 0xc4:
		return d.readBinN(1)
	case 0xc5:
		return d.readBinN(2)
	case 0xc6:
		return d.readBinN(4)
	case 0xdc:
		return d.readArrayN(2)
	case 0xdd:
		return d.readArrayN(4)
	case 0xde:
		return d.readMapN(2)
	case 0xdf:
		return d.readMapN(4)
	case 0xd4:
		return d.readExt(1)
	case 0xd5:
		return d.readExt(2)
	case 0xd6:
		return d.readExt(4)
	case 0xd7:
		return d.readExt(8)
	case 0xd8:
		return d.readExt(16)
	case 0xc7:
		return d.readExtN(1)
	case 0xc8:
		return d.readExtN(2)
	case 0xc9:
		return d.readExtN(4)
	}
	return nil, fmt.Errorf("msgpack: unknown prefix 0x%02x", c)
}

func (d *Decoder) readN(n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(d.r, buf)
	return buf, err
}

func (d *Decoder) readUint(n int) (any, error) {
	b, err := d.readN(n)
	if err != nil {
		return nil, err
	}
	var v uint64
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	return int64(v), nil
}

func (d *Decoder) readInt(n int) (any, error) {
	b, err := d.readN(n)
	if err != nil {
		return nil, err
	}
	var v int64
	if b[0]&0x80 != 0 {
		v = -1 // sign-extend
	}
	for _, x := range b {
		v = v<<8 | int64(x)
	}
	return v, nil
}

func (d *Decoder) readFloat32() (any, error) {
	b, err := d.readN(4)
	if err != nil {
		return nil, err
	}
	return float64(math.Float32frombits(binary.BigEndian.Uint32(b))), nil
}

func (d *Decoder) readFloat64() (any, error) {
	b, err := d.readN(8)
	if err != nil {
		return nil, err
	}
	return math.Float64frombits(binary.BigEndian.Uint64(b)), nil
}

func (d *Decoder) readLen(n int) (int, error) {
	b, err := d.readN(n)
	if err != nil {
		return 0, err
	}
	v := 0
	for _, x := range b {
		v = v<<8 | int(x)
	}
	return v, nil
}

func (d *Decoder) readString(n int) (any, error) {
	b, err := d.readN(n)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (d *Decoder) readStringN(lenBytes int) (any, error) {
	n, err := d.readLen(lenBytes)
	if err != nil {
		return nil, err
	}
	return d.readString(n)
}

func (d *Decoder) readBinN(lenBytes int) (any, error) {
	n, err := d.readLen(lenBytes)
	if err != nil {
		return nil, err
	}
	return d.readN(n)
}

func (d *Decoder) readArray(n int) (any, error) {
	out := make([]any, n)
	for i := 0; i < n; i++ {
		v, err := d.Decode()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (d *Decoder) readArrayN(lenBytes int) (any, error) {
	n, err := d.readLen(lenBytes)
	if err != nil {
		return nil, err
	}
	return d.readArray(n)
}

func (d *Decoder) readMap(n int) (any, error) {
	out := make(map[string]any, n)
	for i := 0; i < n; i++ {
		k, err := d.Decode()
		if err != nil {
			return nil, err
		}
		v, err := d.Decode()
		if err != nil {
			return nil, err
		}
		out[keyString(k)] = v
	}
	return out, nil
}

func (d *Decoder) readMapN(lenBytes int) (any, error) {
	n, err := d.readLen(lenBytes)
	if err != nil {
		return nil, err
	}
	return d.readMap(n)
}

func (d *Decoder) readExt(n int) (any, error) {
	t, err := d.r.ReadByte()
	if err != nil {
		return nil, err
	}
	data, err := d.readN(n)
	if err != nil {
		return nil, err
	}
	return Ext{Type: int8(t), Data: data}, nil
}

func (d *Decoder) readExtN(lenBytes int) (any, error) {
	n, err := d.readLen(lenBytes)
	if err != nil {
		return nil, err
	}
	return d.readExt(n)
}

// keyString renders a decoded map key as a string (Neovim uses string keys).
func keyString(k any) string {
	if s, ok := k.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", k)
}
