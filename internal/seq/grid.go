package seq

import (
	"fmt"
	"sort"
)

// StepCell represents one cell in the step grid.
type StepCell struct {
	Active   bool
	Note     byte
	NoteName string
	Velocity byte
}

// PadRow is one pad's activity within a track row.
type PadRow struct {
	PadIndex int
	PadLabel string // e.g. "A1", "B3"
	Steps    [StepsPerBar]StepCell
}

// TrackRow is one row in the step grid (one track).
type TrackRow struct {
	TrackIndex int
	TrackName  string
	Steps      [StepsPerBar]StepCell
	PadRows    []PadRow // per-pad breakdown, only populated when track has multiple notes
}

// StepGrid is the visualization data for one bar of a sequence.
type StepGrid struct {
	Bar          int
	TotalBars    int
	BPM          float64
	Rows         []TrackRow // track-level rows (kept for compatibility)
	BankAPadRows [16]PadRow // one row per Bank A pad (pads 0-15), always populated
}

// PadLabel returns the display label for a pad index (e.g. 0→"A1", 16→"B1").
func PadLabel(padIndex int) string {
	bank := rune('A' + padIndex/16)
	num := padIndex%16 + 1
	return fmt.Sprintf("%c%d", bank, num)
}

// BuildGrid constructs a step grid for the given bar (1-indexed).
// noteToPad maps MIDI note number → pad index for the loaded program;
// pass nil to use the chromatic fallback (note - 35).
func BuildGrid(s *Sequence, bar int, noteToPad map[int]int) *StepGrid {
	if bar < 1 {
		bar = 1
	}
	if bar > s.Bars {
		bar = s.Bars
	}

	grid := &StepGrid{
		Bar:       bar,
		TotalBars: s.Bars,
		BPM:       s.BPM,
	}

	// Tick range for this bar (0-indexed internally).
	barStart := uint32((bar - 1) * TicksPerBar)
	barEnd := barStart + TicksPerBar

	padForNote := func(note byte) int {
		if noteToPad != nil {
			if idx, ok := noteToPad[int(note)]; ok {
				return idx
			}
		}
		// MPC SEQ files store pad numbers 1-indexed (1=A1, 5=A5, 9=A9 etc).
		if idx := int(note) - 1; idx >= 0 && idx < 64 {
			return idx
		}
		return 0
	}

	// Collect events per track per note for this bar.
	type cell struct {
		note     byte
		velocity byte
	}
	// trackSteps: track → step → loudest cell (for track-level summary row)
	trackSteps := make(map[int][StepsPerBar]*cell)
	// trackNoteSteps: track → note → step → loudest cell (for pad sub-rows)
	trackNoteSteps := make(map[int]map[byte][StepsPerBar]*cell)
	// padSteps: padIndex (0-15) → step → loudest cell (for Bank A pad rows)
	var padSteps [16][StepsPerBar]*cell

	for _, ev := range s.Events {
		if ev.Type != EventNoteOn {
			continue
		}
		if ev.Tick < barStart || ev.Tick >= barEnd {
			continue
		}
		stepIndex := int(ev.Tick-barStart) / TicksPerStep
		if stepIndex < 0 || stepIndex >= StepsPerBar {
			continue
		}

		steps := trackSteps[ev.Track]
		if steps[stepIndex] == nil || ev.Velocity > steps[stepIndex].velocity {
			steps[stepIndex] = &cell{note: ev.Note, velocity: ev.Velocity}
		}
		trackSteps[ev.Track] = steps

		if trackNoteSteps[ev.Track] == nil {
			trackNoteSteps[ev.Track] = make(map[byte][StepsPerBar]*cell)
		}
		noteSteps := trackNoteSteps[ev.Track][ev.Note]
		if noteSteps[stepIndex] == nil || ev.Velocity > noteSteps[stepIndex].velocity {
			noteSteps[stepIndex] = &cell{note: ev.Note, velocity: ev.Velocity}
		}
		trackNoteSteps[ev.Track][ev.Note] = noteSteps

		if padIdx := padForNote(ev.Note); padIdx >= 0 && padIdx < 16 {
			if padSteps[padIdx][stepIndex] == nil || ev.Velocity > padSteps[padIdx][stepIndex].velocity {
				padSteps[padIdx][stepIndex] = &cell{note: ev.Note, velocity: ev.Velocity}
			}
		}
	}

	// Build Bank A pad rows (always 16 entries, padIndex 0-15).
	for padIdx := 0; padIdx < 16; padIdx++ {
		pr := PadRow{
			PadIndex: padIdx,
			PadLabel: PadLabel(padIdx),
		}
		for j := range StepsPerBar {
			if padSteps[padIdx][j] != nil {
				pr.Steps[j] = StepCell{
					Active:   true,
					Note:     padSteps[padIdx][j].note,
					NoteName: NoteName(padSteps[padIdx][j].note),
					Velocity: padSteps[padIdx][j].velocity,
				}
			}
		}
		grid.BankAPadRows[padIdx] = pr
	}

	// Build rows for tracks that have events, sorted by track index.
	for i := range trackCount {
		steps, ok := trackSteps[i]
		if !ok {
			continue
		}
		row := TrackRow{
			TrackIndex: i,
			TrackName:  s.Tracks[i].Name,
		}
		if row.TrackName == "" {
			row.TrackName = trackDefaultName(i)
		}
		for j := range StepsPerBar {
			if steps[j] != nil {
				row.Steps[j] = StepCell{
					Active:   true,
					Note:     steps[j].note,
					NoteName: NoteName(steps[j].note),
					Velocity: steps[j].velocity,
				}
			}
		}

		// Build per-pad sub-rows, sorted by pad index.
		noteMap := trackNoteSteps[i]
		notes := make([]int, 0, len(noteMap))
		for n := range noteMap {
			notes = append(notes, int(n))
		}
		sort.Ints(notes)
		for _, n := range notes {
			padIdx := padForNote(byte(n))
			pr := PadRow{
				PadIndex: padIdx,
				PadLabel: PadLabel(padIdx),
			}
			noteSteps := noteMap[byte(n)]
			for j := range StepsPerBar {
				if noteSteps[j] != nil {
					pr.Steps[j] = StepCell{
						Active:   true,
						Note:     noteSteps[j].note,
						NoteName: NoteName(noteSteps[j].note),
						Velocity: noteSteps[j].velocity,
					}
				}
			}
			row.PadRows = append(row.PadRows, pr)
		}

		grid.Rows = append(grid.Rows, row)
	}

	return grid
}

func trackDefaultName(idx int) string {
	return fmt.Sprintf("Track %02d", idx+1)
}
