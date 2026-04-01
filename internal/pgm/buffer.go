package pgm

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ProgramFileSize is the fixed size of an MPC1000 .pgm file.
const ProgramFileSize = 0x2A04 // 10756 bytes

// Range represents a low/high byte pair (e.g., velocity range 0-127).
type Range struct {
	Low  int
	High int
}

// Contains returns true if v is within [Low, High].
func (r Range) Contains(v float64) bool {
	return v >= float64(r.Low) && v <= float64(r.High)
}

// Buffer provides little-endian binary read/write operations on a byte slice.
// All offsets are absolute positions within the slice.
type Buffer struct {
	data []byte
}

// NewBuffer creates a Buffer wrapping the given byte slice.
func NewBuffer(data []byte) *Buffer {
	return &Buffer{data: data}
}

// NewEmptyBuffer creates a zero-filled Buffer of the given length.
func NewEmptyBuffer(length int) *Buffer {
	return &Buffer{data: make([]byte, length)}
}

// Len returns the length of the underlying data.
func (b *Buffer) Len() int {
	return len(b.data)
}

// Data returns the underlying byte slice.
func (b *Buffer) Data() []byte {
	return b.data
}

// GetByte reads a single byte at the given offset.
func (b *Buffer) GetByte(offset int) byte {
	return b.data[offset]
}

// SetByte writes a single byte at the given offset.
func (b *Buffer) SetByte(offset int, value byte) {
	b.data[offset] = value
}

// GetShort reads a little-endian int16 at the given offset.
func (b *Buffer) GetShort(offset int) int16 {
	return int16(binary.LittleEndian.Uint16(b.data[offset : offset+2]))
}

// SetShort writes a little-endian int16 at the given offset.
func (b *Buffer) SetShort(offset int, value int16) {
	binary.LittleEndian.PutUint16(b.data[offset:offset+2], uint16(value))
}

// GetInt reads a little-endian int32 at the given offset.
func (b *Buffer) GetInt(offset int) int32 {
	return int32(binary.LittleEndian.Uint32(b.data[offset : offset+4]))
}

// SetInt writes a little-endian int32 at the given offset.
func (b *Buffer) SetInt(offset int, value int32) {
	binary.LittleEndian.PutUint32(b.data[offset:offset+4], uint32(value))
}

// GetString reads a null-terminated string (max 16 chars) at the given offset.
func (b *Buffer) GetString(offset int) string {
	for i := 0; i < 16; i++ {
		if b.data[offset+i] == 0 {
			return string(b.data[offset : offset+i])
		}
	}
	return string(b.data[offset : offset+16])
}

// SetString writes a string (max 16 chars) at the given offset, null-padding to 16 bytes.
func (b *Buffer) SetString(offset int, s string) error {
	if len(s) > 16 {
		return fmt.Errorf("string too long: %q (16 chars max)", s)
	}
	// Zero-fill the 16-byte field
	for i := 0; i < 16; i++ {
		b.data[offset+i] = 0
	}
	// Write the string bytes
	copy(b.data[offset:offset+16], []byte(s))
	return nil
}

// GetRange reads a Range (low byte, high byte) at the given offset.
func (b *Buffer) GetRange(offset int) Range {
	return Range{
		Low:  int(b.data[offset]),
		High: int(b.data[offset+1]),
	}
}

// SetRange writes a Range (low byte, high byte) at the given offset.
func (b *Buffer) SetRange(offset int, r Range) {
	b.data[offset] = byte(r.Low)
	b.data[offset+1] = byte(r.High)
}

// OpenFile reads a .pgm file into a Buffer.
func OpenFile(path string) (*Buffer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pgm: %w", err)
	}
	defer f.Close()
	return OpenReader(f)
}

// OpenReader reads exactly ProgramFileSize bytes from a reader into a Buffer.
func OpenReader(r io.Reader) (*Buffer, error) {
	data := make([]byte, ProgramFileSize)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("read pgm: %w", err)
	}
	return NewBuffer(data), nil
}

// SaveFile writes the buffer to a .pgm file.
func (b *Buffer) SaveFile(path string) error {
	return os.WriteFile(path, b.data, 0644)
}
