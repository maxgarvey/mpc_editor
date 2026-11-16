package seq

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// standardLikeLengths mirrors Akai's factory "Standard" sequence: bars 1-9
// in 4/4, 10-25 in 3/4, 26-47 in 4/4 (47 bars, 16512 ticks).
func standardLikeLengths() []int {
	var l []int
	for range 9 {
		l = append(l, 384)
	}
	for range 16 {
		l = append(l, 288)
	}
	for range 22 {
		l = append(l, 384)
	}
	return l
}

// withClockMap writes bar starts for the given lengths; zeroFill selects the
// factory convention (zeros after the end tick) over counting on in 4/4.
func withClockMap(data []byte, lengths []int, zeroFill bool) []byte {
	binary.LittleEndian.PutUint16(data[barsOffset:], uint16(len(lengths)))
	tick := uint32(0)
	for n := range clockMapEntries {
		switch {
		case n <= len(lengths):
			putClockEntry(data, n, tick)
			if n < len(lengths) {
				tick += uint32(lengths[n])
			}
		case zeroFill:
			putClockEntry(data, n, 0)
		default:
			tick += TicksPerBar
			putClockEntry(data, n, tick)
		}
	}
	return data
}

func TestClockMapMixedMeter(t *testing.T) {
	s, err := Parse(withClockMap(buildTestSEQ(95, 1, nil), standardLikeLengths(), true))
	if err != nil {
		t.Fatal(err)
	}
	if s.Bars != 47 || len(s.BarStarts) != 48 {
		t.Fatalf("bars=%d starts=%d", s.Bars, len(s.BarStarts))
	}
	if got := s.TotalTicks(); got != 16512 {
		t.Errorf("TotalTicks = %d, want 16512", got)
	}
	if s.BarLength(9) != 384 || s.BarLength(10) != 288 || s.BarLength(25) != 288 || s.BarLength(26) != 384 {
		t.Errorf("bar lengths 9/10/25/26 = %d/%d/%d/%d", s.BarLength(9), s.BarLength(10), s.BarLength(25), s.BarLength(26))
	}
	want := []MeterChange{{1, 384}, {10, 288}, {26, 384}}
	got := s.MeterChanges()
	if len(got) != len(want) {
		t.Fatalf("MeterChanges = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MeterChanges = %v, want %v", got, want)
		}
	}
}

func TestClockMapUniformHardware(t *testing.T) {
	data, err := os.ReadFile("../../testdata/seq/Sequence01.SEQ")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := Parse(data)
	if s.TotalTicks() != 384 || len(s.MeterChanges()) != 1 {
		t.Errorf("TotalTicks=%d changes=%v", s.TotalTicks(), s.MeterChanges())
	}
}

// Files with a zeroed map (e.g. testdata/test.seq) fall back to 4/4.
func TestClockMapAbsentFallsBack(t *testing.T) {
	s, _ := Parse(buildTestSEQ(120, 3, nil))
	if s.BarStarts != nil {
		t.Fatalf("BarStarts = %v, want nil", s.BarStarts)
	}
	if s.TotalTicks() != 3*384 || s.BarLength(2) != 384 {
		t.Errorf("TotalTicks=%d BarLength=%d", s.TotalTicks(), s.BarLength(2))
	}
}

func TestCreateWritesUniformClockMap(t *testing.T) {
	data := Create(120, 2, "", "", false, nil)
	for _, n := range []int{0, 1, 2, 999} {
		if got := clockEntry(data, n); got != uint32(n*384) {
			t.Errorf("entry %d = %d", n, got)
		}
		if data[clockMapOffset+n*4+3] != 0x60 {
			t.Errorf("entry %d byte 3 = %#x", n, data[clockMapOffset+n*4+3])
		}
	}
}

func patchBars(t *testing.T, data []byte, bars int) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "p.SEQ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PatchFile(path, 95, bars); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	return out
}

func TestPatchFileGrowsMixedMeterMap(t *testing.T) {
	out := patchBars(t, withClockMap(buildTestSEQ(95, 1, nil), standardLikeLengths(), true), 49)
	s, _ := Parse(out)
	if s.TotalTicks() != 16512+2*384 {
		t.Errorf("TotalTicks = %d", s.TotalTicks())
	}
	if s.BarLength(11) != 288 || s.BarLength(48) != 384 {
		t.Errorf("existing 3/4 bar or new bar wrong: %d %d", s.BarLength(11), s.BarLength(48))
	}
	if clockEntry(out, 50) != 0 || out[clockMapOffset+50*4+3] != 0 {
		t.Error("zero-fill convention not kept after the new end tick")
	}
}

func TestPatchFileShrinksMixedMeterMap(t *testing.T) {
	out := patchBars(t, withClockMap(buildTestSEQ(95, 1, nil), standardLikeLengths(), true), 12)
	s, _ := Parse(out)
	if s.TotalTicks() != 9*384+3*288 {
		t.Errorf("TotalTicks = %d", s.TotalTicks())
	}
	for n := 13; n <= 47; n++ {
		if clockEntry(out, n) != 0 {
			t.Fatalf("entry %d not cleared", n)
		}
	}
}

func TestPatchFileUniformMapKeepsCounting(t *testing.T) {
	out := patchBars(t, Create(120, 2, "", "", false, nil), 4)
	if clockEntry(out, 4) != 4*384 || clockEntry(out, 999) != 999*384 {
		t.Errorf("entries 4/999 = %d/%d", clockEntry(out, 4), clockEntry(out, 999))
	}
}

func TestPatchFileLeavesAbsentMapAlone(t *testing.T) {
	out := patchBars(t, buildTestSEQ(120, 2, nil), 4)
	for n := range clockMapEntries {
		if clockEntry(out, n) != 0 {
			t.Fatalf("entry %d written into an absent map", n)
		}
	}
}

func TestBuildGridUsesClockMapLength(t *testing.T) {
	s, _ := Parse(withClockMap(buildTestSEQ(95, 1, nil), standardLikeLengths(), true))
	g := BuildGrid(s, nil, DefaultGridParams())
	// 16512 ticks / 384 = 43 bars of 4/4 display (not 47).
	if g.TotalBars != 43 {
		t.Errorf("TotalBars = %d, want 43", g.TotalBars)
	}
}

func TestTimeSignatureForBarTicks(t *testing.T) {
	for ticks, want := range map[int]string{384: "4_4", 288: "3_4", 192: "2_4", 480: "5_4", 336: "7_8", 100: ""} {
		if got := TimeSignatureForBarTicks(ticks); got != want {
			t.Errorf("%d → %q, want %q", ticks, got, want)
		}
	}
}
