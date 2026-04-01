package midi

const (
	DefaultPPQ      = 96
	DefaultVelocity = 127
	DefaultNoteLen  = 32 // ticks
	DefaultStartKey = 35
)

// Event represents a MIDI event at a specific tick.
type Event struct {
	Tick     int
	Status   byte
	Key      byte
	Velocity byte
}

// Sequence holds a MIDI sequence (Type 0, single track).
type Sequence struct {
	PPQ    int
	Events []Event
}

// NewSequence creates an empty MIDI sequence with the given PPQ.
func NewSequence(ppq int) *Sequence {
	return &Sequence{PPQ: ppq}
}

// AddNote adds a NOTE_ON and NOTE_OFF event pair.
func (s *Sequence) AddNote(startTick, tickLength int, key, velocity byte) {
	s.Events = append(s.Events,
		Event{Tick: startTick, Status: 0x90, Key: key, Velocity: velocity},
		Event{Tick: startTick + tickLength, Status: 0x80, Key: key, Velocity: velocity},
	)
}

// BuildFromMarkers creates a MIDI sequence from marker locations.
// tempo is in BPM, sampleRate in Hz, locations are frame positions.
func BuildFromMarkers(locations []int, tempo float64, sampleRate, ppq int) *Sequence {
	seq := NewSequence(ppq)
	tempoBPS := tempo / 60.0
	key := byte(DefaultStartKey)

	for _, loc := range locations {
		startTick := int(float64(ppq)*tempoBPS*float64(loc)/float64(sampleRate) + 0.5)
		seq.AddNote(startTick, DefaultNoteLen, key, DefaultVelocity)
		key++
	}
	return seq
}
