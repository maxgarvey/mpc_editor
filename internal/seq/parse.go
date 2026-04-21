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
		s.Tracks[i] = Track{
			Index:       i,
			Name:        name,
			MIDIChannel: int(chunk[16]),
			Program:     int(chunk[17]),
			Status:      chunk[18],
		}
	}

	// Events
	off := eventDataOffset
	for off+eventSize <= len(data) {
		ev := data[off : off+eventSize]

		// Terminator: 0xFF x 8
		if isTerminator(ev) {
			break
		}

		event := parseEvent(ev)
		// Skip internal NoteOff markers (note=0, vel=0 housekeeping events).
		if event.Type == EventNoteOn && event.Velocity == 0 {
			off += eventSize
			continue
		}
		s.Events = append(s.Events, event)
		off += eventSize
	}

	return s, nil
}

// isTerminator returns true for the MPC sentinel ff ff ff 7f ff ff ff ff.
// Byte 3 is 0x7F (not 0xFF), which is what distinguishes it from an event.
func isTerminator(b []byte) bool {
	return b[0] == 0xFF && b[1] == 0xFF && b[2] == 0xFF && b[3] == 0x7F &&
		b[4] == 0xFF && b[5] == 0xFF && b[6] == 0xFF && b[7] == 0xFF
}

// parseEvent decodes an 8-byte event using the bit-packed MPC format.
func parseEvent(b []byte) Event {
	// Tick: 20-bit value from bytes 0-2
	tickLow := uint32(binary.LittleEndian.Uint16(b[0:2]))
	tickHigh := uint32(b[2]&0x0F) * 65536
	tick := tickLow + tickHigh

	// Track: byte 3 bits 0-5
	track := int(b[3] & 0x3F)

	// Event type: byte 4 is the MIDI channel for NoteOn (< 0x80),
	// or the MIDI status byte for other event types (≥ 0x80).
	var evType EventType
	var note byte
	if b[4] < 0x80 {
		evType = EventNoteOn
		note = b[6] & 0x7F // note is at byte 6 for NoteOn events
	} else {
		evType = EventType(b[4] & 0xF0)
		note = 0
	}

	// Velocity: byte 7 bits 0-6
	velocity := b[7] & 0x7F

	// Duration: scattered across bytes 2, 3, 5
	dur := (uint16(b[2]&0xF0) << 6) + (uint16(b[3]&0xC0) << 2) + uint16(b[5])
	// Compensate for track bits leaking into duration
	if track > 0 {
		dur -= uint16(track * 4)
	}

	return Event{
		Tick:     tick,
		Track:    track,
		Type:     evType,
		Note:     note,
		Velocity: velocity,
		Duration: dur,
		Data1:    b[5],
		Data2:    b[6],
	}
}
