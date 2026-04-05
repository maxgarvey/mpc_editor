package seq

import "fmt"

// StepCell represents one cell in the step grid.
type StepCell struct {
	Active   bool
	Note     byte
	NoteName string
	Velocity byte
}

// TrackRow is one row in the step grid (one track).
type TrackRow struct {
	TrackIndex int
	TrackName  string
	Steps      [StepsPerBar]StepCell
}

// StepGrid is the visualization data for one bar of a sequence.
type StepGrid struct {
	Bar       int
	TotalBars int
	BPM       float64
	Rows      []TrackRow
}

// BuildGrid constructs a step grid for the given bar (1-indexed).
func BuildGrid(s *Sequence, bar int) *StepGrid {
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

	// Collect events per track for this bar.
	type cell struct {
		note     byte
		velocity byte
	}
	trackSteps := make(map[int][StepsPerBar]*cell)

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

		steps, ok := trackSteps[ev.Track]
		if !ok {
			steps = [StepsPerBar]*cell{}
		}
		// If multiple notes on same step, keep the loudest.
		if steps[stepIndex] == nil || ev.Velocity > steps[stepIndex].velocity {
			steps[stepIndex] = &cell{note: ev.Note, velocity: ev.Velocity}
		}
		trackSteps[ev.Track] = steps
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
		grid.Rows = append(grid.Rows, row)
	}

	return grid
}

func trackDefaultName(idx int) string {
	return fmt.Sprintf("Track %02d", idx+1)
}
