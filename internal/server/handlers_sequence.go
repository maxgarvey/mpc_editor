package server

import (
	"encoding/json"
	"net/http"

	"github.com/maxgarvey/mpc_editor/internal/db"
	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// noteToPadMap builds a MIDI note → pad index map from the session's loaded program.
// Returns nil if no program is loaded (callers use the chromatic fallback).
func (s *Server) noteToPadMap() map[int]int {
	if s.session.Program == nil {
		return nil
	}
	prog := s.session.Program
	m := make(map[int]int, prog.PadCount())
	for i := 0; i < prog.PadCount(); i++ {
		m[prog.Pad(i).GetMIDINote()] = i
	}
	return m
}

// SequenceViewData holds template data for the sequence step grid page.
type SequenceViewData struct {
	Path       string
	Error      string
	BPM        float64
	Bars       int
	Version    string
	CurrentBar int
	Grid       *seq.StepGrid
	FileID     int64
	Tags       []db.FileTag
}

func (s *Server) handleSequencePage(w http.ResponseWriter, r *http.Request) {
	path := s.resolvePath(r.FormValue("path"))
	if path == "" {
		s.renderTemplate(w, "sequence_page.html", SequenceViewData{Error: "path is required"})
		return
	}

	sequence, err := seq.Open(path)
	if err != nil {
		s.renderTemplate(w, "sequence_page.html", SequenceViewData{
			Path:  path,
			Error: err.Error(),
		})
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

	// If HTMX request, render just the grid partial.
	if r.Header.Get("HX-Request") == "true" {
		s.renderTemplate(w, "sequence_grid.html", data)
		return
	}

	s.renderTemplate(w, "sequence_page.html", data)
}

// sequenceEventJSON is one note event for the JSON playback endpoint.
type sequenceEventJSON struct {
	Step          int `json:"step"`
	Track         int `json:"track"`
	Note          int `json:"note"`
	PadIndex      int `json:"padIndex"`
	Velocity      int `json:"velocity"`
	DurationSteps int `json:"durationSteps"`
}

// sequenceEventsResponse is the JSON payload for /sequence/events.
type sequenceEventsResponse struct {
	BPM          float64             `json:"bpm"`
	StepsPerBar  int                 `json:"stepsPerBar"`
	TicksPerStep int                 `json:"ticksPerStep"`
	Bars         int                 `json:"bars"`
	CurrentBar   int                 `json:"currentBar"`
	Events       []sequenceEventJSON `json:"events"`
}

func (s *Server) handleSequenceEvents(w http.ResponseWriter, r *http.Request) {
	path := s.resolvePath(r.FormValue("path"))
	if path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	sequence, err := seq.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bar := parseIntParam(r, "bar", 1)
	if bar < 1 {
		bar = 1
	}
	if bar > sequence.Bars {
		bar = sequence.Bars
	}

	noteToPad := s.noteToPadMap()
	padForNote := func(note int) int {
		if noteToPad != nil {
			if idx, ok := noteToPad[note]; ok {
				return idx
			}
		}
		// MPC SEQ files store pad numbers 1-indexed (1=A1, 5=A5, 9=A9 etc).
		if idx := note - 1; idx >= 0 && idx < 64 {
			return idx
		}
		return 0
	}

	barStart := uint32((bar - 1) * seq.TicksPerBar)
	barEnd := barStart + uint32(seq.TicksPerBar)

	var events []sequenceEventJSON
	for _, ev := range sequence.Events {
		if ev.Type != seq.EventNoteOn {
			continue
		}
		if ev.Tick < barStart || ev.Tick >= barEnd {
			continue
		}
		step := int(ev.Tick-barStart) / seq.TicksPerStep
		durSteps := int(ev.Duration) / seq.TicksPerStep
		if durSteps < 1 {
			durSteps = 1
		}
		events = append(events, sequenceEventJSON{
			Step:          step,
			Track:         ev.Track,
			Note:          int(ev.Note),
			PadIndex:      padForNote(int(ev.Note)),
			Velocity:      int(ev.Velocity),
			DurationSteps: durSteps,
		})
	}

	resp := sequenceEventsResponse{
		BPM:          sequence.BPM,
		StepsPerBar:  seq.StepsPerBar,
		TicksPerStep: seq.TicksPerStep,
		Bars:         sequence.Bars,
		CurrentBar:   bar,
		Events:       events,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
