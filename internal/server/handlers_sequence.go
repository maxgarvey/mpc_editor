package server

import (
	"encoding/json"
	"net/http"

	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// SequenceViewData holds template data for the sequence step grid page.
type SequenceViewData struct {
	Path       string
	Error      string
	BPM        float64
	Bars       int
	Version    string
	CurrentBar int
	Grid       *seq.StepGrid
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

	grid := seq.BuildGrid(sequence, bar)

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
