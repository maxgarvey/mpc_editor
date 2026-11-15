// genpgm generates .PGM probe programs, matching WAV samples and .SEQ files
// for answering open questions about the MPC 1000 program format on real
// hardware.
//
//	go run ./cmd/genpgm OUTDIR              # write the probe set into OUTDIR
//	go run ./cmd/genpgm -diff A.PGM B.PGM   # list differing bytes by field name
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/pgm"
	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// Pads that carry a sample: bank A, pads 0–15.
const samplePads = 16

// Pad indices used by the probes (0-based; A1 = 0).
const (
	padA1 = 0
	padA2 = 1
	padA3 = 2
	padA6 = 5
)

// probe is one generated .PGM file and the question it answers.
type probe struct {
	file     string
	question string
	build    func() (*pgm.Program, error)
}

// toneName is the sample (and WAV file) name for the tone whose home pad is
// pad. Eight characters keeps it well inside the 16-char layer name field and
// 8.3-safe on the CF card.
func toneName(pad int) string {
	return fmt.Sprintf("TONE_A%02d", pad+1)
}

// baseProgram is the control layout: chromatic notes from 35 (A1 = 35, as in
// the hardware-made testdata/chromatic.pgm), bank A pads each playing their
// own tone, and both sliders set to the hardware's defaults (taken from
// chromatic.pgm). Other per-pad fields keep NewProgram's defaults, so this is
// an "editor-made" file for the re-save test.
func baseProgram() *pgm.Program {
	p := pgm.NewProgram()
	for i := range samplePads {
		_ = p.Pad(i).Layer(0).SetSampleName(toneName(i)) // 8 chars, cannot fail
	}
	for s := range 2 {
		sl := p.Slider(s)
		sl.SetPad(0)
		sl.SetParameter(s) // slider 1 = Tune, slider 2 = Filter, as in chromatic.pgm
		sl.SetRange(pgm.SliderTuneRange, pgm.Range{Low: -120, High: 120})
		sl.SetRange(pgm.SliderFilterRange, pgm.Range{Low: -50, High: 50})
		sl.SetRange(pgm.SliderLayerRange, pgm.Range{Low: 0, High: 127})
		sl.SetRange(pgm.SliderAttackRange, pgm.Range{Low: 0, High: 100})
		sl.SetRange(pgm.SliderDecayRange, pgm.Range{Low: 0, High: 100})
	}
	return p
}

func probes() []probe {
	return []probe{
		{
			file:     "P1_CTRL.PGM",
			question: "Control: consistent tables, A1=35 … D16=98. Baseline for every other probe; also the re-save test (Q6) and seq recording (Q7).",
			build:    func() (*pgm.Program, error) { return baseProgram(), nil },
		},
		{
			file: "P2_SWAP.PGM",
			question: "Q1: which table does the hardware trust? pad→note is the control layout (A2=36, A6=40) " +
				"but note→pad says 36→A6 and 40→A2.",
			build: func() (*pgm.Program, error) {
				p := baseProgram()
				if err := p.SetNoteMapEntry(36, padA6); err != nil {
					return nil, err
				}
				if err := p.SetNoteMapEntry(40, padA2); err != nil {
					return nil, err
				}
				return p, nil
			},
		},
		{
			file: "P3_DUPE.PGM",
			question: "Q2: A2 and A3 both on note 36; note→pad says 36→A3 (not the lowest pad); note 37 has no pad. " +
				"Which pad does MIDI note 36 play, and does saving rewrite the tables?",
			build: func() (*pgm.Program, error) {
				p := baseProgram()
				p.Pad(padA3).SetMIDINote(36) // sync points 36→A3 and clears 37
				return p, nil
			},
		},
		{
			file:     "P4_PAD0.PGM",
			question: "Q3: the undocumented pad byte +0x65 set to 0 on all 64 pads (hardware files always have 1).",
			build: func() (*pgm.Program, error) {
				p := baseProgram()
				for i := range p.PadCount() {
					p.Pad(i).SetUnknownConstByte(0)
				}
				return p, nil
			},
		},
		{
			file:     "P5_SLD0.PGM",
			question: "Q3: the undocumented slider byte +0x01 set to 0 on both sliders (hardware files always have 1).",
			build: func() (*pgm.Program, error) {
				p := baseProgram()
				p.Slider(0).SetUnknownConstByte(0)
				p.Slider(1).SetUnknownConstByte(0)
				return p, nil
			},
		},
		{
			file: "P6_REMAP.PGM",
			question: "Q7: consistent but non-chromatic-from-35 layout, A1=60 … D16=123. " +
				"Does a recorded .SEQ store the pad index or the note in event byte 13?",
			build: func() (*pgm.Program, error) {
				p := baseProgram()
				// Assign from the top down so no intermediate state collides.
				for i := p.PadCount() - 1; i >= 0; i-- {
					p.Pad(i).SetMIDINote(60 + i)
				}
				return p, nil
			},
		},
	}
}

// seqFile is one generated .SEQ file.
type seqFile struct {
	file    string
	program string // program name written into track 1
	data    []byte
}

