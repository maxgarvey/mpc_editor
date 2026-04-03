package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/db"
)

// FileDetailData holds template data for the file detail page.
type FileDetailData struct {
	File           FileInfo
	PgmMeta        *PgmMetaInfo
	WavMeta        *WavMetaInfo
	SeqMeta        *SeqMetaInfo
	Samples        []SampleInfo // for .pgm files
	UsedBy         []FileRef    // for .wav files: programs using this sample
	MissingSamples int64
}

// FileInfo is a simplified view of a catalog file.
type FileInfo struct {
	ID       int64
	Path     string
	FileType string
	Size     int64
}

// PgmMetaInfo holds .pgm metadata for display.
type PgmMetaInfo struct {
	MIDIProgramChange int64
}

// WavMetaInfo holds .wav metadata for display.
type WavMetaInfo struct {
	SampleRate    int64
	Channels      int64
	BitsPerSample int64
	FrameCount    int64
	Duration      string // formatted duration
	Source        string
}

// SeqMetaInfo holds .seq metadata for display.
type SeqMetaInfo struct {
	BPM     float64
	Bars    int64
	Version string
}

// SampleInfo represents a sample reference in a .pgm detail view.
type SampleInfo struct {
	Pad        int64
	Layer      int64
	SampleName string
	Found      bool
	SamplePath string
}

// FileRef is a minimal file reference (for "used by" lists).
type FileRef struct {
	ID   int64
	Path string
}

func (s *Server) handleFileDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	if idStr == "" {
		// Try path-based lookup.
		idStr = strings.TrimPrefix(r.URL.Path, "/file/")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	f, err := s.queries.GetFileByID(ctx, id)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	data := FileDetailData{
		File: FileInfo{
			ID:       f.ID,
			Path:     f.Path,
			FileType: f.FileType,
			Size:     f.Size,
		},
	}

	switch f.FileType {
	case "pgm":
		s.enrichPgmDetail(ctx, &data, f.ID)
	case "wav":
		s.enrichWavDetail(ctx, &data, f.ID)
	case "seq":
		s.enrichSeqDetail(ctx, &data, f.ID)
	}

	s.renderTemplate(w, "file_detail.html", data)
}

func (s *Server) enrichPgmDetail(ctx context.Context, data *FileDetailData, fileID int64) {
	meta, err := s.queries.GetPgmMeta(ctx, fileID)
	if err == nil {
		data.PgmMeta = &PgmMetaInfo{MIDIProgramChange: meta.MidiPgmChange}
	}

	samples, err := s.queries.ListPgmSamples(ctx, fileID)
	if err == nil {
		for _, s := range samples {
			data.Samples = append(data.Samples, SampleInfo{
				Pad:        s.Pad,
				Layer:      s.Layer,
				SampleName: s.SampleName,
				Found:      s.SampleFileID.Valid,
				SamplePath: s.SamplePath.String,
			})
		}
	}

	missing, err := s.queries.CountMissingSamples(ctx, fileID)
	if err == nil {
		data.MissingSamples = missing
	}
}

func (s *Server) enrichWavDetail(ctx context.Context, data *FileDetailData, fileID int64) {
	meta, err := s.queries.GetWavMeta(ctx, fileID)
	if err == nil {
		var dur string
		if meta.SampleRate > 0 {
			secs := float64(meta.FrameCount) / float64(meta.SampleRate)
			dur = fmt.Sprintf("%.2fs", secs)
		}
		data.WavMeta = &WavMetaInfo{
			SampleRate:    meta.SampleRate,
			Channels:      meta.Channels,
			BitsPerSample: meta.BitsPerSample,
			FrameCount:    meta.FrameCount,
			Duration:      dur,
			Source:        meta.Source,
		}
	}

	programs, err := s.queries.ListProgramsUsingSample(ctx, sql.NullInt64{Int64: fileID, Valid: true})
	if err == nil {
		for _, p := range programs {
			data.UsedBy = append(data.UsedBy, FileRef{ID: p.ID, Path: p.Path})
		}
	}
}

func (s *Server) handleSetWavSource(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	source := r.FormValue("source")
	ctx := context.Background()
	if err := s.queries.UpdateWavSource(ctx, db.UpdateWavSourceParams{
		Source: source,
		FileID: id,
	}); err != nil {
		http.Error(w, "failed to update source", http.StatusInternalServerError)
		return
	}

	// Re-render file detail
	s.handleFileDetail(w, r)
}

func (s *Server) enrichSeqDetail(ctx context.Context, data *FileDetailData, fileID int64) {
	meta, err := s.queries.GetSeqMeta(ctx, fileID)
	if err == nil && meta.Version != "" {
		data.SeqMeta = &SeqMetaInfo{
			BPM:     meta.Bpm,
			Bars:    meta.Bars,
			Version: meta.Version,
		}
	}
}
