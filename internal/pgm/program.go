package pgm

import "fmt"

const (
	fileVersion = "MPC1000 PGM 1.00"

	headerFileSize = 0x00
	headerFileType = 0x04

	midiNotePadValue  = 0x2918 // 64 bytes: MIDI note → pad mapping
	midiProgramChange = 0x29D8

	padSectionStart = 0x18
	padLength       = 0xA4
	padCount        = 64
	layersPerPad    = 4
	slidersPerProg  = 2
)

// MIDIProgramChange is the parameter for the program's MIDI program change value.
var MIDIProgramChange = OffIntParam("MIDI Program Change", midiProgramChange, 0, 128)

// Program represents an MPC1000 program file (.pgm).
// It wraps a fixed-size byte buffer and provides access to pads, sliders, and global params.
type Program struct {
	buf *Buffer
}

// NewProgram creates a blank program with default header.
func NewProgram() *Program {
	p := &Program{buf: NewEmptyBuffer(ProgramFileSize)}
	// Write file size header
	p.buf.SetInt(headerFileSize, ProgramFileSize)
	// Write file type/version
	_ = p.buf.SetString(headerFileType, fileVersion)
	return p
}

// OpenProgram reads a .pgm file from disk.
func OpenProgram(path string) (*Program, error) {
	buf, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	return programFromBuffer(buf)
}

func programFromBuffer(buf *Buffer) (*Program, error) {
	if buf.Len() != ProgramFileSize {
		return nil, fmt.Errorf("invalid pgm: expected %d bytes, got %d", ProgramFileSize, buf.Len())
	}
	return &Program{buf: buf}, nil
}

// Save writes the program to a file.
func (p *Program) Save(path string) error {
	return p.buf.SaveFile(path)
}

// Buffer returns the underlying buffer (for advanced/test use).
func (p *Program) Buffer() *Buffer {
	return p.buf
}

// Pad returns a Pad view for the given pad index (0-63).
func (p *Program) Pad(index int) *Pad {
	if index < 0 || index >= padCount {
		panic(fmt.Sprintf("pad index %d out of range [0, %d)", index, padCount))
	}
	return &Pad{
		buf:   p.buf,
		base:  padSectionStart + index*padLength,
		index: index,
		prog:  p,
	}
}

// PadCount returns 64 (the number of pads in a program).
func (p *Program) PadCount() int {
	return padCount
}

// Slider returns a Slider view for the given slider index (0-1).
func (p *Program) Slider(index int) *Slider {
	if index < 0 || index >= slidersPerProg {
		panic(fmt.Sprintf("slider index %d out of range [0, %d)", index, slidersPerProg))
	}
	return &Slider{
		buf:   p.buf,
		index: index,
	}
}

// GetMIDIProgramChange returns the MIDI program change value (0=off).
func (p *Program) GetMIDIProgramChange() int {
	return int(p.buf.GetByte(midiProgramChange))
}

// SetMIDIProgramChange sets the MIDI program change value.
func (p *Program) SetMIDIProgramChange(value int) error {
	if value < 0 || value > 128 {
		return fmt.Errorf("MIDI program change %d out of range [0, 128]", value)
	}
	p.buf.SetByte(midiProgramChange, byte(value))
	return nil
}

// Clone returns a deep copy of the program.
func (p *Program) Clone() *Program {
	newData := make([]byte, len(p.buf.data))
	copy(newData, p.buf.data)
	return &Program{buf: NewBuffer(newData)}
}
