package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/db"
)

// ColorPreset is a named color preset for sample display.
type ColorPreset struct {
	Name string
	CSS  string
}

var colorPresets = []ColorPreset{
	{"red", "#e05555"},
	{"orange", "#e07830"},
	{"yellow", "#c8b822"},
	{"green", "#4aaf4a"},
	{"cyan", "#26aaaa"},
	{"blue", "#4888e0"},
	{"purple", "#8855cc"},
	{"pink", "#e05090"},
	{"white", "#cccccc"},
	{"gray", "#777777"},
}

// colorToCSS returns the CSS hex for a preset name, or "" if not found.
func colorToCSS(name string) string {
	for _, p := range colorPresets {
		if p.Name == name {
			return p.CSS
		}
	}
	return ""
}

// sampleKey returns the lookup key for a sample name: lowercase basename without extension.
// This matches the MPC's 16-char-max truncated sample name storage format.
func sampleKey(sampleName string) string {
	base := filepath.Base(sampleName)
	ext := filepath.Ext(base)
	return strings.ToLower(strings.TrimSuffix(base, ext))
}

// sampleColorMap builds a map from sampleKey → CSS color for all colored WAV files.
func (s *Server) sampleColorMap(ctx context.Context) map[string]string {
	rows, err := s.queries.ListWavColored(ctx)
	if err != nil || len(rows) == 0 {
		return nil
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		css := colorToCSS(r.Color)
		if css != "" {
			m[sampleKey(r.Path)] = css
		}
	}
	return m
}

// handleFileColor sets the display color for a WAV file in the catalog.
// POST /file/color — form params: id (file_id), color (preset name or "" to clear)
func (s *Server) handleFileColor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	color := r.FormValue("color")
	if color != "" && colorToCSS(color) == "" {
		http.Error(w, "invalid color", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := s.queries.SetFileColor(ctx, db.SetFileColorParams{
		Color: color,
		ID:    fileID,
	}); err != nil {
		http.Error(w, "failed to set color", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"FileID":  fileID,
		"Color":   color,
		"Presets": colorPresets,
	}
	s.renderTemplate(w, "wav_color_picker.html", data)
}

// LabelSubcat is a subcategory entry in the label taxonomy.
type LabelSubcat struct {
	Name  string
	Color string // preset color name assigned to this subcategory
}

// LabelCategory groups subcategories under a high-level category.
type LabelCategory struct {
	Name    string
	Subcats []LabelSubcat
}

// labelTaxonomy defines the structured label hierarchy for sample classification.
// 8 preset colors are reused across subcategories.
var labelTaxonomy = []LabelCategory{
	{Name: "drum", Subcats: []LabelSubcat{
		{"kick", "red"},
		{"hihat", "orange"},
		{"snare", "yellow"},
		{"crash", "orange"},
		{"clap", "yellow"},
		{"perc", "gray"},
	}},
	{Name: "bass", Subcats: []LabelSubcat{
		{"acoustic", "green"},
		{"electric", "cyan"},
		{"slap", "green"},
		{"synth", "cyan"},
	}},
	{Name: "instrument", Subcats: []LabelSubcat{
		{"strings", "blue"},
		{"guitar", "blue"},
		{"piano", "purple"},
		{"keys", "purple"},
		{"brass", "orange"},
		{"wind", "cyan"},
	}},
	{Name: "loop", Subcats: []LabelSubcat{
		{"drum", "red"},
		{"bass", "green"},
		{"melody", "blue"},
		{"full", "pink"},
	}},
	{Name: "other", Subcats: []LabelSubcat{
		{"vocal", "pink"},
		{"band", "white"},
		{"retro", "gray"},
		{"live", "white"},
		{"sfx", "gray"},
	}},
}

// labelColor returns the preset color name for a given category+subcategory pair.
// Returns "" if not found.
func labelColor(category, subcategory string) (string, bool) {
	for _, cat := range labelTaxonomy {
		if cat.Name == category {
			for _, sub := range cat.Subcats {
				if sub.Name == subcategory {
					return sub.Color, true
				}
			}
			return "", false
		}
	}
	return "", false
}

// handleFileLabel sets the category/subcategory label for a WAV file and auto-assigns
// the corresponding preset color.
// POST /file/label — form params: id (file_id), category, subcategory
func (s *Server) handleFileLabel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	category := r.FormValue("category")
	subcategory := r.FormValue("subcategory")

	var color string
	if category != "" || subcategory != "" {
		var ok bool
		color, ok = labelColor(category, subcategory)
		if !ok {
			http.Error(w, "invalid label", http.StatusBadRequest)
			return
		}
	}

	ctx := r.Context()
	if err := s.queries.SetFileLabel(ctx, db.SetFileLabelParams{
		Category: category, Subcategory: subcategory, ID: fileID,
	}); err != nil {
		http.Error(w, "failed to set label", http.StatusInternalServerError)
		return
	}
	if err := s.queries.SetFileColor(ctx, db.SetFileColorParams{
		Color: color, ID: fileID,
	}); err != nil {
		http.Error(w, "failed to set color", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"FileID":      fileID,
		"Category":    category,
		"Subcategory": subcategory,
		"Taxonomy":    labelTaxonomy,
		"Color":       color,
		"Presets":     colorPresets,
	}
	// Main swap: label picker
	s.renderTemplate(w, "wav_label_picker.html", data)
	// OOB swap: color picker (template checks .OOB to add hx-swap-oob attribute)
	data["OOB"] = true
	s.renderTemplate(w, "wav_color_picker.html", data)
}

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

// handleTagAdd adds a tag to a file.
// POST /file/tags/add — form params: id (file_id), tag (raw string like "kick" or "genre:house")
func (s *Server) handleTagAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	raw := strings.TrimSpace(r.FormValue("tag"))
	if raw == "" {
		http.Error(w, "empty tag", http.StatusBadRequest)
		return
	}

	var key, value string
	if idx := strings.Index(raw, ":"); idx > 0 {
		key = raw[:idx]
		value = raw[idx+1:]
	} else {
		value = raw
	}

	ctx := context.Background()
	_ = s.queries.AddFileTag(ctx, db.AddFileTagParams{
		FileID:   fileID,
		TagKey:   key,
		TagValue: value,
		Auto:     false,
	})

	s.renderTagsSection(w, ctx, fileID)
}

// handleTagRemove removes a tag from a file.
// POST /file/tags/remove — form params: id (file_id), key, value
func (s *Server) handleTagRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid file id", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	_ = s.queries.RemoveFileTag(ctx, db.RemoveFileTagParams{
		FileID:   fileID,
		TagKey:   r.FormValue("key"),
		TagValue: r.FormValue("value"),
	})

	s.renderTagsSection(w, ctx, fileID)
}

// renderTagsSection renders the tags partial for a file.
func (s *Server) renderTagsSection(w http.ResponseWriter, ctx context.Context, fileID int64) {
	tags, _ := s.queries.ListFileTags(ctx, fileID)
	data := map[string]any{
		"FileID": fileID,
		"Tags":   tags,
	}
	s.renderTemplate(w, "tags_section.html", data)
}

// loadTags fetches tags for a file and returns them (for use in detail renderers).
func (s *Server) loadTags(ctx context.Context, fileID int64) []db.FileTag {
	tags, err := s.queries.ListFileTags(ctx, fileID)
	if err != nil {
		return nil
	}
	return tags
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
