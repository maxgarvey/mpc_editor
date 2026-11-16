package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// Hardware probes for docs/seq-hardware-tests.md. Every probe plays program
// P1_CTRL from cmd/genpgm (pad n of bank A = note 35+n, each pad a tone that
// identifies it), so write both generators into the same folder.
//
// Byte-level layout: docs/seq-format.md.

const (
	probeProgram = "P1_CTRL"
	probeBPM     = 90

	noteClick = 35 // pad A1: 1 low beep
	noteLong  = 38 // pad A4: 4 low beeps, long enough to hear decay/attack/filter
	noteBar   = 47 // pad A13: 1 very high beep, marks bar downbeats

	offSeqNumber = 0x15
	offHdr16     = 0x16
	offLoop      = 0x17
	offClockMap  = 0x30
	offRecFD0    = 0x0FD0
	offTracks    = 0x1000
	offEvents    = 0x1C10
	trackSize    = 48
	eventSize    = 16
)

type probeFile struct {
	file     string
	question string
	data     []byte
}

func note(tick uint32, n byte) seq.Event {
	return seq.Event{Tick: tick, Type: seq.EventNoteOn, Note: n, Velocity: 110, Duration: 40}
}

// clicks plays A13 on each bar's downbeat and A1 on the other beats.
func clicks(barLengths []int) []seq.Event {
	var evs []seq.Event
	start := uint32(0)
	for _, l := range barLengths {
		evs = append(evs, note(start, noteBar))
		for b := 1; b < l/seq.PPQN; b++ {
			evs = append(evs, note(start+uint32(b*seq.PPQN), noteClick))
		}
		start += uint32(l)
	}
	return evs
}

func create(bars int, events []seq.Event) []byte {
	return seq.Create(probeBPM, bars, "Track01", probeProgram, true, events)
}

// setClockMap writes bar start ticks for barLengths; entries after the end
// tick are zero-filled (zeroFill) or keep counting in 4/4.
func setClockMap(data []byte, barLengths []int, zeroFill bool) {
	tick := uint32(0)
	for n := range 1000 {
		off := offClockMap + n*4
		switch {
		case n <= len(barLengths):
			if n > 0 {
				tick += uint32(barLengths[n-1])
			}
		case zeroFill:
			binary.LittleEndian.PutUint32(data[off:], 0)
			continue
		default:
			tick += seq.TicksPerBar
		}
		data[off], data[off+1], data[off+2], data[off+3] = byte(tick), byte(tick>>8), byte(tick>>16), 0x60
	}
}

// setTail overwrites bytes 12-15 of event i.
func setTail(data []byte, i int, tail [4]byte) {
	copy(data[offEvents+i*eventSize+12:], tail[:])
}

func variation(tick uint32, n, typ byte, val int8) seq.Event {
	e := note(tick, n)
	e.VariationType, e.VariationValue = typ, val
	return e
}

func probes() []probeFile {
	var out []probeFile
	add := func(file, q string, data []byte) { out = append(out, probeFile{file, q, data}) }

	// Loop flag: which of header bytes 0x16 / 0x17 controls Loop?
	for _, b := range [][2]byte{{0, 0}, {0, 1}, {1, 0}, {1, 1}} {
		d := create(1, clicks([]int{384}))
		d[offHdr16], d[offLoop] = b[0], b[1]
		add(fmt.Sprintf("LP%d%d.SEQ", b[0], b[1]),
			fmt.Sprintf("SQ1 loop: header 0x16=%d, 0x17=%d", b[0], b[1]), d)
	}

	// Sequence number byte.
	d := create(1, clicks([]int{384}))
	d[offSeqNumber] = 7
	add("SN07.SEQ", "SQ2 header 0x15 (sequence number) = 7", d)

	// Clock map: 4/4, 4/4, 3/4, 3/4 in both conventions.
	meter := []int{384, 384, 288, 288}
	for _, zf := range []bool{true, false} {
		d := create(len(meter), clicks(meter))
		setClockMap(d, meter, zf)
		name, how := "MCONT.SEQ", "entries after the end keep counting"
		if zf {
			name, how = "MZERO.SEQ", "entries after the end zeroed"
		}
		add(name, "SQ3 bars 3-4 in 3/4 via the clock map ("+how+")", d)
	}

	// Note Variation, type 0 (Tune): A1 on every beat over two bars.
	var tune []seq.Event
	for i, v := range []int8{0, 10, 60, 120, 0, -10, -60, -120} {
		tune = append(tune, variation(uint32(i*seq.PPQN), noteClick, 0, v))
	}
	add("VTUNE.SEQ", "SQ4 tune variation 0,+10,+60,+120,0,-10,-60,-120 on A1", create(2, tune))

	// Note Variation types 0-3: one bar each on A4: none, 10, 50, 100.
	var types []seq.Event
	for t := range 4 {
		bar := uint32(t * seq.TicksPerBar)
		types = append(types, note(bar, noteLong))
		for i, v := range []int8{10, 50, 100} {
			types = append(types, variation(bar+uint32((i+1)*seq.PPQN), noteLong, byte(t), v))
		}
	}
	add("VTYPES.SEQ", "SQ4 variation types 0-3 (bar 1-4) on A4: none, 10, 50, 100", create(4, types))

	// Unknown 0x82 records at each bar start.
	x82 := clicks([]int{384, 384, 384, 384})
	for i, v := range []byte{1, 5, 10, 13} {
		x82 = append(x82, seq.Event{Tick: uint32(i * seq.TicksPerBar), Type: 0x80, Status: 0x82, Note: v})
	}
	add("X82.SEQ", "SQ5 0x82 records with values 1, 5, 10, 13 at bars 1-4", create(4, x82))

	// Program change records.
	pc := clicks([]int{384, 384})
	pc = append(pc,
		seq.Event{Tick: 0, Type: seq.EventProgramChange, Note: 0},
		seq.Event{Tick: seq.TicksPerBar, Type: seq.EventProgramChange, Note: 4})
	add("PCHG.SEQ", "SQ6 program change 1 at bar 1, 5 at bar 2", create(2, pc))

	// ff ff ff ff tails on beats 2 and 4.
	d = create(1, clicks([]int{384}))
	setTail(d, 1, [4]byte{0xff, 0xff, 0xff, 0xff})
	setTail(d, 3, [4]byte{0xff, 0xff, 0xff, 0xff})
	add("FFTAIL.SEQ", "SQ8 bytes 12-15 = ff ff ff ff on beats 2 and 4", d)

	// Everything at once, for a load-and-resave diff.
	all := []seq.Event{
		{Tick: 0, Type: seq.EventProgramChange, Note: 0},
		{Tick: 0, Type: 0x80, Status: 0x82, Note: 1},
		note(0, noteBar),
		variation(96, noteClick, 0, 60),
		variation(192, noteLong, 1, 50),
		note(288, noteClick),
	}
	d = create(1, all)
	setTail(d, 5, [4]byte{0xff, 0xff, 0xff, 0xff})
	copy(d[offRecFD0:], "(unused)")
	copy(d[offRecFD0+17:], "OFF")
	d[offRecFD0+34] = 1
	d[offTracks+38] = 1
	add("RBASE.SEQ", "SQ9 every record kind; load and re-save as RB_HW", d)

	return out
}

func writeProbes(dir string, w io.Writer) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, p := range probes() {
		if err := os.WriteFile(filepath.Join(dir, p.file), p.data, 0o644); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "%-12s %s\n", p.file, p.question)
	}
	_, _ = fmt.Fprintf(w, "All probes play %s; run `go run ./cmd/genpgm %s` too.\n", probeProgram, dir)
	return nil
}
