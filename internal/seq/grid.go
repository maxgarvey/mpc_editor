package seq

import (
	"fmt"
	"sort"
)

// StepCell represents one cell in the step grid.
type StepCell struct {
	Active     bool
	Note       byte
	NoteName   string
	Velocity   byte
	Duration   uint16
	Bar        int // 1-indexed bar this cell belongs to
	StepInBar  int // 0-indexed step within the bar (0–15)
	GlobalStep int // = (Bar-1)*StepsPerBar + StepInBar
}

// PadRow is one pad's activity across all bars.
type PadRow struct {
	PadIndex   int
	PadLabel   string // e.g. "A1", "B3"
	SampleName string // first non-empty layer sample name from the loaded program
	Steps      []StepCell
}

// TrackRow is one row in the step grid (one track).
type TrackRow struct {
	TrackIndex int
	TrackName  string
	Steps      []StepCell
	PadRows    []PadRow // per-pad breakdown, only populated when track has multiple notes
}

// StepGrid is the visualization data for all bars of a sequence.
type StepGrid struct {
	TotalBars        int
	TotalSteps       int // = TotalBars * StepsPerBar
	BPM              float64
	Rows             []TrackRow // track-level rows (kept for compatibility)
	BankAPadRows     [16]PadRow // one row per Bank A pad (pads 0-15), always populated
	ExtraBankPadRows [48]PadRow // Banks B/C/D (pads 16-63), populated on demand
}

// PadLabel returns the display label for a pad index (e.g. 0→"A1", 16→"B1").
func PadLabel(padIndex int) string {
	bank := rune('A' + padIndex/16)
	num := padIndex%16 + 1
	return fmt.Sprintf("%c%d", bank, num)
}

// BuildGrid constructs a step grid spanning all bars of s.
// noteToPad maps MIDI note number → pad index for the loaded program;
// pass nil to use the chromatic fallback (note - 35).
func BuildGrid(s *Sequence, noteToPad map[int]int) *StepGrid {
	totalSteps := s.Bars * StepsPerBar

	grid := &StepGrid{
		TotalBars:  s.Bars,
		TotalSteps: totalSteps,
		BPM:        s.BPM,
	}

	padForNote := func(note byte) int {
		if noteToPad != nil {
			if idx, ok := noteToPad[int(note)]; ok {
				return idx
			}
		}
		// Chromatic fallback: MPC default assigns note 35=A1, 36=A2, ... 98=D16.
		if idx := int(note) - 35; idx >= 0 && idx < 64 {
			return idx
		}
		return 0
	}

	type cell struct {
		note     byte
		velocity byte
		duration uint16
	}

	// padGlobalSteps: padIndex → globalStep → loudest cell
	padGlobalSteps := make([]map[int]*cell, 64)
	for i := range padGlobalSteps {
		padGlobalSteps[i] = make(map[int]*cell)
	}

	// trackGlobalSteps: track → globalStep → loudest cell
	trackGlobalSteps := make(map[int]map[int]*cell)

	// trackNoteGlobalSteps: track → note → globalStep → loudest cell
	trackNoteGlobalSteps := make(map[int]map[byte]map[int]*cell)

	barTicks := uint32(s.Bars * TicksPerBar)

	for _, ev := range s.Events {
		if ev.Type != EventNoteOn {
			continue
		}
		if ev.Tick >= barTicks {
			continue
		}
		barIdx := int(ev.Tick) / TicksPerBar   // 0-indexed
		stepInBar := int(ev.Tick%uint32(TicksPerBar)) / TicksPerStep
		if stepInBar >= StepsPerBar {
			continue
		}
		gs := barIdx*StepsPerBar + stepInBar // globalStep

		padIdx := padForNote(ev.Note)
		if padIdx >= 0 && padIdx < 64 {
			if padGlobalSteps[padIdx][gs] == nil || ev.Velocity > padGlobalSteps[padIdx][gs].velocity {
				padGlobalSteps[padIdx][gs] = &cell{note: ev.Note, velocity: ev.Velocity, duration: ev.Duration}
			}
		}

		if trackGlobalSteps[ev.Track] == nil {
			trackGlobalSteps[ev.Track] = make(map[int]*cell)
		}
		if trackGlobalSteps[ev.Track][gs] == nil || ev.Velocity > trackGlobalSteps[ev.Track][gs].velocity {
			trackGlobalSteps[ev.Track][gs] = &cell{note: ev.Note, velocity: ev.Velocity, duration: ev.Duration}
		}

		if trackNoteGlobalSteps[ev.Track] == nil {
			trackNoteGlobalSteps[ev.Track] = make(map[byte]map[int]*cell)
		}
		if trackNoteGlobalSteps[ev.Track][ev.Note] == nil {
			trackNoteGlobalSteps[ev.Track][ev.Note] = make(map[int]*cell)
		}
		ns := trackNoteGlobalSteps[ev.Track][ev.Note]
		if ns[gs] == nil || ev.Velocity > ns[gs].velocity {
			ns[gs] = &cell{note: ev.Note, velocity: ev.Velocity, duration: ev.Duration}
		}
	}

	// makeSteps builds a full []StepCell for a given pad or track.
	makeSteps := func(globalCells map[int]*cell) []StepCell {
		steps := make([]StepCell, totalSteps)
		for gs := range totalSteps {
			bar := gs/StepsPerBar + 1
			sib := gs % StepsPerBar
			steps[gs] = StepCell{Bar: bar, StepInBar: sib, GlobalStep: gs}
			if c := globalCells[gs]; c != nil {
				steps[gs].Active = true
				steps[gs].Note = c.note
				steps[gs].NoteName = NoteName(c.note)
				steps[gs].Velocity = c.velocity
				steps[gs].Duration = c.duration
			}
		}
		return steps
	}

	// Build Bank A pad rows (always 16 entries, padIndex 0-15).
	for padIdx := range 16 {
		grid.BankAPadRows[padIdx] = PadRow{
			PadIndex: padIdx,
			PadLabel: PadLabel(padIdx),
			Steps:    makeSteps(padGlobalSteps[padIdx]),
		}
	}

	// Build Banks B/C/D pad rows (48 entries, padIndex 16-63).
	for i := range 48 {
		padIdx := i + 16
		grid.ExtraBankPadRows[i] = PadRow{
			PadIndex: padIdx,
			PadLabel: PadLabel(padIdx),
			Steps:    makeSteps(padGlobalSteps[padIdx]),
		}
	}

	// Build rows for tracks that have events, sorted by track index.
	for i := range trackCount {
		cells, ok := trackGlobalSteps[i]
		if !ok {
			continue
		}
		row := TrackRow{
			TrackIndex: i,
			TrackName:  s.Tracks[i].Name,
			Steps:      makeSteps(cells),
		}
		if row.TrackName == "" {
			row.TrackName = trackDefaultName(i)
		}

		// Build per-pad sub-rows, sorted by note.
		noteMap := trackNoteGlobalSteps[i]
		notes := make([]int, 0, len(noteMap))
		for n := range noteMap {
			notes = append(notes, int(n))
		}
		sort.Ints(notes)
		for _, n := range notes {
			padIdx := padForNote(byte(n))
			row.PadRows = append(row.PadRows, PadRow{
				PadIndex: padIdx,
				PadLabel: PadLabel(padIdx),
				Steps:    makeSteps(noteMap[byte(n)]),
			})
		}

		grid.Rows = append(grid.Rows, row)
	}

	return grid
}

func trackDefaultName(idx int) string {
	return fmt.Sprintf("Track %02d", idx+1)
}
