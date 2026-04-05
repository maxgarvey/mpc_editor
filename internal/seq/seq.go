// Package seq parses MPC 1000/500 .SEQ sequence files.
package seq

import "strconv"

// Binary format constants.
const (
	PPQN         = 96                         // Pulses per quarter note
	TicksPerStep = 24                         // 96 / 4 = one sixteenth note
	StepsPerBar  = 16                         // 16 sixteenth notes per bar (4/4)
	TicksPerBar  = StepsPerBar * TicksPerStep // 384

	versionOffset = 0x04
	versionLen    = 16
	barsOffset    = 0x1C
	bpmOffset     = 0x20

	trackDataOffset = 0x0FD0
	trackChunkSize  = 48
	trackCount      = 64
	trackNameLen    = 16

	eventDataOffset = 0x1C10
	eventSize       = 8
)

// EventType identifies the kind of MIDI event.
type EventType byte

const (
	EventNoteOn        EventType = 0x00 // byte 4 in 0-127 range
	EventPolyPressure  EventType = 0xA0
	EventControlChange EventType = 0xB0
	EventProgramChange EventType = 0xC0
	EventChanPressure  EventType = 0xD0
	EventPitchBend     EventType = 0xE0
)

// Sequence represents a parsed .SEQ file.
type Sequence struct {
	Version string
	BPM     float64
	Bars    int
	Tracks  [64]Track
	Events  []Event
}

// Track holds metadata for one of the 64 sequence tracks.
type Track struct {
	Index       int
	Name        string
	MIDIChannel int
	Program     int
	Status      byte
}

// Event represents a single MIDI event in the sequence.
type Event struct {
	Tick     uint32
	Track    int
	Type     EventType
	Note     byte
	Velocity byte
	Duration uint16
	Data1    byte // CC number or bend low byte
	Data2    byte // CC value or bend high byte
}

// noteNames maps MIDI note number mod 12 to note name.
var noteNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// NoteName returns the musical note name for a MIDI note number (e.g. 60 -> "C4").
func NoteName(note byte) string {
	octave := int(note)/12 - 1
	name := noteNames[int(note)%12]
	return name + strconv.Itoa(octave)
}
