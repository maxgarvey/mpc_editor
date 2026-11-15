package pgm

import "fmt"

// ByteDiff is one differing byte between two program buffers.
type ByteDiff struct {
	Offset int
	Field  string // human-readable field name from DescribeOffset
	A, B   byte
}

// DiffBytes compares two program buffers byte by byte and names each
// differing offset. Used to analyse files saved by the hardware.
// Bytes beyond the shorter buffer are ignored.
func DiffBytes(a, b []byte) []ByteDiff {
	n := min(len(a), len(b))
	var out []ByteDiff
	for i := range n {
		if a[i] != b[i] {
			out = append(out, ByteDiff{Offset: i, Field: DescribeOffset(i), A: a[i], B: b[i]})
		}
	}
	return out
}

// fieldSpan names a byte range relative to some base.
type fieldSpan struct {
	off, size int
	name      string
}

var layerFields = []fieldSpan{
	{0x00, 16, "Sample name"},
	{0x10, 1, "unknown"},
	{0x11, 1, "Level"},
	{0x12, 1, "Velocity range low"},
	{0x13, 1, "Velocity range high"},
	{0x14, 2, "Tuning"},
	{0x16, 1, "Play mode"},
	{0x17, 1, "unknown"},
}

var padFields = buildPadFields()

var sliderFields = buildSliderFields()

func paramSpans(prefix string, params []Parameter) []fieldSpan {
	var out []fieldSpan
	for _, p := range params {
		name := prefix + p.Label
		switch p.Type {
		case TypeRange:
			out = append(out, fieldSpan{p.Offset, 1, name + " low"}, fieldSpan{p.Offset + 1, 1, name + " high"})
		case TypeTuning:
			out = append(out, fieldSpan{p.Offset, 2, name})
		case TypeString:
			out = append(out, fieldSpan{p.Offset, 16, name})
		default:
			out = append(out, fieldSpan{p.Offset, 1, name})
		}
	}
	return out
}

func buildPadFields() []fieldSpan {
	out := paramSpans("", []Parameter{PadVoiceOverlap, PadMuteGroup})
	out = append(out, fieldSpan{padConstOneOffset, 1, "unknown (always 1)"})
	out = append(out, paramSpans("Envelope ", EnvelopeParameters)...)
	out = append(out, paramSpans("Filter 1 ", []Parameter{Filter1Type, Filter1Freq, Filter1Res, Filter1VelocityToFreq})...)
	out = append(out, paramSpans("Filter 2 ", Filter2Parameters)...)
	out = append(out, paramSpans("Mixer ", MixerParameters)...)
	out = append(out, paramSpans("Filter ", []Parameter{FilterAttenuation})...)
	return out
}

func buildSliderFields() []fieldSpan {
	out := paramSpans("", SliderParameters)
	return append(out, fieldSpan{sliderConstOneOffset, 1, "unknown (always 1)"})
}

func lookupSpan(spans []fieldSpan, rel int) string {
	for _, s := range spans {
		if rel >= s.off && rel < s.off+s.size {
			return s.name
		}
	}
	return "unknown"
}

// PadName returns the hardware label for a pad index, e.g. 0 → "A1", 17 → "B2".
func PadName(index int) string {
	return fmt.Sprintf("%c%d", 'A'+index/16, index%16+1)
}

// DescribeOffset names the field at an absolute offset in a .pgm buffer,
// e.g. "pad A1 layer 1 Tuning" or "note→pad table: note 36". Bytes the
// format does not document are reported as "unknown" with their location.
func DescribeOffset(off int) string {
	switch {
	case off < 0 || off >= ProgramFileSize:
		return "out of range"
	case off < headerFileType:
		return "header: file size"
	case off < headerFileType+16:
		return "header: version string"
	case off < padSectionStart:
		return "header: unknown"
	case off < midiNotePadValue:
		rel := off - padSectionStart
		pad, prel := rel/padLength, rel%padLength
		if prel < layersPerPad*layerLength {
			return fmt.Sprintf("pad %s layer %d %s (+0x%02X)", PadName(pad), prel/layerLength+1,
				lookupSpan(layerFields, prel%layerLength), prel)
		}
		return fmt.Sprintf("pad %s %s (+0x%02X)", PadName(pad), lookupSpan(padFields, prel), prel)
	case off < midiNoteToPad:
		pad := off - midiNotePadValue
		return fmt.Sprintf("pad→note table: pad %s", PadName(pad))
	case off < midiProgramChange:
		return fmt.Sprintf("note→pad table: note %d", off-midiNoteToPad)
	case off == midiProgramChange:
		return "MIDI program change"
	case off < sliderSection+slidersPerProg*sliderLength:
		rel := off - sliderSection
		return fmt.Sprintf("slider %d %s (+0x%02X)", rel/sliderLength+1, lookupSpan(sliderFields, rel%sliderLength), rel%sliderLength)
	default:
		return "trailing unknown"
	}
}
