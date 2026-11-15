package pgm

import "fmt"

// Raw access for hardware probes and file forensics.
//
// The normal API keeps the program self-consistent: OpenProgram repairs the
// note→pad table and Pad.SetMIDINote updates both note tables together. Tools
// that must see or write a file exactly as bytes (to check what the hardware
// saved, or to build a deliberately inconsistent probe) use the functions
// below instead. Editor code should not.

// OpenProgramRaw reads a .pgm file without calling RepairNoteMap, so the note
// tables are left exactly as they are on disk.
func OpenProgramRaw(path string) (*Program, error) {
	buf, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	if buf.Len() != ProgramFileSize {
		return nil, fmt.Errorf("invalid pgm: expected %d bytes, got %d", ProgramFileSize, buf.Len())
	}
	return &Program{buf: buf}, nil
}

// SetNoteMapEntry writes one entry of the note→pad table (pad 64 = no pad)
// without touching the pad→note table. The result may be inconsistent, and
// OpenProgram will repair it on the next load.
func (p *Program) SetNoteMapEntry(note, pad int) error {
	if note < 0 || note >= midiNoteCount {
		return fmt.Errorf("note %d out of range [0, %d)", note, midiNoteCount)
	}
	if pad < 0 || pad > noPad {
		return fmt.Errorf("pad %d out of range [0, %d]", pad, noPad)
	}
	p.buf.SetByte(midiNoteToPad+note, byte(pad))
	return nil
}

// UnknownConstByte returns the undocumented pad byte at 0x65, which is 1 in
// every hardware file seen so far.
func (p *Pad) UnknownConstByte() byte {
	return p.buf.GetByte(p.base + padConstOneOffset)
}

// SetUnknownConstByte overwrites the undocumented pad byte at 0x65.
func (p *Pad) SetUnknownConstByte(v byte) {
	p.buf.SetByte(p.base+padConstOneOffset, v)
}

// UnknownConstByte returns the undocumented slider byte at offset 1, which is
// 1 in every hardware file seen so far.
func (s *Slider) UnknownConstByte() byte {
	return s.buf.GetByte(s.base() + sliderConstOneOffset)
}

// SetUnknownConstByte overwrites the undocumented slider byte at offset 1.
func (s *Slider) SetUnknownConstByte(v byte) {
	s.buf.SetByte(s.base()+sliderConstOneOffset, v)
}
