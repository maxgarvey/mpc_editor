package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/midi"
)

func (s *Server) handleSlicerPage(w http.ResponseWriter, r *http.Request) {
	data := s.slicerData()
	s.renderTemplate(w, "slicer_page.html", data)
}

func (s *Server) handleSlicerLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.FormValue("path")
	if path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	sample, err := audio.OpenWAV(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("open WAV: %v", err), http.StatusBadRequest)
		return
	}

	s.session.Slicer = audio.NewSlicer(sample)
	s.session.SlicerPath = path

	s.session.Prefs.LastWAVPath = path
	if err := SavePreferences(s.session.Prefs); err != nil {
		log.Printf("save preferences: %v", err)
	}

	data := s.slicerData()
	s.renderTemplate(w, "slicer.html", data)
}

func (s *Server) handleSlicerWaveform(w http.ResponseWriter, r *http.Request) {
	if s.session.Slicer == nil {
		http.Error(w, "no slicer active", http.StatusNotFound)
		return
	}

	buckets := parseIntParam(r, "width", 2000)
	if buckets < 100 {
		buckets = 100
	}
	if buckets > 4000 {
		buckets = 4000
	}

	slicer := s.session.Slicer
	channels := slicer.Channels()
	frameLen := slicer.Sample().FrameLength

	// Downsample to peaks: for each bucket compute min/max
	type Peak struct {
		Min int `json:"min"`
		Max int `json:"max"`
	}

	downsample := func(samples []int) []Peak {
		peaks := make([]Peak, buckets)
		samplesPerBucket := float64(len(samples)) / float64(buckets)
		for i := range peaks {
			start := int(float64(i) * samplesPerBucket)
			end := int(float64(i+1) * samplesPerBucket)
			if end > len(samples) {
				end = len(samples)
			}
			if start >= end {
				continue
			}
			mn, mx := samples[start], samples[start]
			for _, v := range samples[start:end] {
				if v < mn {
					mn = v
				}
				if v > mx {
					mx = v
				}
			}
			peaks[i] = Peak{Min: mn, Max: mx}
		}
		return peaks
	}

	type WaveformData struct {
		Channels    [][]Peak `json:"channels"`
		Markers     []int    `json:"markers"`
		Selected    int      `json:"selected"`
		SampleRate  int      `json:"sampleRate"`
		FrameLength int      `json:"frameLength"`
		Sensitivity int      `json:"sensitivity"`
		Tempo       string   `json:"tempo"`
		Duration    string   `json:"duration"`
		MarkerCount int      `json:"markerCount"`
	}

	var chPeaks [][]Peak
	for _, ch := range channels {
		chPeaks = append(chPeaks, downsample(ch))
	}

	data := WaveformData{
		Channels:    chPeaks,
		Markers:     slicer.Markers.Locations(),
		Selected:    slicer.Markers.SelectedIndex(),
		SampleRate:  slicer.Sample().Format.SampleRate,
		FrameLength: frameLen,
		Sensitivity: slicer.GetSensitivity(),
		Tempo:       formatTempo(slicer.Markers.Tempo(8)),
		Duration:    fmt.Sprintf("%.2fs", slicer.Markers.Duration()),
		MarkerCount: slicer.Markers.Size(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) handleSlicerSensitivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.session.Slicer == nil {
		http.Error(w, "no slicer active", http.StatusNotFound)
		return
	}

	sens := parseIntParam(r, "sensitivity", 130)
	s.session.Slicer.SetSensitivity(sens)

	data := s.slicerData()
	s.renderTemplate(w, "slicer.html", data)
}

func (s *Server) handleSlicerMarker(w http.ResponseWriter, r *http.Request) {
	if s.session.Slicer == nil {
		http.Error(w, "no slicer active", http.StatusNotFound)
		return
	}

	markers := s.session.Slicer.Markers

	// Parse action from URL: /slicer/marker/{action}
	action := strings.TrimPrefix(r.URL.Path, "/slicer/marker/")
	action = strings.TrimRight(action, "/")

	switch action {
	case "select":
		idx := parseIntParam(r, "index", 0)
		// SelectMarker takes a shift, so compute shift from current
		shift := idx - markers.SelectedIndex()
		markers.SelectMarker(shift)
	case "next":
		markers.SelectMarker(1)
	case "prev":
		markers.SelectMarker(-1)
	case "delete":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		markers.DeleteSelected()
	case "insert":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		markers.InsertAtMidpoint()
	case "nudge":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ticks := parseIntParam(r, "ticks", 100)
		markers.NudgeMarker(ticks)
	default:
		http.Error(w, "unknown marker action: "+action, http.StatusBadRequest)
		return
	}

	data := s.slicerData()
	s.renderTemplate(w, "slicer.html", data)
}

func (s *Server) handleSlicerExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.session.Slicer == nil {
		http.Error(w, "no slicer active", http.StatusNotFound)
		return
	}

	dir := r.FormValue("dir")
	if dir == "" {
		// Default to same directory as the loaded WAV
		dir = filepath.Dir(s.session.SlicerPath)
	}
	prefix := r.FormValue("prefix")
	if prefix == "" {
		base := strings.TrimSuffix(filepath.Base(s.session.SlicerPath), filepath.Ext(s.session.SlicerPath))
		prefix = base + "_"
	}

	slicer := s.session.Slicer

	// Export WAV slices
	paths, err := slicer.ExportSlices(dir, prefix)
	if err != nil {
		http.Error(w, fmt.Sprintf("export slices: %v", err), http.StatusInternalServerError)
		return
	}

	// Export MIDI
	tempo := slicer.Markers.Tempo(8)
	if tempo > 0 {
		seq := midi.BuildFromMarkers(
			slicer.Markers.Locations(),
			tempo,
			slicer.Sample().Format.SampleRate,
			midi.DefaultPPQ,
		)
		midiPath := filepath.Join(dir, prefix+"slices.mid")
		seq.Save(midiPath)
		paths = append(paths, midiPath)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"exported": len(paths),
		"dir":      dir,
		"files":    paths,
	})
}

// slicerData builds template data for the slicer page.
func (s *Server) slicerData() map[string]any {
	path := s.session.SlicerPath
	if path == "" {
		path = s.session.Prefs.LastWAVPath
	}
	data := map[string]any{
		"Active":    s.session.Slicer != nil,
		"Path":      path,
		"SampleDir": s.session.SampleDir,
	}

	if s.session.Slicer != nil {
		slicer := s.session.Slicer

		data["Sensitivity"] = slicer.GetSensitivity()
		data["MarkerCount"] = slicer.Markers.Size()
		data["Selected"] = slicer.Markers.SelectedIndex()
		data["Duration"] = fmt.Sprintf("%.2fs", slicer.Markers.Duration())
		data["Tempo"] = formatTempo(slicer.Markers.Tempo(8))
		data["FrameLength"] = slicer.Sample().FrameLength
		data["SampleRate"] = slicer.Sample().Format.SampleRate
		data["Channels"] = slicer.Sample().Format.Channels

		if slicer.Markers.Size() > 0 {
			sel := slicer.Markers.SelectedMarker()
			if sel != nil {
				data["SelectedLocation"] = sel.Location
			}
		}
	}

	return data
}

// formatTempo returns a human-readable BPM string, or empty if tempo is 0.
func formatTempo(tempo float64) string {
	if tempo > 0 {
		return fmt.Sprintf("%.2f BPM", tempo)
	}
	return ""
}
