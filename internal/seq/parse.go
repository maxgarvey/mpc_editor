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
//	[0-3]   tick position (uint32)
//	[4]     track number, 1-indexed in file; stored 0-indexed in Event.Track
//	[5]     status byte (0x90 = NoteOn, 0xC0 = program change, 0x82 = unknown)
//	[6]     note (data byte 1)
//	[7]     velocity (data byte 2)
//	[8-9]   duration in ticks (uint16)
//	[10]    Note Variation type
//	[11]    Note Variation value (signed)
//	[12-15] 00 PP 00 00 with PP = pad index in recorded files (0 in Akai's
//	        factory sequences), or ff ff ff ff (meaning unknown). Not modelled;
//	        preserved through Event.raw.
func parseEvent(b []byte) Event {
	tick := binary.LittleEndian.Uint32(b[0:4])
	track := int(b[4])
	if track > 0 {
		track-- // file is 1-indexed; internal representation is 0-indexed
	}
	status := b[5]

	evType := EventNoteOn
	if status >= 0x80 {
		evType = EventType(status & 0xF0)
	}

	ev := Event{
		Tick:           tick,
		Track:          track,
		Type:           evType,
		Status:         status,
		Note:           b[6],
		Velocity:       b[7] & 0x7F,
		Duration:       binary.LittleEndian.Uint16(b[8:10]),
		VariationType:  b[10],
		VariationValue: int8(b[11]),
		hasRaw:         true,
	}
	copy(ev.raw[:], b)
	return ev
}
