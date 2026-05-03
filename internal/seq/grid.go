package seq

import (
	"fmt"
	"sort"
)

// GridParams defines the time signature and step division for building the display grid.
type GridParams struct {
	BeatsPerBar  int
	BeatDenom    int
	TicksPerStep int
	TicksPerBar  int // BeatsPerBar * PPQN * 4 / BeatDenom
	StepsPerBar  int // TicksPerBar / TicksPerStep
	StepsPerBeat int // TicksPerBar / (BeatsPerBar * TicksPerStep)
}

// DefaultGridParams returns 4/4 time with 16th-note steps.
func DefaultGridParams() GridParams { return NewGridParams(4, 4, TicksPerStep) }

// NewGridParams constructs a GridParams for the given time signature and step division.
func NewGridParams(beatsPerBar, beatDenom, ticksPerStep int) GridParams {
	if beatsPerBar < 1 {
		beatsPerBar = 4
	}
	if beatDenom < 1 {
		beatDenom = 4
	}
	if ticksPerStep < 1 {
		ticksPerStep = TicksPerStep
	}
	ticksPerBeat := PPQN * 4 / beatDenom
	tpb := beatsPerBar * ticksPerBeat
	spb := tpb / ticksPerStep
	if spb < 1 {
		spb = 1
	}
	spBeat := ticksPerBeat / ticksPerStep
	if spBeat < 1 {
		spBeat = 1
	}
	return GridParams{
		BeatsPerBar:  beatsPerBar,
		BeatDenom:    beatDenom,
		TicksPerStep: ticksPerStep,
		TicksPerBar:  tpb,
		StepsPerBar:  spb,
		StepsPerBeat: spBeat,
	}
}

// StepCell represents one cell in the step grid.
type StepCell struct {
	Active     bool
	Note       byte
	NoteName   string
	Velocity   byte
	Duration   uint16
	Tick       uint32 // raw event tick (may be off-grid); zero for inactive cells
	Bar        int    // 1-indexed bar this cell belongs to
	StepInBar  int    // 0-indexed step within the bar (0–15)
	GlobalStep int    // = (Bar-1)*StepsPerBar + StepInBar
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
	TotalSteps       int // = TotalBars * Params.StepsPerBar
	BPM              float64
	Params           GridParams
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
// p controls the time signature and step division; use DefaultGridParams() for standard 4/4 16th-note grids.
func BuildGrid(s *Sequence, noteToPad map[int]int, p GridParams) *StepGrid {
	// Compute display bars: how many p-bars fit in the file's total ticks.
	fileTicks := s.Bars * TicksPerBar
	displayBars := fileTicks / p.TicksPerBar
	if fileTicks%p.TicksPerBar != 0 {
		displayBars++
	}
	if displayBars < 1 {
		displayBars = 1
	}
	totalSteps := displayBars * p.StepsPerBar

	grid := &StepGrid{
		TotalBars:  displayBars,
		TotalSteps: totalSteps,
		BPM:        s.BPM,
		Params:     p,
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
		tick     uint32
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

	barTicks := uint32(displayBars * p.TicksPerBar)

	for _, ev := range s.Events {
		if ev.Type != EventNoteOn {
			continue
		}
		if ev.Tick >= barTicks {
			continue
		}
		barIdx := int(ev.Tick) / p.TicksPerBar // 0-indexed
		stepInBar := int(ev.Tick%uint32(p.TicksPerBar)) / p.TicksPerStep
		if stepInBar >= p.StepsPerBar {
			continue
		}
		gs := barIdx*p.StepsPerBar + stepInBar // globalStep

		padIdx := padForNote(ev.Note)
		if padIdx >= 0 && padIdx < 64 {
			if padGlobalSteps[padIdx][gs] == nil || ev.Velocity > padGlobalSteps[padIdx][gs].velocity {
				padGlobalSteps[padIdx][gs] = &cell{note: ev.Note, velocity: ev.Velocity, duration: ev.Duration, tick: ev.Tick}
			}
		}

		if trackGlobalSteps[ev.Track] == nil {
			trackGlobalSteps[ev.Track] = make(map[int]*cell)
		}
		if trackGlobalSteps[ev.Track][gs] == nil || ev.Velocity > trackGlobalSteps[ev.Track][gs].velocity {
			trackGlobalSteps[ev.Track][gs] = &cell{note: ev.Note, velocity: ev.Velocity, duration: ev.Duration, tick: ev.Tick}
		}

		if trackNoteGlobalSteps[ev.Track] == nil {
			trackNoteGlobalSteps[ev.Track] = make(map[byte]map[int]*cell)
		}
		if trackNoteGlobalSteps[ev.Track][ev.Note] == nil {
			trackNoteGlobalSteps[ev.Track][ev.Note] = make(map[int]*cell)
		}
		ns := trackNoteGlobalSteps[ev.Track][ev.Note]
		if ns[gs] == nil || ev.Velocity > ns[gs].velocity {
			ns[gs] = &cell{note: ev.Note, velocity: ev.Velocity, duration: ev.Duration, tick: ev.Tick}
		}
	}

	// makeSteps builds a full []StepCell for a given pad or track.
	makeSteps := func(globalCells map[int]*cell) []StepCell {
		steps := make([]StepCell, totalSteps)
		for gs := range totalSteps {
			bar := gs/p.StepsPerBar + 1
			sib := gs % p.StepsPerBar
			steps[gs] = StepCell{Bar: bar, StepInBar: sib, GlobalStep: gs}
			if c := globalCells[gs]; c != nil {
				steps[gs].Active = true
				steps[gs].Note = c.note
				steps[gs].NoteName = NoteName(c.note)
				steps[gs].Velocity = c.velocity
				steps[gs].Duration = c.duration
				steps[gs].Tick = c.tick
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
