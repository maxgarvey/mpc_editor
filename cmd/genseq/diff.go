package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/maxgarvey/mpc_editor/internal/seq"
)

var headerFields = []struct {
	off, size int
	name      string
}{
	{0x00, 4, "file size"},
	{0x04, 16, "version / name"},
	{0x14, 1, "header 0x14 [?]"},
	{0x15, 1, "sequence number"},
	{0x16, 1, "header 0x16 [?]"},
	{0x17, 1, "loop flag"},
	{0x18, 1, "header 0x18 [?]"},
	{0x19, 1, "header 0x19 [?]"},
	{0x1A, 2, "clock map count? (1000)"},
	{0x1C, 2, "bars"},
	{0x1E, 2, "header 0x1E [?]"},
	{0x20, 2, "BPM x10"},
	{0x22, 14, "header tail [?]"},
}

var trackFields = []struct {
	off, size int
	name      string
}{
	{0, 16, "name"},
	{16, 16, "program name"},
	{32, 2, "[?] +32"},
	{34, 1, "MIDI channel"},
	{35, 1, "active [?]"},
	{36, 2, "[?] +36 (100 in .SEQ)"},
	{38, 1, "program change number"},
	{39, 1, "[?] +39"},
	{40, 2, "[?] +40 (30 in .SEQ)"},
	{42, 4, "[?] +42 (110 in .SEQ)"},
	{46, 1, "[?] +46"},
	{47, 1, "[?] +47 (50 in .SEQ)"},
}

var eventFields = []string{
	"tick", "tick", "tick", "tick", "track", "status", "note/data1", "velocity/data2",
	"duration", "duration", "variation type", "variation value", "byte 12 [?]", "byte 13 (pad index)", "byte 14 [?]", "byte 15 [?]",
}

// describeOffset names the field at an absolute offset in a .SEQ file.
func describeOffset(off int) string {
	switch {
	case off < 0x30:
		for _, f := range headerFields {
			if off >= f.off && off < f.off+f.size {
				return "header: " + f.name
			}
		}
	case off < offRecFD0:
		n := (off - offClockMap) / 4
		return fmt.Sprintf("clock map entry %d (start of bar %d) byte %d", n, n+1, (off-offClockMap)%4)
	case off < offTracks:
		return fmt.Sprintf("record at 0xFD0 +%d", off-offRecFD0)
	case off < 0x1C00:
		t, rel := (off-offTracks)/trackSize, (off-offTracks)%trackSize
		for _, f := range trackFields {
			if rel >= f.off && rel < f.off+f.size {
				return fmt.Sprintf("track %d %s", t+1, f.name)
			}
		}
	case off < offEvents:
		return "event separator"
	default:
		return fmt.Sprintf("event %d %s", (off-offEvents)/eventSize, eventFields[(off-offEvents)%eventSize])
	}
	return "unknown"
}

// diffFiles prints differing bytes between two .SEQ files by field name. When
// the event lists differ in length, the fixed part is diffed byte by byte and
// the events are listed decoded instead.
func diffFiles(a, b string, w io.Writer) error {
	da, err := os.ReadFile(a)
	if err != nil {
		return err
	}
	db, err := os.ReadFile(b)
	if err != nil {
		return err
	}
	if len(da) < offEvents || len(db) < offEvents {
		return fmt.Errorf("file too small to be a .SEQ")
	}
	limit := min(len(da), len(db))
	sameShape := len(da) == len(db)
	if !sameShape {
		limit = offEvents
		_, _ = fmt.Fprintf(w, "sizes differ (%d vs %d bytes): diffing the fixed part, then listing events\n", len(da), len(db))
	}
	n := 0
	for i := range limit {
		if da[i] != db[i] {
			_, _ = fmt.Fprintf(w, "0x%04X  %3d -> %3d  (0x%02X -> 0x%02X)  %s\n", i, da[i], db[i], da[i], db[i], describeOffset(i))
			n++
		}
	}
	_, _ = fmt.Fprintf(w, "%d byte(s) differ\n", n)
	if !sameShape {
		for _, f := range []struct {
			name string
			data []byte
		}{{a, da}, {b, db}} {
			_, _ = fmt.Fprintf(w, "== events in %s\n", f.name)
			listEvents(f.data, w)
		}
	}
	return nil
}

var terminator = []byte{0xff, 0xff, 0xff, 0x7f}

func listEvents(data []byte, w io.Writer) {
	for off := offEvents; off+eventSize <= len(data); off += eventSize {
		rec := data[off : off+eventSize]
		if bytes.Equal(rec[:4], terminator) {
			return
		}
		// Parse a copy holding just this record, so nothing aliases data.
		one := make([]byte, 0, offEvents+2*eventSize)
		one = append(one, data[:offEvents]...)
		one = append(one, rec...)
		one = append(one, terminator...)
		s, _ := seq.Parse(one)
		label := ""
		if s != nil && len(s.Events) == 1 {
			e := s.Events[0]
			label = fmt.Sprintf("tick %d track %d status 0x%02X note %d vel %d dur %d %s",
				e.Tick, e.Track+1, e.Status, e.Note, e.Velocity, e.Duration, seq.VariationLabel(e.VariationType, e.VariationValue))
		}
		_, _ = fmt.Fprintf(w, "  % x  %s\n", rec, label)
	}
}
