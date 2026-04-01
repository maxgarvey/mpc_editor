package pgm

// Filter1 parameter offsets (relative to pad base).
var (
	Filter1Type          = EnumParam("Type", 0x71, []string{"Off", "Lowpass", "Bandpass", "Highpass"})
	Filter1Freq          = IntParam("Frequency", 0x72, 0, 100)
	Filter1Res           = IntParam("Resonance", 0x73, 0, 100)
	Filter1VelocityToFreq = IntParam("Velocity to Freq.", 0x78, 0, 100)
	FilterAttenuation    = EnumParam("Pre-attenuation", 0x94, []string{"0dB", "-6dB", "-12dB"})
)

var Filter1Parameters = []Parameter{Filter1Type, Filter1Freq, Filter1Res, Filter1VelocityToFreq, FilterAttenuation}

// Filter1 represents the first filter section of a pad.
type Filter1 struct {
	buf  *Buffer
	base int
}

func (f *Filter1) GetType() int            { return int(f.buf.GetByte(f.base + Filter1Type.Offset)) }
func (f *Filter1) SetType(v int)            { f.buf.SetByte(f.base+Filter1Type.Offset, byte(v)) }
func (f *Filter1) GetFrequency() int        { return int(f.buf.GetByte(f.base + Filter1Freq.Offset)) }
func (f *Filter1) SetFrequency(v int)        { f.buf.SetByte(f.base+Filter1Freq.Offset, byte(v)) }
func (f *Filter1) GetResonance() int        { return int(f.buf.GetByte(f.base + Filter1Res.Offset)) }
func (f *Filter1) SetResonance(v int)        { f.buf.SetByte(f.base+Filter1Res.Offset, byte(v)) }
func (f *Filter1) GetVelocityToFreq() int   { return int(f.buf.GetByte(f.base + Filter1VelocityToFreq.Offset)) }
func (f *Filter1) SetVelocityToFreq(v int)   { f.buf.SetByte(f.base+Filter1VelocityToFreq.Offset, byte(v)) }
func (f *Filter1) GetAttenuation() int      { return int(f.buf.GetByte(f.base + FilterAttenuation.Offset)) }
func (f *Filter1) SetAttenuation(v int)      { f.buf.SetByte(f.base+FilterAttenuation.Offset, byte(v)) }

func (f *Filter1) CopyFrom(src *Filter1) {
	for _, p := range Filter1Parameters {
		_ = p.Set(f.buf, f.base, p.Get(src.buf, src.base))
	}
}

// Filter2 parameter offsets (relative to pad base).
var (
	Filter2Type          = EnumParam("Type", 0x79, []string{"Off", "Lowpass", "Bandpass", "Highpass", "Link"})
	Filter2Freq          = IntParam("Frequency", 0x7A, 0, 100)
	Filter2Res           = IntParam("Resonance", 0x7B, 0, 100)
	Filter2VelocityToFreq = IntParam("Velocity to Freq.", 0x80, 0, 100)
)

var Filter2Parameters = []Parameter{Filter2Type, Filter2Freq, Filter2Res, Filter2VelocityToFreq}

// Filter2 represents the second filter section of a pad.
type Filter2 struct {
	buf  *Buffer
	base int
}

func (f *Filter2) GetType() int            { return int(f.buf.GetByte(f.base + Filter2Type.Offset)) }
func (f *Filter2) SetType(v int)            { f.buf.SetByte(f.base+Filter2Type.Offset, byte(v)) }
func (f *Filter2) GetFrequency() int        { return int(f.buf.GetByte(f.base + Filter2Freq.Offset)) }
func (f *Filter2) SetFrequency(v int)        { f.buf.SetByte(f.base+Filter2Freq.Offset, byte(v)) }
func (f *Filter2) GetResonance() int        { return int(f.buf.GetByte(f.base + Filter2Res.Offset)) }
func (f *Filter2) SetResonance(v int)        { f.buf.SetByte(f.base+Filter2Res.Offset, byte(v)) }
func (f *Filter2) GetVelocityToFreq() int   { return int(f.buf.GetByte(f.base + Filter2VelocityToFreq.Offset)) }
func (f *Filter2) SetVelocityToFreq(v int)   { f.buf.SetByte(f.base+Filter2VelocityToFreq.Offset, byte(v)) }

func (f *Filter2) CopyFrom(src *Filter2) {
	for _, p := range Filter2Parameters {
		_ = p.Set(f.buf, f.base, p.Get(src.buf, src.base))
	}
}
