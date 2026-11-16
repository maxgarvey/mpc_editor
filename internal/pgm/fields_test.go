package pgm

import (
	"strings"
	"testing"
)

func TestDescribeOffset(t *testing.T) {
	pad := func(i, rel int) int { return padSectionStart + i*padLength + rel }
	cases := []struct {
		off  int
		want string
	}{
		{0x00, "header: file size"},
		{0x08, "header: version string"},
		{0x15, "header: unknown"},
		{pad(0, 0x03), "pad A1 layer 1 Sample name"},
		{pad(0, 0x18+0x14), "pad A1 layer 2 Tuning"},
		{pad(0, 0x18+0x15), "pad A1 layer 2 Tuning"},
		{pad(17, 0x10), "pad B2 layer 1 unknown"},
		{pad(0, 0x13), "pad A1 layer 1 Velocity range high"},
		{pad(63, 0x62), "pad D16 Voice Overlap"},
		{pad(0, 0x65), "pad A1 unknown (always 1)"},
		{pad(0, 0x60), "pad A1 unknown int16"},
		{pad(5, 0x61), "pad A6 unknown int16"},
		{pad(0, 0x66), "pad A1 Envelope Attack"},
		{pad(0, 0x71), "pad A1 Filter 1 Type"},
		{pad(0, 0x79), "pad A1 Filter 2 Type"},
		{pad(0, 0x8F), "pad A1 Mixer Level"},
		{pad(0, 0x94), "pad A1 Filter Pre-attenuation"},
		{pad(0, 0x69), "pad A1 unknown"},
		{pad(0, 0xA3), "pad A1 unknown"},
		{midiNotePadValue + 4, "pad→note table: pad A5"},
		{midiNoteToPad + 36, "note→pad table: note 36"},
		{midiProgramChange, "MIDI program change"},
		{sliderSection, "slider 1 Pad"},
		{sliderSection + 1, "slider 1 unknown (always 1)"},
		{sliderSection + sliderLength + 4, "slider 2 Tune high"},
		{ProgramFileSize - 1, "trailing unknown"},
		{ProgramFileSize, "out of range"},
		{-1, "out of range"},
	}
	for _, tc := range cases {
		got := DescribeOffset(tc.off)
		if !strings.HasPrefix(got, tc.want) {
			t.Errorf("DescribeOffset(0x%X) = %q, want prefix %q", tc.off, got, tc.want)
		}
	}
}

func TestPadName(t *testing.T) {
	for i, want := range map[int]string{0: "A1", 15: "A16", 16: "B1", 63: "D16"} {
		if got := PadName(i); got != want {
			t.Errorf("PadName(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestDiffBytes(t *testing.T) {
	a := NewProgram()
	b := a.Clone()
	b.Pad(0).Envelope().SetAttack(50)
	b.Pad(0).SetUnknownConstByte(0)
	d := DiffBytes(a.Buffer().Data(), b.Buffer().Data())
	if len(d) != 2 {
		t.Fatalf("got %d diffs, want 2: %+v", len(d), d)
	}
	if d[0].Field != "pad A1 unknown (always 1) (+0x65)" || d[0].A != 1 || d[0].B != 0 {
		t.Errorf("diff[0] = %+v", d[0])
	}
	if d[1].Field != "pad A1 Envelope Attack (+0x66)" || d[1].B != 50 {
		t.Errorf("diff[1] = %+v", d[1])
	}
	if got := DiffBytes(a.Buffer().Data(), a.Buffer().Data()[:10]); len(got) != 0 {
		t.Errorf("identical prefix: got %d diffs", len(got))
	}
}
