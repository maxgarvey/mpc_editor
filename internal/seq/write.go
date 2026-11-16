package seq

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// PatchLoop updates the loop flag (byte 0x17) in an existing .SEQ file in-place.
func PatchLoop(path string, loop bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < eventDataOffset {
		return fmt.Errorf("seq: file too small to patch")
	}
	if loop {
		data[loopOffset] = 0x01
	} else {
		data[loopOffset] = 0x00
	}
	return os.WriteFile(path, data, 0o644)
}

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

// encodeEvent encodes an Event into the 16-byte MPC wire format (see parseEvent).
//
// An event that came from a parsed file starts from its original bytes and only
// the fields that differ from what was parsed are rewritten, so unmodelled bytes
// round-trip unchanged. A new event starts from zeros: status 0x90 (or Status /
// Type for other kinds) and byte 13 = note − 36, as recorded files do.
func encodeEvent(ev Event) [eventSize]byte {
	var b [eventSize]byte
	var orig Event
	all := !ev.hasRaw // new event: write every field
	if ev.hasRaw {
		b = ev.raw
		orig = parseEvent(b[:])
	}

	if all || ev.Tick != orig.Tick {
		binary.LittleEndian.PutUint32(b[0:4], ev.Tick)
	}
	if all || ev.Track != orig.Track {
		b[4] = byte(ev.Track + 1)
	}
	switch {
	case ev.Status != 0:
		if all || ev.Status != orig.Status {
			b[5] = ev.Status
		}
	case all:
		b[5] = byte(ev.Type)
		if ev.Type == EventNoteOn || ev.Type == 0 {
			b[5] = 0x90
		}
	case ev.Type != orig.Type:
		b[5] = byte(ev.Type)
	}
	if all || ev.Note != orig.Note {
		b[6] = ev.Note
		// Keep byte 13 (pad index) in step with the note, as recorded files do.
		if ev.IsNote() {
			b[13] = 0
			if ev.Note >= 36 && ev.Note-36 < 64 {
				b[13] = ev.Note - 36
			}
		}
	}
	if all || ev.Velocity != orig.Velocity {
		b[7] = ev.Velocity & 0x7F
	}
	if all || ev.Duration != orig.Duration {
		binary.LittleEndian.PutUint16(b[8:10], ev.Duration)
	}
	if all || ev.VariationType != orig.VariationType {
		b[10] = ev.VariationType
	}
	if all || ev.VariationValue != orig.VariationValue {
		b[11] = byte(ev.VariationValue)
	}
	return b
}

// Create builds a complete, self-consistent .SEQ binary from scratch.
// trackName and pgmName may be empty; events are sorted by tick before encoding.
// loop=true sets the MPC loop flag (byte 0x17=1, meaning "1-End"); loop=false means off.
// The returned bytes are ready to write directly to a .SEQ file.
func Create(bpm float64, bars int, trackName, pgmName string, loop bool, events []Event) []byte {
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
	if loop {
		data[0x17] = 0x01
	}
	data[0x18] = 0x01
	data[0x19] = 0x00
	binary.LittleEndian.PutUint16(data[0x1A:], 1000) // unknown constant

	// Bars and BPM.
	binary.LittleEndian.PutUint16(data[barsOffset:], uint16(bars))
	binary.LittleEndian.PutUint16(data[bpmOffset:], uint16(bpm*10))

	// Timing/clock map: 1000 entries at 0x30, 4 bytes each.
	// Entry n: bytes[0-2] = n*TicksPerBar as 24-bit LE, byte[3] = 0x60.
	for n := range 1000 {
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

// WriteEvents re-serializes the events of s back into the .SEQ file at path.
// The header (everything before the event data region) is preserved unchanged.
// Every event is written, including program changes and record types the
// editor does not understand; only NoteOn events with velocity 0 are dropped.
// Events are stably sorted by tick, so records sharing a tick keep their order
// (factory files put the program change first). Parsed events keep their
// unmodelled bytes (see encodeEvent). The file is padded with 0xFF to maintain
// its original length.
func WriteEvents(path string, s *Sequence) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < eventDataOffset+eventSize {
		return fmt.Errorf("seq: file too small to write events")
	}

	events := make([]Event, 0, len(s.Events))
	for _, ev := range s.Events {
		if ev.IsNote() && ev.Velocity == 0 {
			continue
		}
		events = append(events, ev)
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Tick < events[j].Tick })

	// Grow the output if the new event count no longer fits in the original file.
	// Always reserve one slot for the terminator.
	needed := eventDataOffset + (len(events)+1)*eventSize
	outSize := max(len(data), needed)

	// Build output: copy header unchanged, then write events.
	out := make([]byte, outSize)
	copy(out, data[:eventDataOffset])
	// Keep the file-size field in the header consistent with the actual size.
	binary.LittleEndian.PutUint32(out[0:4], uint32(outSize))

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
