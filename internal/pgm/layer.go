package pgm

const layerLength = 0x18 // 24 bytes per layer

// Layer parameter definitions (offsets relative to layer base).
var (
	LayerSampleName = StringParam("Sample", 0x00)
	LayerLevel      = IntParam("Level", 0x11, 0, 100)
	LayerRange      = RangeParam("Range", 0x12, 0, 127)
	LayerTuning     = TuningParam("Tuning", 0x14, -36, 36)
	LayerPlayMode   = EnumParam("Play Mode", 0x16, []string{"One Shot", "Note On"})
)

// LayerParameters is the list of all layer parameters.
var LayerParameters = []Parameter{LayerSampleName, LayerLevel, LayerTuning, LayerPlayMode, LayerRange}

// Layer represents a sample layer on a pad (4 layers per pad).
type Layer struct {
	buf   *Buffer
	base  int // absolute offset of this layer's data
	index int // 0-3
	pad   *Pad
}

// Index returns the layer index (0-3).
func (l *Layer) Index() int {
	return l.index
}

// GetSampleName returns the sample name (max 16 chars, no extension).
func (l *Layer) GetSampleName() string {
	return l.buf.GetString(l.base + LayerSampleName.Offset)
}

// SetSampleName sets the sample name.
func (l *Layer) SetSampleName(name string) error {
	return l.buf.SetString(l.base+LayerSampleName.Offset, name)
}

// GetLevel returns the level (0-100).
func (l *Layer) GetLevel() int {
	return int(l.buf.GetByte(l.base + LayerLevel.Offset))
}

// SetLevel sets the level (0-100).
func (l *Layer) SetLevel(value int) {
	l.buf.SetByte(l.base+LayerLevel.Offset, byte(value))
}

// GetTuning returns the tuning in semitones (-36.00 to +36.00).
func (l *Layer) GetTuning() float64 {
	return float64(l.buf.GetShort(l.base+LayerTuning.Offset)) / 100.0
}

// SetTuning sets the tuning in semitones.
func (l *Layer) SetTuning(semitones float64) {
	l.buf.SetShort(l.base+LayerTuning.Offset, int16(semitones*100.0))
}

// GetPlayMode returns 0=One Shot, 1=Note On.
func (l *Layer) GetPlayMode() int {
	return int(l.buf.GetByte(l.base + LayerPlayMode.Offset))
}

// SetPlayMode sets the play mode (0=One Shot, 1=Note On).
func (l *Layer) SetPlayMode(value int) {
	l.buf.SetByte(l.base+LayerPlayMode.Offset, byte(value))
}

// IsOneShot returns true if play mode is One Shot.
func (l *Layer) IsOneShot() bool {
	return l.GetPlayMode() == 0
}

// GetRange returns the velocity range.
func (l *Layer) GetRange() Range {
	return l.buf.GetRange(l.base + LayerRange.Offset)
}

// SetRange sets the velocity range.
func (l *Layer) SetRange(r Range) {
	l.buf.SetRange(l.base+LayerRange.Offset, r)
}

// GetParam reads a parameter value relative to the layer's base offset.
func (l *Layer) GetParam(param Parameter) interface{} {
	return param.Get(l.buf, l.base)
}

// SetParam writes a parameter value relative to the layer's base offset.
func (l *Layer) SetParam(param Parameter, value interface{}) error {
	return param.Set(l.buf, l.base, value)
}

// CopyFrom copies all layer parameters from src, except those whose labels are in the ignore set.
func (l *Layer) CopyFrom(src *Layer, ignoreLabels map[string]bool) {
	for _, param := range LayerParameters {
		if ignoreLabels[param.Label] {
			continue
		}
		_ = param.Set(l.buf, l.base, param.Get(src.buf, src.base))
	}
}
