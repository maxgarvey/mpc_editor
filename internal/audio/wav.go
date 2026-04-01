package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Format describes the audio format of a WAV file.
type Format struct {
	SampleRate    int // e.g., 44100
	Channels      int // 1=mono, 2=stereo
	BitsPerSample int // typically 16
}

// FrameSize returns the number of bytes per frame (channels * bitsPerSample/8).
func (f Format) FrameSize() int {
	return f.Channels * f.BitsPerSample / 8
}

// Sample holds decoded audio data from a WAV file.
type Sample struct {
	Format      Format
	FrameLength int       // number of frames (samples per channel)
	Data        []byte    // raw PCM data
}

// OpenWAV reads a WAV file and returns a Sample.
func OpenWAV(path string) (*Sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open wav: %w", err)
	}
	defer f.Close()
	return ReadWAV(f)
}

// ReadWAV reads a WAV file from a reader.
func ReadWAV(r io.ReadSeeker) (*Sample, error) {
	// Read RIFF header
	var riffHeader [12]byte
	if _, err := io.ReadFull(r, riffHeader[:]); err != nil {
		return nil, fmt.Errorf("read RIFF header: %w", err)
	}
	if string(riffHeader[0:4]) != "RIFF" {
		return nil, fmt.Errorf("not a RIFF file")
	}
	if string(riffHeader[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a WAVE file")
	}

	var format Format
	var dataBytes []byte

	// Read chunks
	for {
		var chunkHeader [8]byte
		if _, err := io.ReadFull(r, chunkHeader[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, fmt.Errorf("read chunk header: %w", err)
		}
		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		switch chunkID {
		case "fmt ":
			if err := readFmtChunk(r, chunkSize, &format); err != nil {
				return nil, err
			}
		case "data":
			dataBytes = make([]byte, chunkSize)
			if _, err := io.ReadFull(r, dataBytes); err != nil {
				return nil, fmt.Errorf("read data chunk: %w", err)
			}
		default:
			// Skip unknown chunks
			if _, err := r.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("skip chunk %q: %w", chunkID, err)
			}
		}

		// Chunks are word-aligned (pad byte if odd size)
		if chunkSize%2 != 0 {
			r.Seek(1, io.SeekCurrent)
		}
	}

	if format.SampleRate == 0 {
		return nil, fmt.Errorf("no fmt chunk found")
	}
	if dataBytes == nil {
		return nil, fmt.Errorf("no data chunk found")
	}

	frameSize := format.FrameSize()
	frameLength := len(dataBytes) / frameSize

	return &Sample{
		Format:      format,
		FrameLength: frameLength,
		Data:        dataBytes,
	}, nil
}

func readFmtChunk(r io.Reader, size uint32, format *Format) error {
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("read fmt chunk: %w", err)
	}

	audioFormat := binary.LittleEndian.Uint16(data[0:2])
	if audioFormat != 1 {
		return fmt.Errorf("unsupported audio format %d (only PCM=1 supported)", audioFormat)
	}

	format.Channels = int(binary.LittleEndian.Uint16(data[2:4]))
	format.SampleRate = int(binary.LittleEndian.Uint32(data[4:8]))
	format.BitsPerSample = int(binary.LittleEndian.Uint16(data[14:16]))

	return nil
}

// AsSamples converts raw PCM data to per-channel int arrays.
// Returns [numChannels][frameLength]int.
func (s *Sample) AsSamples() [][]int {
	channels := make([][]int, s.Format.Channels)
	for i := range channels {
		channels[i] = make([]int, s.FrameLength)
	}

	bytesPerSample := s.Format.BitsPerSample / 8
	frameSize := s.Format.FrameSize()

	for frame := 0; frame < s.FrameLength; frame++ {
		for ch := 0; ch < s.Format.Channels; ch++ {
			offset := frame*frameSize + ch*bytesPerSample
			if offset+1 < len(s.Data) {
				// 16-bit signed little-endian
				low := int(s.Data[offset])
				high := int(s.Data[offset+1])
				sample := (high << 8) + (low & 0x00ff)
				// Sign extend
				if sample >= 32768 {
					sample -= 65536
				}
				channels[ch][frame] = sample
			}
		}
	}
	return channels
}

// SubRegion extracts a slice of the sample from frame `from` to frame `to`.
func (s *Sample) SubRegion(from, to int) *Sample {
	if from < 0 {
		from = 0
	}
	if to > s.FrameLength {
		to = s.FrameLength
	}
	frameSize := s.Format.FrameSize()
	regionLen := to - from
	regionData := make([]byte, regionLen*frameSize)
	copy(regionData, s.Data[from*frameSize:(from+regionLen)*frameSize])
	return &Sample{
		Format:      s.Format,
		FrameLength: regionLen,
		Data:        regionData,
	}
}

// SaveWAV writes the sample as a WAV file.
func (s *Sample) SaveWAV(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create wav: %w", err)
	}
	defer f.Close()
	return s.WriteWAV(f)
}

// WriteWAV writes the sample as a WAV to a writer.
func (s *Sample) WriteWAV(w io.Writer) error {
	frameSize := s.Format.FrameSize()
	dataSize := uint32(s.FrameLength * frameSize)
	fileSize := 36 + dataSize // RIFF header (12) + fmt chunk (24) + data header (8) + data - 8

	// RIFF header
	write := func(data interface{}) error { return binary.Write(w, binary.LittleEndian, data) }

	if _, err := w.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := write(uint32(fileSize)); err != nil {
		return err
	}
	if _, err := w.Write([]byte("WAVE")); err != nil {
		return err
	}

	// fmt chunk
	if _, err := w.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := write(uint32(16)); err != nil { // chunk size
		return err
	}
	if err := write(uint16(1)); err != nil { // PCM format
		return err
	}
	if err := write(uint16(s.Format.Channels)); err != nil {
		return err
	}
	if err := write(uint32(s.Format.SampleRate)); err != nil {
		return err
	}
	byteRate := uint32(s.Format.SampleRate * frameSize)
	if err := write(byteRate); err != nil {
		return err
	}
	if err := write(uint16(frameSize)); err != nil { // block align
		return err
	}
	if err := write(uint16(s.Format.BitsPerSample)); err != nil {
		return err
	}

	// data chunk
	if _, err := w.Write([]byte("data")); err != nil {
		return err
	}
	if err := write(dataSize); err != nil {
		return err
	}
	if _, err := w.Write(s.Data[:dataSize]); err != nil {
		return err
	}

	return nil
}
