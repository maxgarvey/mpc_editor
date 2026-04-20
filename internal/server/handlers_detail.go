package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// handleDetailSelect updates the server-side selected path without rendering.
// Used when switching tabs so browser nav stays in sync.
func (s *Server) handleDetailSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.FormValue("path")
	if path != "" {
		path = s.resolvePath(path)
	}
	s.session.SelectedDetailPath = path
	if err := s.queries.UpdateLastDetailPath(r.Context(), path); err != nil {
		log.Printf("save last detail path: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleDetail inspects the file type and renders the appropriate detail partial
// into #detail-panel.
func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	path := s.resolvePath(r.FormValue("path"))
	if path == "" {
		s.renderTemplate(w, "detail_empty.html", nil)
		return
	}

	s.session.SelectedDetailPath = path
	_ = s.queries.UpdateLastDetailPath(r.Context(), path)

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pgm":
		s.renderDetailPGM(w, r, path)
	case ".wav":
		s.renderDetailWAV(w, path)
	case ".seq":
		s.renderDetailSEQ(w, r, path)
	case ".sng":
		s.renderDetailSNG(w, path)
	case ".txt":
		s.renderDetailTXT(w, path)
	default:
		s.renderDetailFile(w, path, ext)
	}
}

func (s *Server) renderDetailPGM(w http.ResponseWriter, r *http.Request, path string) {
	// Reuse the in-memory program if it's already loaded for this path,
	// so unsaved changes (e.g. from drag-drop assignment) are preserved.
	if path != s.session.FilePath || s.session.Program == nil {
		prog, err := pgm.OpenProgram(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		pgmDir := filepath.Dir(path)

		s.session.Program = prog
		s.session.FilePath = path
		s.session.SampleDir = pgmDir
		s.session.SelectedPad = 0
		s.session.Matrix.Clear()

		for i := range 64 {
			pad := prog.Pad(i)
			for j := range 4 {
				name := pad.Layer(j).GetSampleName()
				if name != "" {
					sampleLibrary := filepath.Join(s.session.WorkspacePath, "sample_library")
					ref := pgm.FindSampleInDirs(name, pgmDir, sampleLibrary, s.session.WorkspacePath)
					s.session.Matrix.Set(i, j, &ref)
				}
			}
		}
	}

	if err := s.queries.UpdateLastPGMPath(r.Context(), path); err != nil {
		log.Printf("save last pgm path: %v", err)
	}
	s.session.Prefs.LastPGMPath = path

	bank := parseIntParam(r, "bank", 0)
	data := map[string]any{
		"Session":   s.session,
		"PadGrid":   s.padGridData(bank),
		"PadParams": s.padParamsData(),
		"Banks":     []int{0, 1, 2, 3},
		"Bank":      bank,
	}

	// Look up file ID for tags.
	workspace := s.session.WorkspacePath
	if relPath, err := filepath.Rel(workspace, path); err == nil {
		if f, err := s.queries.GetFileByPath(r.Context(), relPath); err == nil {
			data["FileID"] = f.ID
			data["Tags"] = s.loadTags(r.Context(), f.ID)
		}
	}

	s.renderTemplate(w, "detail_pgm.html", data)
}

func (s *Server) renderDetailWAV(w http.ResponseWriter, path string) {
	ctx := context.Background()
	workspace := s.session.WorkspacePath

	relPath, err := filepath.Rel(workspace, path)
	if err != nil {
		relPath = path
	}

	data := map[string]any{
		"Path":       path,
		"RelPath":    relPath,
		"HasProgram": s.session.Program != nil,
	}

	f, err := s.queries.GetFileByPath(ctx, relPath)
	if err == nil {
		data["FileID"] = f.ID

		meta, err := s.queries.GetWavMeta(ctx, f.ID)
		if err == nil {
			var dur string
			if meta.SampleRate > 0 {
				secs := float64(meta.FrameCount) / float64(meta.SampleRate)
				dur = fmt.Sprintf("%.2fs", secs)
			}
			data["WavMeta"] = &WavMetaInfo{
				SampleRate:    meta.SampleRate,
				Channels:      meta.Channels,
				BitsPerSample: meta.BitsPerSample,
				FrameCount:    meta.FrameCount,
				Duration:      dur,
				Source:        meta.Source,
			}
		}

		programs, err := s.queries.ListProgramsUsingSample(ctx, sql.NullInt64{Int64: f.ID, Valid: true})
		if err == nil {
			var usedBy []FileRef
			for _, p := range programs {
				usedBy = append(usedBy, FileRef{ID: p.ID, Path: p.Path})
			}
			data["UsedBy"] = usedBy
		}

		data["Tags"] = s.loadTags(ctx, f.ID)
	}

	s.renderTemplate(w, "detail_wav.html", data)
}

func (s *Server) renderDetailSEQ(w http.ResponseWriter, r *http.Request, path string) {
	sequence, err := seq.Open(path)
	if err != nil {
		data := SequenceViewData{Path: path, Error: err.Error()}
		s.renderTemplate(w, "detail_seq.html", data)
		return
	}

	bar := parseIntParam(r, "bar", 1)
	if bar < 1 {
		bar = 1
	}
	if bar > sequence.Bars {
		bar = sequence.Bars
	}

	grid := seq.BuildGrid(sequence, bar, s.noteToPadMap())
	data := SequenceViewData{
		Path:       path,
		BPM:        sequence.BPM,
		Bars:       sequence.Bars,
		Version:    sequence.Version,
		CurrentBar: bar,
		Grid:       grid,
	}

	// Look up file ID for tags.
	workspace := s.session.WorkspacePath
	if relPath, err := filepath.Rel(workspace, path); err == nil {
		if f, err := s.queries.GetFileByPath(r.Context(), relPath); err == nil {
			data.FileID = f.ID
			data.Tags = s.loadTags(r.Context(), f.ID)
		}
	}

	s.renderTemplate(w, "detail_seq.html", data)
}

func (s *Server) renderDetailSNG(w http.ResponseWriter, path string) {
	ctx := context.Background()
	workspace := s.session.WorkspacePath

	relPath, err := filepath.Rel(workspace, path)
	if err != nil {
		relPath = path
	}

	data := map[string]any{
		"Path":    path,
		"RelPath": relPath,
	}

	f, err := s.queries.GetFileByPath(ctx, relPath)
	if err == nil {
		data["FileID"] = f.ID
		data["Size"] = f.Size
		data["Tags"] = s.loadTags(ctx, f.ID)
	}

	s.renderTemplate(w, "detail_sng.html", data)
}

func (s *Server) renderDetailFile(w http.ResponseWriter, path, ext string) {
	ctx := context.Background()
	workspace := s.session.WorkspacePath

	relPath, err := filepath.Rel(workspace, path)
	if err != nil {
		relPath = path
	}

	data := map[string]any{
		"Path":     path,
		"RelPath":  relPath,
		"FileType": strings.TrimPrefix(ext, "."),
	}

	f, err := s.queries.GetFileByPath(ctx, relPath)
	if err == nil {
		data["FileID"] = f.ID
		data["Size"] = f.Size
		data["Tags"] = s.loadTags(ctx, f.ID)
	}

	s.renderTemplate(w, "detail_file.html", data)
}

// SampleReportEntry holds parsed data for one sample in a report.
type SampleReportEntry struct {
	Number int
	Name   string
	Pads   string
	Status string // "found" or "NOT FOUND"
	Found  bool
	Audio  string
	Source string
	Tags   string
	AlsoIn string
}

func (s *Server) renderDetailTXT(w http.ResponseWriter, path string) {
	workspace := s.session.WorkspacePath
	relPath, err := filepath.Rel(workspace, path)
	if err != nil {
		relPath = path
	}

	content, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	text := string(content)
	isSampleReport := strings.HasPrefix(text, "Sample Report for ")

	data := map[string]any{
		"Path":           path,
		"RelPath":        relPath,
		"Content":        text,
		"IsSampleReport": isSampleReport,
	}

	if isSampleReport {
		data["ReportEntries"] = parseSampleReport(text)
		// Extract the title (first line).
		if idx := strings.Index(text, "\n"); idx > 0 {
			data["ReportTitle"] = text[:idx]
		}
	}

	s.renderTemplate(w, "detail_txt.html", data)
}

// parseSampleReport extracts structured entries from a sample report text file.
func parseSampleReport(text string) []SampleReportEntry {
	var entries []SampleReportEntry
	lines := strings.Split(text, "\n")

	var current *SampleReportEntry
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Numbered entry: "1. SampleName"
		if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' {
			dotIdx := strings.Index(trimmed, ". ")
			if dotIdx > 0 {
				num, err := strconv.Atoi(trimmed[:dotIdx])
				if err == nil && num > 0 {
					if current != nil {
						entries = append(entries, *current)
					}
					current = &SampleReportEntry{
						Number: num,
						Name:   trimmed[dotIdx+2:],
					}
					continue
				}
			}
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(trimmed, "Pads: ") {
			current.Pads = strings.TrimPrefix(trimmed, "Pads: ")
		} else if strings.HasPrefix(trimmed, "Status: ") {
			current.Status = strings.TrimPrefix(trimmed, "Status: ")
			current.Found = current.Status == "found"
		} else if strings.HasPrefix(trimmed, "Audio: ") {
			current.Audio = strings.TrimPrefix(trimmed, "Audio: ")
		} else if strings.HasPrefix(trimmed, "Source: ") {
			current.Source = strings.TrimPrefix(trimmed, "Source: ")
		} else if strings.HasPrefix(trimmed, "Tags: ") {
			current.Tags = strings.TrimPrefix(trimmed, "Tags: ")
		} else if strings.HasPrefix(trimmed, "Also used in: ") {
			current.AlsoIn = strings.TrimPrefix(trimmed, "Also used in: ")
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}

	return entries
}
