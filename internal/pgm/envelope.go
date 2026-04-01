package pgm

// Envelope parameter offsets (relative to pad base).
var (
	EnvAttack          = IntParam("Attack", 0x66, 0, 100)
	EnvDecay           = IntParam("Decay", 0x67, 0, 100)
	EnvDecayMode       = EnumParam("Decay Mode", 0x68, []string{"End", "Start"})
	EnvVelocityToLevel = IntParam("Velocity to Level", 0x6B, 0, 100)
)

var EnvelopeParameters = []Parameter{EnvAttack, EnvDecay, EnvDecayMode, EnvVelocityToLevel}

// Envelope represents the envelope section of a pad.
type Envelope struct {
	buf  *Buffer
	base int // pad base offset
}

func (e *Envelope) GetAttack() int     { return int(e.buf.GetByte(e.base + EnvAttack.Offset)) }
func (e *Envelope) SetAttack(v int)    { e.buf.SetByte(e.base+EnvAttack.Offset, byte(v)) }
func (e *Envelope) GetDecay() int      { return int(e.buf.GetByte(e.base + EnvDecay.Offset)) }
func (e *Envelope) SetDecay(v int)     { e.buf.SetByte(e.base+EnvDecay.Offset, byte(v)) }
func (e *Envelope) GetDecayMode() int  { return int(e.buf.GetByte(e.base + EnvDecayMode.Offset)) }
func (e *Envelope) SetDecayMode(v int) { e.buf.SetByte(e.base+EnvDecayMode.Offset, byte(v)) }
func (e *Envelope) GetVelocityToLevel() int {
	return int(e.buf.GetByte(e.base + EnvVelocityToLevel.Offset))
}
func (e *Envelope) SetVelocityToLevel(v int) {
	e.buf.SetByte(e.base+EnvVelocityToLevel.Offset, byte(v))
}

func (e *Envelope) CopyFrom(src *Envelope) {
	for _, p := range EnvelopeParameters {
		_ = p.Set(e.buf, e.base, p.Get(src.buf, src.base))
	}
}
