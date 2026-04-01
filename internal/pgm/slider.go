package pgm

const (
	sliderSection = 0x29D9
	sliderLength  = 0x29E6 - sliderSection // 13 bytes per slider
)

// Slider parameter definitions. These use absolute offsets (not relative to pad).
// The Java code creates Slider with BaseElement(parent, 0, sliderIndex, SLIDER_LENGTH),
// meaning slider 0 starts at 0x29D9 and slider 1 at 0x29D9 + 13.
var (
	SliderPad        = OffIntParam("Pad", 0x00, 0, 64)
	SliderParameter  = EnumParam("Parameter", 0x02, []string{"Tune", "Filter", "Layer", "Attack", "Decay"})
	SliderTuneRange  = RangeParam("Tune", 0x03, -120, 120)
	SliderFilterRange = RangeParam("Filter", 0x05, -50, 50)
	SliderLayerRange = RangeParam("Layer", 0x07, 0, 127)
	SliderAttackRange = RangeParam("Attack", 0x09, 0, 100)
	SliderDecayRange = RangeParam("Decay", 0x0B, 0, 100)
)

var SliderParameters = []Parameter{SliderPad, SliderParameter, SliderTuneRange, SliderFilterRange, SliderLayerRange, SliderAttackRange, SliderDecayRange}

// Slider represents one of 2 sliders in the program.
type Slider struct {
	buf   *Buffer
	index int // 0 or 1
}

func (s *Slider) base() int {
	return sliderSection + s.index*sliderLength
}

// Index returns the slider index (0 or 1).
func (s *Slider) Index() int {
	return s.index
}

// GetPad returns which pad this slider controls (0=off).
func (s *Slider) GetPad() int {
	return int(s.buf.GetByte(s.base() + SliderPad.Offset))
}

// SetPad sets which pad this slider controls.
func (s *Slider) SetPad(pad int) {
	s.buf.SetByte(s.base()+SliderPad.Offset, byte(pad))
}

// GetParameter returns which parameter this slider controls (0=Tune, 1=Filter, etc.).
func (s *Slider) GetParameter() int {
	return int(s.buf.GetByte(s.base() + SliderParameter.Offset))
}

// SetParameter sets which parameter this slider controls.
func (s *Slider) SetParameter(param int) {
	s.buf.SetByte(s.base()+SliderParameter.Offset, byte(param))
}

// GetRange returns the range for the given parameter type.
func (s *Slider) GetRange(param Parameter) Range {
	return s.buf.GetRange(s.base() + param.Offset)
}

// SetRange sets the range for the given parameter type.
func (s *Slider) SetRange(param Parameter, r Range) {
	s.buf.SetRange(s.base()+param.Offset, r)
}
