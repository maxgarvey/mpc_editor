// genseq generates test .SEQ files for MPC 1000 hardware verification.
// Run from the project root:
//
//	go run ./cmd/genseq
//
// Files are written to testdata/seq/.
package main

import (
	"fmt"
	"os"

	"github.com/maxgarvey/mpc_editor/internal/seq"
)

func main() {
	outDir := "testdata/seq"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	files := []struct {
		name   string
		data   []byte
	}{
		// ── General behaviour ──────────────────────────────────────────────

		{
			"general_single_note.SEQ",
			seq.Create(120.0, 1, "Track01", "", []seq.Event{
				{Tick: 0, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
			}),
		},

		{
			"general_quarter_notes.SEQ",
			seq.Create(120.0, 1, "Track01", "", []seq.Event{
				{Tick: 0, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
				{Tick: 96, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
				{Tick: 192, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
				{Tick: 288, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
			}),
		},

		{
			"general_all_16_steps.SEQ",
			seq.Create(120.0, 1, "Track01", "", func() []seq.Event {
				evs := make([]seq.Event, 16)
				for i := range evs {
					evs[i] = seq.Event{
						Tick: uint32(i * seq.TicksPerStep), Track: 0,
						Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 12,
					}
				}
				return evs
			}()),
		},

		{
			"general_bank_a_pads_each_step.SEQ",
			// A1 on step 1, A2 on step 2, …, A16 on step 16.
			seq.Create(120.0, 1, "Track01", "", func() []seq.Event {
				evs := make([]seq.Event, 16)
				for i := range evs {
					evs[i] = seq.Event{
						Tick: uint32(i * seq.TicksPerStep), Track: 0,
						Type:     seq.EventNoteOn,
						Note:     byte(36 + i), // A1=36 … A16=51
						Velocity: 100, Duration: 12,
					}
				}
				return evs
			}()),
		},

		{
			"general_two_bars.SEQ",
			// Bar 1: A1 on every beat. Bar 2: A5 on every beat.
			seq.Create(120.0, 2, "Track01", "", func() []seq.Event {
				evs := []seq.Event{}
				for beat := 0; beat < 4; beat++ {
					evs = append(evs, seq.Event{
						Tick: uint32(beat * 96), Track: 0,
						Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23,
					})
				}
				for beat := 0; beat < 4; beat++ {
					evs = append(evs, seq.Event{
						Tick: uint32(384 + beat*96), Track: 0,
						Type: seq.EventNoteOn, Note: 40, Velocity: 100, Duration: 23,
					})
				}
				return evs
			}()),
		},

		// ── Boundary conditions ────────────────────────────────────────────

		{
			"boundary_first_and_last_step.SEQ",
			// A1 on step 1 (tick 0) and step 16 (tick 360) — the two edge steps.
			seq.Create(120.0, 1, "Track01", "", []seq.Event{
				{Tick: 0, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 12},
				{Tick: 360, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 12},
			}),
		},

		{
			"boundary_all_bank_a_simultaneous.SEQ",
			// All 16 Bank A pads (A1–A16, notes 36–51) at tick 0 simultaneously.
			seq.Create(120.0, 1, "Track01", "", func() []seq.Event {
				evs := make([]seq.Event, 16)
				for i := range evs {
					evs[i] = seq.Event{
						Tick: 0, Track: 0,
						Type:     seq.EventNoteOn,
						Note:     byte(36 + i),
						Velocity: 100, Duration: 12,
					}
				}
				return evs
			}()),
		},

		{
			"boundary_velocity_range.SEQ",
			// A1 at five steps with velocities 1, 32, 64, 96, 127 (full dynamic range).
			seq.Create(120.0, 1, "Track01", "", []seq.Event{
				{Tick: 0, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 1, Duration: 23},
				{Tick: 72, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 32, Duration: 23},
				{Tick: 144, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 64, Duration: 23},
				{Tick: 216, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 96, Duration: 23},
				{Tick: 288, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 127, Duration: 23},
			}),
		},

		{
			"boundary_bpm_90.SEQ",
			// Same quarter-note pattern as general_quarter_notes but at 90 BPM — should play slower.
			seq.Create(90.0, 1, "Track01", "", []seq.Event{
				{Tick: 0, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
				{Tick: 96, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
				{Tick: 192, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
				{Tick: 288, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
			}),
		},

		{
			"boundary_four_bars.SEQ",
			// 4 bars, 120 BPM. A1 on beat 1 of every bar; A5 on beat 3 of every bar.
			seq.Create(120.0, 4, "Track01", "", func() []seq.Event {
				evs := []seq.Event{}
				for bar := 0; bar < 4; bar++ {
					barStart := uint32(bar * seq.TicksPerBar)
					evs = append(evs,
						seq.Event{Tick: barStart, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 23},
						seq.Event{Tick: barStart + 192, Track: 0, Type: seq.EventNoteOn, Note: 40, Velocity: 100, Duration: 23},
					)
				}
				return evs
			}()),
		},

		{
			"boundary_short_duration.SEQ",
			// A1 on every 16th step with duration=1 tick (shortest possible note).
			seq.Create(120.0, 1, "Track01", "", func() []seq.Event {
				evs := make([]seq.Event, 16)
				for i := range evs {
					evs[i] = seq.Event{
						Tick: uint32(i * seq.TicksPerStep), Track: 0,
						Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 1,
					}
				}
				return evs
			}()),
		},

		{
			"boundary_long_duration.SEQ",
			// A1 on beat 1 with duration=383 ticks (fills nearly the full bar).
			// A5 on beat 3 with duration=191 ticks (fills the remaining half bar).
			seq.Create(120.0, 1, "Track01", "", []seq.Event{
				{Tick: 0, Track: 0, Type: seq.EventNoteOn, Note: 36, Velocity: 100, Duration: 383},
				{Tick: 192, Track: 0, Type: seq.EventNoteOn, Note: 40, Velocity: 100, Duration: 191},
			}),
		},
	}

	for _, f := range files {
		path := outDir + "/" + f.name
		if err := os.WriteFile(path, f.data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
		s, _ := seq.Parse(f.data)
		fmt.Printf("%-45s  %5d bytes  bpm=%.0f bars=%d events=%d\n",
			f.name, len(f.data), s.BPM, s.Bars, len(s.Events))
	}
}
