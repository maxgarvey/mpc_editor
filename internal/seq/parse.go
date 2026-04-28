package seq

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// Open reads and parses a .SEQ file from disk.
func Open(path string) (*Sequence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse decodes a .SEQ binary blob into a Sequence.
func Parse(data []byte) (*Sequence, error) {
	if len(data) < eventDataOffset {
		return nil, fmt.Errorf("seq: file too small (%d bytes, need at least %d)", len(data), eventDataOffset)
	}

	s := &Sequence{}

	// Header
	s.Version = strings.TrimRight(string(data[versionOffset:versionOffset+versionLen]), "\x00")
	s.Loop = data[loopOffset] != 0
	s.Bars = int(binary.LittleEndian.Uint16(data[barsOffset:]))
	bpmRaw := binary.LittleEndian.Uint16(data[bpmOffset:])
	s.BPM = float64(bpmRaw) / 10.0

	// Tracks
	for i := range trackCount {
		off := trackDataOffset + i*trackChunkSize
		if off+trackChunkSize > len(data) {
			break
		}
		chunk := data[off : off+trackChunkSize]
		name := strings.TrimRight(string(chunk[:trackNameLen]), "\x00")
		pgm := strings.Trim(string(chunk[trackPGMNameOff:trackPGMNameOff+trackNameLen]), "\x00")
		s.Tracks[i] = Track{
			Index:       i,
			Name:        name,
			PGMName:     pgm,
			MIDIChannel: int(chunk[trackMIDIChanOff]),
		}
	}

	// Events
	off := eventDataOffset
	for off+eventSize <= len(data) {
		ev := data[off : off+eventSize]
		if isTerminator(ev) {
			break
		}
		s.Events = append(s.Events, parseEvent(ev))
		off += eventSize
	}

	return s, nil
}

// isTerminator returns true for the 16-byte end sentinel.
// Byte 3 = 0x7F (not 0xFF) distinguishes it from the separator and from events.
func isTerminator(b []byte) bool {
	return b[0] == 0xFF && b[1] == 0xFF && b[2] == 0xFF && b[3] == 0x7F &&
		b[4] == 0xFF && b[5] == 0xFF && b[6] == 0xFF && b[7] == 0xFF
}

// parseEvent decodes a 16-byte event record.
//
// Wire layout (all multi-byte fields are little-endian):
//
//	[0-3]  tick position (uint32)
//	[4]    track number, 1-indexed in file; stored 0-indexed in Event.Track
//	[5]    MIDI status byte (0x90 = NoteOn ch0)
//	[6]    MIDI note
//	[7]    velocity
//	[8-11] duration in ticks (uint32)
//	[12]   padding (0x00)
//	[13]   pad index approximation (note - 36 for factory mappings)
//	[14-15] padding (0x00 0x00)
func parseEvent(b []byte) Event {
	tick := binary.LittleEndian.Uint32(b[0:4])
	track := int(b[4])
	if track > 0 {
		track-- // file is 1-indexed; internal representation is 0-indexed
	}
	status := b[5]
	note := b[6]
	velocity := b[7] & 0x7F
	duration := uint16(binary.LittleEndian.Uint32(b[8:12]))

	evType := EventNoteOn
	if status >= 0x80 {
		evType = EventType(status & 0xF0)
	}

	return Event{
		Tick:     tick,
		Track:    track,
		Type:     evType,
		Note:     note,
		Velocity: velocity,
		Duration: duration,
	}
}
