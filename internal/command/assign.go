package command

import (
	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// AssignMode determines how samples are assigned to pads.
type AssignMode int

const (
	AssignPerPad   AssignMode = iota // one sample per pad (layer 0)
	AssignPerLayer                   // fill all layers on each pad before moving to next
)

// SimpleAssign assigns imported samples to available pad slots starting from startPad.
// Returns the list of pad indices that were modified.
func SimpleAssign(prog *pgm.Program, matrix *pgm.SampleMatrix, samples []*pgm.SampleRef, startPad int, mode AssignMode) []int {
	var modified []int
	si := 0

	for i := startPad; i < prog.PadCount() && si < len(samples); i++ {
		pad := prog.Pad(i)
		for j := 0; j < 4 && si < len(samples); j++ {
			if matrix.Get(i, j) != nil {
				continue
			}
			sample := samples[si]
			matrix.Set(i, j, sample)
			_ = pad.Layer(j).SetSampleName(sample.Name)
			modified = append(modified, i)
			si++
			if mode == AssignPerPad {
				break // one sample per pad
			}
		}
	}
	return modified
}

// MultisampleAssign builds a chromatic multisample program from samples.
// Uses the MultisampleBuilder to assign notes with tuning across pads.
func MultisampleAssign(prog *pgm.Program, matrix *pgm.SampleMatrix, samples []*pgm.SampleRef) (modified []int, warnings []string) {
	builder := &pgm.MultisampleBuilder{}
	slots := builder.Assign(samples)
	if slots == nil {
		return nil, builder.Warnings
	}

	for i, slot := range slots {
		if slot == nil {
			continue
		}
		pad := prog.Pad(i)
		layer := pad.Layer(0)

		matrix.Set(i, 0, slot.Source)
		_ = layer.SetSampleName(slot.Source.Name)
		layer.SetTuning(slot.Tuning)
		pad.SetMIDINote(slot.Note)
		layer.SetPlayMode(0) // one-shot

		modified = append(modified, i)
	}
	warnings = builder.Warnings
	return
}
