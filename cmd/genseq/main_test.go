package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxgarvey/mpc_editor/internal/seq"
)

func probeMap(t *testing.T) map[string][]byte {
	t.Helper()
	m := map[string][]byte{}
	for _, p := range probes() {
		if _, dup := m[p.file]; dup {
			t.Fatalf("duplicate probe %s", p.file)
		}
		m[p.file] = p.data
	}
	return m
}

func clock(data []byte, n int) int {
	off := offClockMap + n*4
	return int(data[off]) | int(data[off+1])<<8 | int(data[off+2])<<16
}

// Every probe must parse, play P1_CTRL on track 1, and keep an 8.3 name.
func TestProbesAreValid(t *testing.T) {
	for name, data := range probeMap(t) {
		if base := strings.TrimSuffix(name, ".SEQ"); len(base) > 8 || base == name {
			t.Errorf("%s: not an 8.3 .SEQ name", name)
		}
		s, err := seq.Parse(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if s.Tracks[0].PGMName != probeProgram {
			t.Errorf("%s: track 1 program %q", name, s.Tracks[0].PGMName)
		}
		if len(s.Events) == 0 {
			t.Errorf("%s: no events", name)
		}
	}
}

// The checklist's expected results depend on these exact bytes.
func TestProbeBytes(t *testing.T) {
	m := probeMap(t)
	for _, c := range []struct {
		file     string
		hdr16    byte
		loopByte byte
	}{{"LP00.SEQ", 0, 0}, {"LP01.SEQ", 0, 1}, {"LP10.SEQ", 1, 0}, {"LP11.SEQ", 1, 1}} {
		if d := m[c.file]; d[offHdr16] != c.hdr16 || d[offLoop] != c.loopByte {
			t.Errorf("%s: 0x16=%d 0x17=%d", c.file, d[offHdr16], d[offLoop])
		}
	}
	if m["SN07.SEQ"][offSeqNumber] != 7 {
		t.Error("SN07: sequence number byte")
	}

	for _, f := range []string{"MZERO.SEQ", "MCONT.SEQ"} {
		s, _ := seq.Parse(m[f])
		if s.TotalTicks() != 1344 || s.BarLength(3) != 288 || len(s.MeterChanges()) != 2 {
			t.Errorf("%s: total=%d bar3=%d", f, s.TotalTicks(), s.BarLength(3))
		}
		// Downbeat clicks follow the map: A13 at ticks 0, 384, 768, 1056.
		var downbeats []uint32
		for _, e := range s.Events {
			if e.Note == noteBar {
				downbeats = append(downbeats, e.Tick)
			}
		}
		if len(downbeats) != 4 || downbeats[3] != 1056 {
			t.Errorf("%s: downbeats %v", f, downbeats)
		}
	}
	if clock(m["MZERO.SEQ"], 5) != 0 || clock(m["MCONT.SEQ"], 5) != 1344+384 {
		t.Errorf("entry 5: zero=%d cont=%d", clock(m["MZERO.SEQ"], 5), clock(m["MCONT.SEQ"], 5))
	}

	s, _ := seq.Parse(m["VTUNE.SEQ"])
	want := []int8{0, 10, 60, 120, 0, -10, -60, -120}
	for i, e := range s.Events {
		if e.VariationType != 0 || e.VariationValue != want[i] || e.Tick != uint32(i*96) {
			t.Errorf("VTUNE event %d: %+v", i, e)
		}
	}

	s, _ = seq.Parse(m["VTYPES.SEQ"])
	for i, e := range s.Events {
		bar, beat := i/4, i%4
		wantType, wantVal := byte(bar), []int8{0, 10, 50, 100}[beat]
		if beat == 0 {
			wantType = 0
		}
		if e.Note != noteLong || e.VariationType != wantType || e.VariationValue != wantVal {
			t.Errorf("VTYPES event %d: %+v", i, e)
		}
	}

	countStatus := func(file string, status byte) (vals []byte) {
		s, _ := seq.Parse(m[file])
		for _, e := range s.Events {
			if e.Status == status {
				vals = append(vals, e.Note)
			}
		}
		return vals
	}
	if v := countStatus("X82.SEQ", 0x82); !bytes.Equal(v, []byte{1, 5, 10, 13}) {
		t.Errorf("X82 values %v", v)
	}
	if v := countStatus("PCHG.SEQ", 0xC0); !bytes.Equal(v, []byte{0, 4}) {
		t.Errorf("PCHG values %v", v)
	}

	ff := m["FFTAIL.SEQ"]
	for i := range 4 {
		tail := ff[offEvents+i*eventSize+12 : offEvents+i*eventSize+16]
		wantFF := i == 1 || i == 3
		if bytes.Equal(tail, []byte{0xff, 0xff, 0xff, 0xff}) != wantFF {
			t.Errorf("FFTAIL event %d tail % x", i, tail)
		}
	}

	rb := m["RBASE.SEQ"]
	if string(rb[offRecFD0:offRecFD0+8]) != "(unused)" || rb[offTracks+38] != 1 {
		t.Error("RBASE: 0xFD0 record or track byte 38 missing")
	}
	if v := countStatus("RBASE.SEQ", 0xC0); len(v) != 1 {
		t.Error("RBASE: program change missing")
	}
}

func TestWriteProbes(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if code := run([]string{"-probe", dir}, &out, &out); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.SEQ"))
	if len(files) != len(probes()) {
		t.Fatalf("%d files, want %d", len(files), len(probes()))
	}
	if !strings.Contains(out.String(), "genpgm") {
		t.Error("output should point at genpgm")
	}
}

func TestDescribeOffset(t *testing.T) {
	for off, want := range map[int]string{
		0x15:                  "header: sequence number",
		0x17:                  "header: loop flag",
		0x1C:                  "header: bars",
		0x30 + 4*10:           "clock map entry 10 (start of bar 11) byte 0",
		0xFD0 + 3:             "record at 0xFD0 +3",
		0x1000 + 48 + 38:      "track 2 program change number",
		0x1000 + 34:           "track 1 MIDI channel",
		0x1C05:                "event separator",
		offEvents + 16*2 + 11: "event 2 variation value",
		offEvents + 13:        "event 0 byte 13 (pad index)",
		offEvents + 16*3 + 9:  "event 3 duration",
	} {
		if got := describeOffset(off); got != want {
			t.Errorf("0x%X: %q, want %q", off, got, want)
		}
	}
}

func TestDiff(t *testing.T) {
	dir := t.TempDir()
	m := probeMap(t)
	write := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, m[name], 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	var out bytes.Buffer
	if code := run([]string{"-diff", write("LP00.SEQ"), write("LP11.SEQ")}, &out, &out); code != 0 {
		t.Fatal(out.String())
	}
	if !strings.Contains(out.String(), "loop flag") || !strings.Contains(out.String(), "2 byte(s) differ") {
		t.Errorf("same-size diff:\n%s", out.String())
	}

	out.Reset()
	run([]string{"-diff", write("LP00.SEQ"), write("RBASE.SEQ")}, &out, &out)
	if !strings.Contains(out.String(), "sizes differ") || !strings.Contains(out.String(), "status 0xC0") {
		t.Errorf("different-size diff:\n%s", out.String())
	}
}

func TestRunUsage(t *testing.T) {
	var out bytes.Buffer
	for _, args := range [][]string{{"-diff", "a"}, {"extra"}, {"-bogus"}} {
		if code := run(args, &out, &out); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	if code := run([]string{"-diff", "/nonexistent/a", "/nonexistent/b"}, &out, &out); code != 1 {
		t.Errorf("missing files: exit %d, want 1", code)
	}
}
