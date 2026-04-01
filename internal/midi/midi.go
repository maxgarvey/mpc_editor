package midi

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
)

const (
	DefaultPPQ      = 96
	DefaultVelocity = 127
	DefaultNoteLen  = 32 // ticks
	DefaultStartKey = 35
)

// Event represents a MIDI event at a specific tick.
type Event struct {
	Tick    int
	Status  byte
	Key     byte
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

// Save writes the sequence to a Standard MIDI File (Type 0).
func (s *Sequence) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.Write(f)
}

// Write writes the sequence as a Standard MIDI File (Type 0) to the writer.
func (s *Sequence) Write(w io.Writer) error {
	trackData := s.buildTrackData()

	// MThd header
	if _, err := w.Write([]byte("MThd")); err != nil {
		return err
	}
	if err := writeUint32BE(w, 6); err != nil { // header length
		return err
	}
	if err := writeUint16BE(w, 0); err != nil { // format 0
		return err
	}
	if err := writeUint16BE(w, 1); err != nil { // 1 track
		return err
	}
	if err := writeUint16BE(w, uint16(s.PPQ)); err != nil { // ticks per quarter note
		return err
	}

	// MTrk chunk
	if _, err := w.Write([]byte("MTrk")); err != nil {
		return err
	}
	if err := writeUint32BE(w, uint32(len(trackData))); err != nil {
		return err
	}
	_, err := w.Write(trackData)
	return err
}

// buildTrackData generates the binary track data including end-of-track.
func (s *Sequence) buildTrackData() []byte {
	// Sort events by tick, then NOTE_OFF before NOTE_ON at same tick
	sorted := make([]Event, len(s.Events))
	copy(sorted, s.Events)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Tick != sorted[j].Tick {
			return sorted[i].Tick < sorted[j].Tick
		}
		// NOTE_OFF (0x80) before NOTE_ON (0x90) at same tick
		return sorted[i].Status < sorted[j].Status
	})

	var data []byte
	lastTick := 0

	for _, ev := range sorted {
		delta := ev.Tick - lastTick
		if delta < 0 {
			delta = 0
		}
		data = append(data, encodeVarLen(delta)...)
		data = append(data, ev.Status, ev.Key, ev.Velocity)
		lastTick = ev.Tick
	}

	// End of track meta event
	data = append(data, 0x00, 0xFF, 0x2F, 0x00)

	return data
}

// encodeVarLen encodes an integer as MIDI variable-length quantity.
func encodeVarLen(value int) []byte {
	if value < 0 {
		value = 0
	}
	if value < 0x80 {
		return []byte{byte(value)}
	}

	var buf []byte
	buf = append(buf, byte(value&0x7F))
	value >>= 7
	for value > 0 {
		buf = append(buf, byte((value&0x7F)|0x80))
		value >>= 7
	}
	// Reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return buf
}

func writeUint16BE(w io.Writer, v uint16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func writeUint32BE(w io.Writer, v uint32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// ReadMIDI reads a Standard MIDI File and returns the sequence.
// This is a minimal reader for testing purposes — supports Type 0 only.
func ReadMIDI(path string) (*Sequence, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseMIDI(f)
}

// ParseMIDI parses a Standard MIDI File from a reader.
func ParseMIDI(r io.Reader) (*Sequence, error) {
	// Read MThd
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if string(hdr[:]) != "MThd" {
		return nil, fmt.Errorf("not a MIDI file: header %q", hdr)
	}

	var headerLen uint32
	if err := binary.Read(r, binary.BigEndian, &headerLen); err != nil {
		return nil, err
	}
	if headerLen < 6 {
		return nil, fmt.Errorf("invalid header length: %d", headerLen)
	}

	var format, numTracks, ppq uint16
	binary.Read(r, binary.BigEndian, &format)
	binary.Read(r, binary.BigEndian, &numTracks)
	if err := binary.Read(r, binary.BigEndian, &ppq); err != nil {
		return nil, err
	}

	// Skip any extra header bytes
	if headerLen > 6 {
		io.CopyN(io.Discard, r, int64(headerLen-6))
	}

	seq := NewSequence(int(ppq))

	// Read tracks
	for t := 0; t < int(numTracks); t++ {
		var trkHdr [4]byte
		if _, err := io.ReadFull(r, trkHdr[:]); err != nil {
			return seq, nil // no more tracks
		}
		if string(trkHdr[:]) != "MTrk" {
			return nil, fmt.Errorf("expected MTrk, got %q", trkHdr)
		}

		var trkLen uint32
		if err := binary.Read(r, binary.BigEndian, &trkLen); err != nil {
			return nil, err
		}

		trackData := make([]byte, trkLen)
		if _, err := io.ReadFull(r, trackData); err != nil {
			return nil, fmt.Errorf("read track data: %w", err)
		}

		// Parse track events
		pos := 0
		tick := 0
		for pos < len(trackData) {
			delta, bytesRead := decodeVarLen(trackData[pos:])
			pos += bytesRead
			tick += delta

			if pos >= len(trackData) {
				break
			}

			status := trackData[pos]
			if status == 0xFF {
				// Meta event
				pos++ // skip 0xFF
				if pos >= len(trackData) {
					break
				}
				pos++ // skip type
				if pos >= len(trackData) {
					break
				}
				metaLen, br := decodeVarLen(trackData[pos:])
				pos += br + metaLen
			} else if status >= 0x80 && status <= 0xEF {
				pos++ // skip status
				if pos+1 >= len(trackData) {
					break
				}
				key := trackData[pos]
				vel := trackData[pos+1]
				pos += 2
				seq.Events = append(seq.Events, Event{
					Tick:     tick,
					Status:   status,
					Key:      key,
					Velocity: vel,
				})
			} else {
				// Skip unknown
				pos++
			}
		}
	}

	return seq, nil
}

// decodeVarLen reads a MIDI variable-length quantity.
func decodeVarLen(data []byte) (int, int) {
	value := 0
	bytesRead := 0
	for i := 0; i < len(data) && i < 4; i++ {
		b := data[i]
		value = (value << 7) | int(b&0x7F)
		bytesRead++
		if b&0x80 == 0 {
			break
		}
	}
	return value, bytesRead
}
