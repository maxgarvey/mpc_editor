package pgm

// Mixer parameter offsets (relative to pad base).
var (
	MixerLevel   = IntParam("Level", 0x8F, 0, 100)
	MixerPan     = IntParam("Pan", 0x90, 0, 100) // 0-49=Left, 50=Center, 51-100=Right
	MixerOutput  = EnumParam("Output", 0x91, []string{"Stereo", "1-2", "3-4"})
	MixerFXSend  = EnumParam("FX Send", 0x92, []string{"Off", "1", "2"})
	MixerFXLevel = IntParam("FX Send Level", 0x93, 0, 100)
)

var MixerParameters = []Parameter{MixerLevel, MixerPan, MixerOutput, MixerFXSend, MixerFXLevel}

// Mixer represents the mixer/FX section of a pad.
type Mixer struct {
	buf  *Buffer
	base int
}

func (m *Mixer) GetLevel() int        { return int(m.buf.GetByte(m.base + MixerLevel.Offset)) }
func (m *Mixer) SetLevel(v int)       { m.buf.SetByte(m.base+MixerLevel.Offset, byte(v)) }
func (m *Mixer) GetPan() int          { return int(m.buf.GetByte(m.base + MixerPan.Offset)) }
func (m *Mixer) SetPan(v int)         { m.buf.SetByte(m.base+MixerPan.Offset, byte(v)) }
func (m *Mixer) GetOutput() int       { return int(m.buf.GetByte(m.base + MixerOutput.Offset)) }
func (m *Mixer) SetOutput(v int)      { m.buf.SetByte(m.base+MixerOutput.Offset, byte(v)) }
func (m *Mixer) GetFXSend() int       { return int(m.buf.GetByte(m.base + MixerFXSend.Offset)) }
func (m *Mixer) SetFXSend(v int)      { m.buf.SetByte(m.base+MixerFXSend.Offset, byte(v)) }
func (m *Mixer) GetFXSendLevel() int  { return int(m.buf.GetByte(m.base + MixerFXLevel.Offset)) }
func (m *Mixer) SetFXSendLevel(v int) { m.buf.SetByte(m.base+MixerFXLevel.Offset, byte(v)) }

func (m *Mixer) CopyFrom(src *Mixer) {
	for _, p := range MixerParameters {
		_ = p.Set(m.buf, m.base, p.Get(src.buf, src.base))
	}
}
