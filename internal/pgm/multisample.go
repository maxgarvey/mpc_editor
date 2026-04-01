package pgm

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	firstNote = 35
	maxPads   = 64
)

var noteNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// With space variants for matching (like Java's NOTES_BIS)
var noteNamesBis = []string{"C ", "C#", "D ", "D#", "E ", "F ", "F#", "G ", "G#", "A ", "A#", "B "}

// Slot represents one sample in a multisample program, with its target note and tuning.
type Slot struct {
	Source *SampleRef // the sample reference
	Note   int        // MIDI note number
	Tuning float64    // tuning offset in semitones
}

// MultisampleBuilder finds a multisample configuration from a set of sample files.
type MultisampleBuilder struct {
	Warnings []string
}

// NoteName returns the note name for a MIDI note number (e.g., 60 → "C3").
func NoteName(note int) string {
	chromatic := (note - 24) % 12
	octave := (note - 24) / 12
	return fmt.Sprintf("%s%d", noteNames[chromatic], octave)
}

// ExtractNote extracts a MIDI note number from a string containing a note name.
// Returns -1 if no note found.
func ExtractNote(name string) int {
	// Search from highest note down (so C# matches before C)
	for i := len(noteNames) - 1; i >= 0; i-- {
		candidate := noteNamesBis[i]
		idx := strings.LastIndex(name, candidate)
		if idx == -1 {
			candidate = noteNames[i]
			idx = strings.LastIndex(name, candidate)
		}
		if idx != -1 {
			octaveIdx := idx + len(candidate)
			if octaveIdx < len(name) {
				ch := rune(name[octaveIdx])
				octave := 3 // default
				if unicode.IsDigit(ch) {
					octave = int(ch - '0')
				}
				return 24 + octave*12 + i
			}
		}
	}
	return -1
}

// Assign builds a multisample slot assignment from a list of sample references.
// Returns a 64-element slice where index i corresponds to note (firstNote + i).
// Nil entries mean no sample is assigned to that note.
func (b *MultisampleBuilder) Assign(samples []*SampleRef) []*Slot {
	b.Warnings = nil

	if len(samples) < 2 {
		return nil
	}

	// Extract sample names
	names := make([]string, len(samples))
	for i, s := range samples {
		names[i] = s.Name
	}

	// Find longest common prefix
	commonIdx := longestPrefix(names)
	if commonIdx == 0 {
		return nil
	}

	// Build slots from samples that have recognizable notes
	type indexedSlot struct {
		sample *SampleRef
		note   int
	}

	var slots []indexedSlot
	for _, s := range samples {
		variablePart := s.Name[commonIdx:]
		note := ExtractNote(variablePart)
		if note != -1 && note >= firstNote && note <= firstNote+maxPads {
			slots = append(slots, indexedSlot{sample: s, note: note})
		} else {
			b.Warnings = append(b.Warnings, fmt.Sprintf(
				"File: %s is not consistently named, will be ignored when building the multisamples", s.Name))
		}
	}

	if len(slots) <= 1 {
		return nil
	}

	// Sort by note
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].note < slots[j].note
	})

	// Build the 64-slot multisample array
	result := make([]*Slot, maxPads)

	var last *indexedSlot
	for idx := range slots {
		slot := &slots[idx]
		note := slot.note

		// Place exact note
		if note-firstNote >= 0 && note-firstNote < maxPads {
			result[note-firstNote] = &Slot{Source: slot.sample, Note: note, Tuning: 0}
		}

		// Cross note = halfway between last and current
		crossNote := firstNote - 1
		if last != nil {
			crossNote = (note + last.note) / 2
		}

		// Fill upward from last note to cross note
		if last != nil {
			for transposeUp := last.note + 1; transposeUp <= crossNote; transposeUp++ {
				idx := transposeUp - firstNote
				if idx >= 0 && idx < maxPads && result[idx] == nil {
					tuning := float64(transposeUp - last.note)
					if tuning >= -36 && tuning <= 36 {
						result[idx] = &Slot{Source: last.sample, Note: transposeUp, Tuning: tuning}
					}
				}
			}
		}

		// Fill downward from cross note to current note
		for transposeDown := crossNote + 1; transposeDown < note; transposeDown++ {
			idx := transposeDown - firstNote
			if idx >= 0 && idx < maxPads && result[idx] == nil {
				tuning := float64(transposeDown - note)
				if tuning >= -36 && tuning <= 36 {
					result[idx] = &Slot{Source: slot.sample, Note: transposeDown, Tuning: tuning}
				}
			}
		}

		last = slot
	}

	// Fill remaining notes above the last sample
	if last != nil {
		for transposeUp := last.note + 1; transposeUp < firstNote+maxPads; transposeUp++ {
			idx := transposeUp - firstNote
			if idx >= 0 && idx < maxPads && result[idx] == nil {
				tuning := float64(transposeUp - last.note)
				if tuning >= -36 && tuning <= 36 {
					result[idx] = &Slot{Source: last.sample, Note: transposeUp, Tuning: tuning}
				}
			}
		}
	}

	return result
}

func longestPrefix(words []string) int {
	if len(words) == 0 {
		return 0
	}
	commonIdx := 16 // max
	var last string
	for _, word := range words {
		if last != "" {
			idx := longestPrefixPair(commonIdx, word, last)
			if idx < commonIdx {
				commonIdx = idx
			}
		}
		last = word
	}
	return commonIdx
}

func longestPrefixPair(maxIdx int, a, b string) int {
	limit := maxIdx
	if len(a) < limit {
		limit = len(a)
	}
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return limit
}
