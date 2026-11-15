package pgm

import "fmt"

const (
	fileVersion = "MPC1000 PGM 1.00"

	headerFileSize = 0x00
	headerFileType = 0x04

	midiNotePadValue  = 0x2918 // 64 bytes: pad → MIDI note (indexed by pad)
	midiNoteToPad     = 0x2958 // 128 bytes: MIDI note → pad (indexed by note; noPad = unassigned)
	midiProgramChange = 0x29D8

	midiNoteCount = 128
	noPad         = 64

	// Bytes Norum documents as "unknown, always 0x01"; hardware files have 1.
	padConstOneOffset    = 0x65 // relative to pad base
	sliderConstOneOffset = 0x01 // relative to slider base

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

// NewProgram creates a blank program with default header and pad defaults.
func NewProgram() *Program {
	p := &Program{buf: NewEmptyBuffer(ProgramFileSize)}
	// Write file size header
	p.buf.SetInt(headerFileSize, ProgramFileSize)
	// Write file type/version
	_ = p.buf.SetString(headerFileType, fileVersion)
	for n := range midiNoteCount {
		p.buf.SetByte(midiNoteToPad+n, noPad)
	}
	// Defaults below match a new program saved by the MPC 1000 itself
	// (testdata/chromatic.pgm); a test asserts the two are byte-identical.
	for s := range slidersPerProg {
		sl := p.Slider(s)
		p.buf.SetByte(sl.base()+sliderConstOneOffset, 1)
		sl.SetRange(SliderTuneRange, Range{Low: -120, High: 120})
		sl.SetRange(SliderFilterRange, Range{Low: -50, High: 50})
		sl.SetRange(SliderLayerRange, Range{Low: 0, High: 127})
		sl.SetRange(SliderAttackRange, Range{Low: 0, High: 100})
		sl.SetRange(SliderDecayRange, Range{Low: 0, High: 100})
	}
	p.Slider(1).SetParameter(1) // slider 2 defaults to Filter
	for i := range padCount {
		pad := p.Pad(i)
		p.buf.SetByte(pad.base+padConstOneOffset, 1)
		for j := range layersPerPad {
			pad.Layer(j).SetLevel(70)
			pad.Layer(j).SetRange(Range{Low: 0, High: 127})
		}
		pad.Envelope().SetDecay(5)
		pad.Envelope().SetVelocityToLevel(100)
		pad.Filter1().SetFrequency(100)
		pad.Filter2().SetFrequency(100)
		pad.Mixer().SetLevel(100)
		pad.Mixer().SetPan(50)
		pad.Mixer().SetFXSendLevel(33)
		pad.SetMIDINote(35 + i) // chromatic default: A1=35, A2=36, ...
	}
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
	p := &Program{buf: buf}
	p.RepairNoteMap()
	return p, nil
}

// RepairNoteMap fixes entries in the note→pad table that disagree with the
// pad→note table, as left by editors that only write the latter (or by older
// versions of this one, which left the table zeroed). Each consistent entry is
// kept; an inconsistent one is pointed at the lowest pad playing that note, or
// set to "no pad". It returns true if anything changed.
func (p *Program) RepairNoteMap() bool {
	changed := false
	for n := range midiNoteCount {
		cur := int(p.buf.GetByte(midiNoteToPad + n))
		if cur < padCount && int(p.buf.GetByte(midiNotePadValue+cur)) == n {
			continue
		}
		want := lowestPadForNote(p.buf, n)
		if cur != want {
			p.buf.SetByte(midiNoteToPad+n, byte(want))
			changed = true
		}
	}
	return changed
}

// PadForNote returns the pad the note→pad table assigns to note, or -1.
func (p *Program) PadForNote(note int) int {
	if note < 0 || note >= midiNoteCount {
		return -1
	}
	pad := int(p.buf.GetByte(midiNoteToPad + note))
	if pad >= padCount {
		return -1
	}
	return pad
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
