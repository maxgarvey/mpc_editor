package seq

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// PatchFile reads a .SEQ file, updates BPM and bar count in-place, and writes it back.
func PatchFile(path string, bpm float64, bars int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < eventDataOffset {
		return fmt.Errorf("seq: file too small to patch")
	}
	binary.LittleEndian.PutUint16(data[bpmOffset:], uint16(bpm*10))
	binary.LittleEndian.PutUint16(data[barsOffset:], uint16(bars))
	return os.WriteFile(path, data, 0o644)
}

// encodeEvent encodes a single NoteOn Event into the 16-byte MPC wire format.
//
// Wire layout (all multi-byte fields are little-endian):
//
//	[0-3]  tick position (uint32)
//	[4]    track number, 1-indexed (Event.Track + 1)
//	[5]    MIDI status: 0x90 for NoteOn
//	[6]    note
//	[7]    velocity
//	[8-11] duration in ticks (uint32)
//	[12]   0x00 (padding)
//	[13]   pad index: note - 36 (for factory note mapping; 0 if note < 36)
//	[14-15] 0x00 0x00 (padding)
func encodeEvent(ev Event) [eventSize]byte {
	var b [eventSize]byte
	binary.LittleEndian.PutUint32(b[0:4], ev.Tick)
	b[4] = byte(ev.Track + 1)
	b[5] = 0x90 // NoteOn, channel 0
	b[6] = ev.Note
	b[7] = ev.Velocity & 0x7F
	binary.LittleEndian.PutUint32(b[8:12], uint32(ev.Duration))
	if ev.Note >= 36 {
		b[13] = ev.Note - 36
	}
	return b
}

// Create builds a complete, self-consistent .SEQ binary from scratch.
// trackName and pgmName may be empty; events are sorted by tick before encoding.
// The returned bytes are ready to write directly to a .SEQ file.
func Create(bpm float64, bars int, trackName, pgmName string, events []Event) []byte {
	// Sort events by tick before encoding.
	evs := make([]Event, len(events))
	copy(evs, events)
	sort.Slice(evs, func(i, j int) bool { return evs[i].Tick < evs[j].Tick })

	fileSize := eventDataOffset + (len(evs)+1)*eventSize // +1 for terminator
	data := make([]byte, fileSize)

	// File size (bytes 0-3).
	binary.LittleEndian.PutUint32(data[0:4], uint32(fileSize))

	// Magic (bytes 4-19).
	copy(data[versionOffset:], "MPC1000 SEQ 4.40")

	// Unknown header bytes (observed constant values from Sequence01.SEQ).
	data[0x14] = 0x00
	data[0x15] = 0x01
	data[0x16] = 0x01
	data[0x17] = 0x00
	data[0x18] = 0x01
	data[0x19] = 0x00
	binary.LittleEndian.PutUint16(data[0x1A:], 1000) // unknown constant

	// Bars and BPM.
	binary.LittleEndian.PutUint16(data[barsOffset:], uint16(bars))
	binary.LittleEndian.PutUint16(data[bpmOffset:], uint16(bpm*10))

	// Timing/clock map: 1000 entries at 0x30, 4 bytes each.
	// Entry n: bytes[0-2] = n*TicksPerBar as 24-bit LE, byte[3] = 0x60.
	for n := 0; n < 1000; n++ {
		tick := uint32(n) * TicksPerBar
		off := 0x30 + n*4
		data[off+0] = byte(tick)
		data[off+1] = byte(tick >> 8)
		data[off+2] = byte(tick >> 16)
		data[off+3] = 0x60
	}

	// Track headers: 64 tracks × 48 bytes at 0x1000.
	// Active track settings (MIDI channel 1): bytes [32-47] from real files.
	activeSettings := [16]byte{0x00, 0x00, 0x01, 0x01, 0x64, 0x00, 0x00, 0x00, 0x1e, 0x00, 0x6e, 0x00, 0x00, 0x00, 0x00, 0x32}
	// Empty track settings (MIDI channel 0): same but channel bytes zeroed.
	emptySettings := [16]byte{0x00, 0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00, 0x1e, 0x00, 0x6e, 0x00, 0x00, 0x00, 0x00, 0x32}

	for i := range trackCount {
		off := trackDataOffset + i*trackChunkSize
		var name string
		if i == 0 && trackName != "" {
			name = trackName
		} else {
			name = fmt.Sprintf("Track%02d", i+1)
		}
		copy(data[off:], name)
		if i == 0 && pgmName != "" {
			data[off+trackPGMNameOff] = 0x00
			copy(data[off+trackPGMNameOff+1:], pgmName)
		}
		if i == 0 {
			copy(data[off+32:], activeSettings[:])
		} else {
			copy(data[off+32:], emptySettings[:])
		}
	}

	// Event section separator at 0x1C00.
	copy(data[0x1C00:], []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	// Events starting at 0x1C10.
	off := eventDataOffset
	for _, ev := range evs {
		b := encodeEvent(ev)
		copy(data[off:], b[:])
		off += eventSize
	}

	// Terminator.
	copy(data[off:], []byte{0xFF, 0xFF, 0xFF, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	return data
}

// WriteEvents re-serializes all NoteOn events from s back into the .SEQ file at path.
// The header (everything before the event data region) is preserved unchanged.
// The file is padded with 0xFF to maintain its original length.
func WriteEvents(path string, s *Sequence) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < eventDataOffset+eventSize {
		return fmt.Errorf("seq: file too small to write events")
	}

	// Only write NoteOn events with velocity > 0, sorted by tick.
	events := make([]Event, 0, len(s.Events))
	for _, ev := range s.Events {
		if ev.Type == EventNoteOn && ev.Velocity > 0 {
			events = append(events, ev)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Tick < events[j].Tick })

	// Maximum events that fit before the terminator (one slot reserved for terminator).
	maxEvents := (len(data) - eventDataOffset - eventSize) / eventSize
	if len(events) > maxEvents {
		return fmt.Errorf("seq: too many events (%d > max %d)", len(events), maxEvents)
	}

	// Build output: copy header unchanged, then write events.
	out := make([]byte, len(data))
	copy(out, data[:eventDataOffset])

	off := eventDataOffset
	for _, ev := range events {
		b := encodeEvent(ev)
		copy(out[off:], b[:])
		off += eventSize
	}

	// Write 16-byte terminator: ff ff ff 7f ff ff ff ff ff ff ff ff ff ff ff ff
	terminator := [eventSize]byte{
		0xFF, 0xFF, 0xFF, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	}
	copy(out[off:], terminator[:])
	off += eventSize

	// Pad remainder with 0xFF.
	for i := off; i < len(out); i++ {
		out[i] = 0xFF
	}

	return os.WriteFile(path, out, 0o644)
}
