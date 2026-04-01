package pgm

// Pad parameter offsets (relative to pad base).
var (
	PadVoiceOverlap = EnumParam("Voice Overlap", 0x62, []string{"Poly", "Mono"})
	PadMuteGroup    = OffIntParam("Mute Group", 0x63, 0, 32)
	// Note: PAD_MIDI_NOTE_VALUE is at absolute offset 0x2918+padIndex, handled specially.
	PadMIDINote = EnumParam("Note", 0x00, midiNoteNames()) // offset overridden in get/set
)

// Pad represents one of 64 pads in an MPC program.
// It is a view into the program's buffer at a specific offset.
type Pad struct {
	buf   *Buffer
	base  int // absolute offset of this pad's data
	index int // 0-63
	prog  *Program
}

// Index returns the pad index (0-63).
func (p *Pad) Index() int {
	return p.index
}

// Layer returns a Layer view for the given layer index (0-3).
func (p *Pad) Layer(index int) *Layer {
	if index < 0 || index >= layersPerPad {
		panic("layer index out of range [0, 4)")
	}
	return &Layer{
		buf:  p.buf,
		base: p.base + index*layerLength,
		index: index,
		pad:  p,
	}
}

// LayerCount returns 4.
func (p *Pad) LayerCount() int {
	return layersPerPad
}

// GetVoiceOverlap returns 0=Poly, 1=Mono.
func (p *Pad) GetVoiceOverlap() int {
	return int(p.buf.GetByte(p.base + PadVoiceOverlap.Offset))
}

// SetVoiceOverlap sets voice overlap mode (0=Poly, 1=Mono).
func (p *Pad) SetVoiceOverlap(value int) {
	p.buf.SetByte(p.base+PadVoiceOverlap.Offset, byte(value))
}

// GetMuteGroup returns the mute group (0=Off, 1-32).
func (p *Pad) GetMuteGroup() int {
	return int(p.buf.GetByte(p.base + PadMuteGroup.Offset))
}

// SetMuteGroup sets the mute group.
func (p *Pad) SetMuteGroup(value int) {
	p.buf.SetByte(p.base+PadMuteGroup.Offset, byte(value))
}

// GetMIDINote returns the MIDI note number for this pad (0-127).
// The MIDI note map is at absolute offset 0x2918 + padIndex.
func (p *Pad) GetMIDINote() int {
	return int(p.buf.GetByte(midiNotePadValue + p.index))
}

// SetMIDINote sets the MIDI note number for this pad.
func (p *Pad) SetMIDINote(note int) {
	p.buf.SetByte(midiNotePadValue+p.index, byte(note))
}

// Envelope returns the envelope section of this pad.
func (p *Pad) Envelope() *Envelope {
	return &Envelope{buf: p.buf, base: p.base}
}

// Filter1 returns the first filter section of this pad.
func (p *Pad) Filter1() *Filter1 {
	return &Filter1{buf: p.buf, base: p.base}
}

// Filter2 returns the second filter section of this pad.
func (p *Pad) Filter2() *Filter2 {
	return &Filter2{buf: p.buf, base: p.base}
}

// Mixer returns the mixer section of this pad.
func (p *Pad) Mixer() *Mixer {
	return &Mixer{buf: p.buf, base: p.base}
}

// GetParam reads a parameter value relative to the pad's base offset.
func (p *Pad) GetParam(param Parameter) interface{} {
	return param.Get(p.buf, p.base)
}

// SetParam writes a parameter value relative to the pad's base offset.
func (p *Pad) SetParam(param Parameter, value interface{}) error {
	return param.Set(p.buf, p.base, value)
}

// CopyFrom copies all pad parameters from src, except those whose labels are in the ignore set.
func (p *Pad) CopyFrom(src *Pad, ignoreLabels map[string]bool) {
	params := []Parameter{PadVoiceOverlap, PadMuteGroup}
	for _, param := range params {
		if ignoreLabels[param.Label] {
			continue
		}
		_ = param.Set(p.buf, p.base, param.Get(src.buf, src.base))
	}

	// Copy layers
	for i := 0; i < layersPerPad; i++ {
		dstLayer := p.Layer(i)
		srcLayer := src.Layer(i)
		dstLayer.CopyFrom(srcLayer, ignoreLabels)
	}

	// Copy envelope, filters, mixer
	p.Envelope().CopyFrom(src.Envelope())
	p.Filter1().CopyFrom(src.Filter1())
	p.Filter2().CopyFrom(src.Filter2())
	p.Mixer().CopyFrom(src.Mixer())
}

func midiNoteNames() []string {
	noteNames := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	notes := make([]string, 64)
	for i := range notes {
		k := 35 + i
		chromatic := (k - 24) % 12
		octave := (k - 24) / 12
		notes[i] = noteNames[chromatic] + string(rune('0'+octave))
	}
	return notes
}