// seqFiles builds the playback probes: note 36 on beat 1 and note 40 on
// beat 3, one bar at 90 BPM, looping. seq.Create writes event byte 13 as
// note-36 (0 and 4, i.e. A1 and A5), which no table maps to these notes, so
// a third answer ("the sequencer uses byte 13") is also distinguishable.
func seqFiles() []seqFile {
	events := []seq.Event{
		{Tick: 0, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 110, Duration: 48},
		{Tick: 192, Track: 0, Type: seq.EventNoteOn, Note: 40, Velocity: 110, Duration: 48},
	}
	var out []seqFile
	for _, f := range []struct{ file, program string }{
		{"S1_CTRL.SEQ", "P1_CTRL"},
		{"S2_SWAP.SEQ", "P2_SWAP"},
		{"S3_DUPE.SEQ", "P3_DUPE"},
	} {
		out = append(out, seqFile{f.file, f.program, seq.Create(90, 1, "Track01", f.program, true, events)})
	}
	return out
}

// Tone parameters. Pad n in bank A (0-based) sits in row n/4 (bottom row =
// A1–A4) and column n%4. The tone's pitch encodes the row and the number of
// beeps encodes the column, so "2 beeps, high" is A10 without a lookup.
const (
	sampleRate = 44100
	beepFrames = 4000 // ~91 ms of tone per beep
	gapFrames  = 3000 // ~68 ms of silence after each beep
	fadeFrames = 220  // ~5 ms fade in/out to avoid clicks
	amplitude  = 0.5
)

var rowFreqs = [4]float64{220, 440, 880, 1760}

// beepCount and rowPitch describe the tone of a bank-A pad.
func beepCount(pad int) int      { return pad%4 + 1 }
func rowPitch(pad int) float64   { return rowFreqs[pad/4] }
func rowLabel(pad int) string    { return [4]string{"low", "mid", "high", "very high"}[pad/4] }
func toneSummary(pad int) string { return fmt.Sprintf("%d beep(s), %s", beepCount(pad), rowLabel(pad)) }

// tone renders the identifying sample for a bank-A pad as 16-bit 44.1 kHz mono.
func tone(pad int) *audio.Sample {
	n := beepCount(pad)
	frames := n * (beepFrames + gapFrames)
	data := make([]byte, frames*2)
	freq := rowPitch(pad)
	for b := range n {
		start := b * (beepFrames + gapFrames)
		for i := range beepFrames {
			env := 1.0
			if i < fadeFrames {
				env = float64(i) / fadeFrames
			} else if i >= beepFrames-fadeFrames {
				env = float64(beepFrames-1-i) / fadeFrames
			}
			v := amplitude * env * math.Sin(2*math.Pi*freq*float64(i)/sampleRate)
			binary.LittleEndian.PutUint16(data[(start+i)*2:], uint16(int16(v*math.MaxInt16)))
		}
	}
	return &audio.Sample{
		Format:      audio.Format{SampleRate: sampleRate, Channels: 1, BitsPerSample: 16},
		FrameLength: frames,
		Data:        data,
	}
}

// writeSet writes all probe programs, tones and sequences into dir.
func writeSet(dir string, w io.Writer) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, pr := range probes() {
		p, err := pr.build()
		if err != nil {
			return fmt.Errorf("%s: %w", pr.file, err)
		}
		if err := p.Save(filepath.Join(dir, pr.file)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "%-14s %s\n", pr.file, pr.question)
	}
	for i := range samplePads {
		name := toneName(i) + ".WAV"
		if err := tone(i).SaveWAV(filepath.Join(dir, name)); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "%-14s pad %s: %s\n", name, pgm.PadName(i), toneSummary(i))
	}
	for _, s := range seqFiles() {
		if err := os.WriteFile(filepath.Join(dir, s.file), s.data, 0o644); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "%-14s note 36 beat 1, note 40 beat 3, for %s\n", s.file, s.program)
	}
	return nil
}

// diffFiles prints every differing byte between two .pgm files with its
// field name. It reads raw bytes, so nothing is repaired before comparing.
func diffFiles(a, b string, w io.Writer) error {
	da, err := os.ReadFile(a)
	if err != nil {
		return err
	}
	db, err := os.ReadFile(b)
	if err != nil {
		return err
	}
	if len(da) != len(db) {
		_, _ = fmt.Fprintf(w, "sizes differ: %s=%d bytes, %s=%d bytes (comparing common prefix)\n", a, len(da), b, len(db))
	}
	diffs := pgm.DiffBytes(da, db)
	for _, d := range diffs {
		_, _ = fmt.Fprintf(w, "0x%04X  %3d -> %3d  (0x%02X -> 0x%02X)  %s\n", d.Offset, d.A, d.B, d.A, d.B, d.Field)
	}
	_, _ = fmt.Fprintf(w, "%d byte(s) differ\n", len(diffs))
	return nil
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("genpgm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	diff := fs.Bool("diff", false, "compare two .pgm files: genpgm -diff A.PGM B.PGM")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: genpgm OUTDIR\n       genpgm -diff A.PGM B.PGM")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *diff {
		if fs.NArg() != 2 {
			fs.Usage()
			return 2
		}
		if err := diffFiles(fs.Arg(0), fs.Arg(1), stdout); err != nil {
			_, _ = fmt.Fprintf(stderr, "genpgm: %v\n", err)
			return 1
		}
		return 0
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	if err := writeSet(fs.Arg(0), stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "genpgm: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
