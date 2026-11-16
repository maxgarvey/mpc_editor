package seq

import "encoding/binary"

// The clock map at 0x30 is a table of bar start ticks: entry n holds the tick
// at which bar n starts (24-bit LE in bytes 0-2; byte 3 is 0x60 = 96 in every
// observed file, probably ticks per beat). Entry Bars holds the end tick.
// Hardware files in 4/4 keep counting +384 through all 1000 entries; Akai's
// factory "Standard" sequence (bars in 3/4 between 4/4 sections) stops after
// the end tick and zero-fills the rest. Time signature changes live only here.
const (
	clockMapOffset  = 0x30
	clockMapEntries = 1000
	clockMapByte3   = 0x60
)

func clockEntry(data []byte, n int) uint32 {
	off := clockMapOffset + n*4
	return uint32(data[off]) | uint32(data[off+1])<<8 | uint32(data[off+2])<<16
}

func putClockEntry(data []byte, n int, tick uint32) {
	off := clockMapOffset + n*4
	if tick == 0 && n > 0 {
		binary.LittleEndian.PutUint32(data[off:], 0)
		return
	}
	data[off] = byte(tick)
	data[off+1] = byte(tick >> 8)
	data[off+2] = byte(tick >> 16)
	data[off+3] = clockMapByte3
}

// parseClockMap returns the start tick of bars 0..bars (bars+1 entries, the
// last being the end tick), or nil when the map is absent or inconsistent
// (some third-party files leave it zeroed).
func parseClockMap(data []byte, bars int) []uint32 {
	if bars < 1 || bars >= clockMapEntries || len(data) < clockMapOffset+clockMapEntries*4 {
		return nil
	}
	starts := make([]uint32, bars+1)
	for n := 0; n <= bars; n++ {
		starts[n] = clockEntry(data, n)
		if n > 0 && starts[n] <= starts[n-1] {
			return nil
		}
	}
	if starts[0] != 0 {
		return nil
	}
	return starts
}

// rebuildClockMap rewrites the map for a new bar count, keeping the lengths of
// existing bars and giving added bars 384 ticks (4/4). The file's convention
// for entries past the end (keep counting vs. zero-fill) is preserved. A map
// that is absent or inconsistent is left untouched.
func rebuildClockMap(data []byte, oldBars, newBars int) {
	old := parseClockMap(data, oldBars)
	if old == nil || newBars < 1 || newBars >= clockMapEntries {
		return
	}
	zeroFill := oldBars+1 < clockMapEntries && clockEntry(data, oldBars+1) == 0

	starts := make([]uint32, newBars+1)
	for n := 0; n <= newBars; n++ {
		if n <= oldBars {
			starts[n] = old[n]
		} else {
			starts[n] = starts[n-1] + TicksPerBar
		}
	}
	for n := 0; n < clockMapEntries; n++ {
		switch {
		case n <= newBars:
			putClockEntry(data, n, starts[n])
		case zeroFill:
			putClockEntry(data, n, 0)
		default:
			putClockEntry(data, n, starts[newBars]+uint32(n-newBars)*TicksPerBar)
		}
	}
}

// writeUniformClockMap fills all 1000 entries with 4/4 bar starts, as the
// hardware does for a sequence with no time signature changes.
func writeUniformClockMap(data []byte) {
	for n := range clockMapEntries {
		putClockEntry(data, n, uint32(n)*TicksPerBar)
	}
}

// MeterChange marks a bar whose length differs from the bar before it.
type MeterChange struct {
	Bar   int // 1-indexed
	Ticks int // bar length in ticks (384 = 4/4, 288 = 3/4 …)
}

// TotalTicks returns the sequence length in ticks: the clock map's end tick
// when the map is valid, otherwise Bars × 384.
func (s *Sequence) TotalTicks() int {
	if len(s.BarStarts) == s.Bars+1 && s.Bars > 0 {
		return int(s.BarStarts[s.Bars])
	}
	return s.Bars * TicksPerBar
}

// BarLength returns the length in ticks of the 1-indexed bar.
func (s *Sequence) BarLength(bar int) int {
	if len(s.BarStarts) == s.Bars+1 && bar >= 1 && bar <= s.Bars {
		return int(s.BarStarts[bar] - s.BarStarts[bar-1])
	}
	return TicksPerBar
}

// MeterChanges lists every bar (from bar 1) whose length differs from the
// previous bar's; the first entry is always bar 1.
func (s *Sequence) MeterChanges() []MeterChange {
	var out []MeterChange
	prev := -1
	for bar := 1; bar <= max(s.Bars, 1); bar++ {
		if l := s.BarLength(bar); l != prev {
			out = append(out, MeterChange{Bar: bar, Ticks: l})
			prev = l
		}
	}
	return out
}

// TimeSignatureForBarTicks names a bar length as one of the editor's time
// signatures ("4_4", "3_4", "2_4", "5_4", "7_8"); "" if none matches.
// 288 ticks is reported as 3/4, although 6/8 has the same length.
func TimeSignatureForBarTicks(ticks int) string {
	switch ticks {
	case 384:
		return "4_4"
	case 288:
		return "3_4"
	case 192:
		return "2_4"
	case 480:
		return "5_4"
	case 336:
		return "7_8"
	}
	return ""
}
