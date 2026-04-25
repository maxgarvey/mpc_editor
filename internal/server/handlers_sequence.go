package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/maxgarvey/mpc_editor/internal/db"
	"github.com/maxgarvey/mpc_editor/internal/pgm"
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


// noteToPadMapFor builds a MIDI note → pad index map using the selected or session program.
func (s *Server) noteToPadMapFor(pgmRelPath string) map[int]int {
	var prog *pgm.Program
	if pgmRelPath != "" {
		if p, err := pgm.OpenProgram(s.resolvePath(pgmRelPath)); err == nil {
			prog = p
		}
	}
	if prog == nil {
		prog = s.session.Program
	}
	if prog == nil {
		return nil
	}
	m := make(map[int]int, prog.PadCount())
	for i := 0; i < prog.PadCount() && i < 16; i++ {
		note := prog.Pad(i).GetMIDINote()
		if note == 0 {
			note = 35 + i // uninitialized pad → chromatic default
		}
		m[note] = i
	}
	return m
}

// padSampleNames returns the first non-empty layer sample name for each of the 16 Bank A pads
// from the explicitly selected program (pgmRelPath) or the session program as a fallback.
func (s *Server) padSampleNames(pgmRelPath string) [16]string {
	var prog *pgm.Program
	if pgmRelPath != "" {
		if p, err := pgm.OpenProgram(s.resolvePath(pgmRelPath)); err == nil {
			prog = p
		}
	}
	if prog == nil {
		prog = s.session.Program
	}
	var names [16]string
	if prog == nil {
		return names
	}
	for i := 0; i < 16; i++ {
		pad := prog.Pad(i)
		for j := 0; j < 4; j++ {
			if name := pad.Layer(j).GetSampleName(); name != "" {
				names[i] = name
				break
			}
		}
	}
	return names
}

// padToNote returns the MIDI note for a Bank A pad using the selected or session program,
// falling back to the chromatic default (padIndex + 35).
func (s *Server) padToNote(padIndex int, pgmRelPath string) byte {
	var prog *pgm.Program
	if pgmRelPath != "" {
		if p, err := pgm.OpenProgram(s.resolvePath(pgmRelPath)); err == nil {
			prog = p
		}
	}
	if prog == nil {
		prog = s.session.Program
	}
	if prog != nil && padIndex < prog.PadCount() {
		if note := prog.Pad(padIndex).GetMIDINote(); note != 0 {
			return byte(note)
		}
	}
	return byte(padIndex + 35)
}

// padForNoteFunc returns a closure mapping MIDI note → Bank A pad index (0-15)
// using the selected or session program, with chromatic fallback.
func (s *Server) padForNoteFunc(pgmRelPath string) func(note byte) int {
	var prog *pgm.Program
	if pgmRelPath != "" {
		if p, err := pgm.OpenProgram(s.resolvePath(pgmRelPath)); err == nil {
			prog = p
		}
	}
	if prog == nil {
		prog = s.session.Program
	}
	if prog != nil {
		m := make(map[byte]int, 16)
		for i := 0; i < prog.PadCount() && i < 16; i++ {
			note := prog.Pad(i).GetMIDINote()
			if note == 0 {
				note = 35 + i // uninitialized pad → chromatic default
			}
			m[byte(note)] = i
		}
		return func(note byte) int {
			if idx, ok := m[note]; ok {
				return idx
			}
			if idx := int(note) - 35; idx >= 0 && idx < 16 {
				return idx
			}
			// Match BuildGrid's fallback: unrecognized notes display at A1 and are editable from there.
			return 0
		}
	}
	return func(note byte) int {
		if idx := int(note) - 35; idx >= 0 && idx < 16 {
			return idx
		}
		// Match BuildGrid's fallback: unrecognized notes display at A1 and are editable from there.
		return 0
	}
}

// handleSequenceEventEdit applies a single edit (toggle/move/update) to a sequence
// and re-renders the grid partial.
func (s *Server) handleSequenceEventEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	seqPath := s.resolvePath(r.FormValue("path"))
	if seqPath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	bar := parseIntParam(r, "bar", 1)
	pgmRelPath := r.FormValue("pgm")
	action := r.FormValue("action")

	sequence, err := seq.Open(seqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if bar < 1 {
		bar = 1
	}
	if bar > sequence.Bars {
		bar = sequence.Bars
	}
	barStart := uint32((bar - 1) * seq.TicksPerBar)
	padForNote := s.padForNoteFunc(pgmRelPath)

	switch action {
	case "toggle":
		padIndex := parseIntParam(r, "pad", 0)
		step := parseIntParam(r, "step", 0)
		velocity := parseIntParam(r, "velocity", 100)
		duration := parseIntParam(r, "duration", 23)
		targetTick := barStart + uint32(step*seq.TicksPerStep)

		existing := false
		newEvents := make([]seq.Event, 0, len(sequence.Events))
		for _, ev := range sequence.Events {
			if ev.Tick == targetTick && padForNote(ev.Note) == padIndex {
				existing = true
				continue
			}
			newEvents = append(newEvents, ev)
		}
		if !existing {
			note := s.padToNote(padIndex, pgmRelPath)
			newEvents = append(newEvents, seq.Event{
				Tick:     targetTick,
				Track:    0,
				Type:     seq.EventNoteOn,
				Note:     note,
				Velocity: byte(velocity),
				Duration: uint16(duration),
			})
		}
		sequence.Events = newEvents

	case "move":
		fromPad := parseIntParam(r, "from_pad", 0)
		fromStep := parseIntParam(r, "from_step", 0)
		toPad := parseIntParam(r, "to_pad", 0)
		toStep := parseIntParam(r, "to_step", 0)
		fromTick := barStart + uint32(fromStep*seq.TicksPerStep)
		toTick := barStart + uint32(toStep*seq.TicksPerStep)
		toNote := s.padToNote(toPad, pgmRelPath)

		newEvents := make([]seq.Event, 0, len(sequence.Events))
		moved := false
		for _, ev := range sequence.Events {
			if ev.Tick == fromTick && padForNote(ev.Note) == fromPad && !moved {
				ev.Tick = toTick
				ev.Note = toNote
				moved = true
				newEvents = append(newEvents, ev)
			} else if ev.Tick == toTick && padForNote(ev.Note) == toPad {
				// Discard any existing event at the destination.
				continue
			} else {
				newEvents = append(newEvents, ev)
			}
		}
		sequence.Events = newEvents

	case "delete":
		padIndex := parseIntParam(r, "pad", 0)
		step := parseIntParam(r, "step", 0)
		targetTick := barStart + uint32(step*seq.TicksPerStep)

		newEvents := make([]seq.Event, 0, len(sequence.Events))
		removed := false
		for _, ev := range sequence.Events {
			if ev.Tick == targetTick && padForNote(ev.Note) == padIndex && !removed {
				removed = true
				continue
			}
			newEvents = append(newEvents, ev)
		}
		sequence.Events = newEvents

	case "update":
		padIndex := parseIntParam(r, "pad", 0)
		step := parseIntParam(r, "step", 0)
		velocity := parseIntParam(r, "velocity", 100)
		duration := parseIntParam(r, "duration", 23)
		targetTick := barStart + uint32(step*seq.TicksPerStep)

		for i, ev := range sequence.Events {
			if ev.Tick == targetTick && padForNote(ev.Note) == padIndex {
				sequence.Events[i].Velocity = byte(velocity)
				sequence.Events[i].Duration = uint16(duration)
				break
			}
		}

	default:
		http.Error(w, "unknown action: "+action, http.StatusBadRequest)
		return
	}

	if err := seq.WriteEvents(seqPath, sequence); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-open to confirm round-trip.
	sequence, err = seq.Open(seqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	grid := seq.BuildGrid(sequence, bar, s.noteToPadMapFor(pgmRelPath))
	names := s.padSampleNames(pgmRelPath)
	for i := range grid.BankAPadRows {
		grid.BankAPadRows[i].SampleName = names[i]
	}

	data := SequenceViewData{
		Path:       seqPath,
		FileName:   filepath.Base(seqPath),
		BPM:        sequence.BPM,
		Bars:       sequence.Bars,
		Version:    sequence.Version,
		CurrentBar: bar,
		Grid:       grid,
		PGMPath:    pgmRelPath,
		PGMFiles:   s.pgmFilesInWorkspace(),
	}
	s.renderTemplate(w, "sequence_grid.html", data)
}

// pgmFilesInWorkspace returns the workspace-relative paths of all PGM files in the catalog.
func (s *Server) pgmFilesInWorkspace() []string {
	files, err := s.queries.ListFilesByType(context.Background(), "pgm")
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	return paths
}

// SequenceViewData holds template data for the sequence step grid page.
type SequenceViewData struct {
	Path        string
	FileName    string   // base name only, e.g. "Sequence01.SEQ"
	Error       string
	BPM         float64
	Bars        int
	Version     string
	CurrentBar  int
	Grid        *seq.StepGrid
	FileID      int64
	Tags        []db.FileTag
	PGMPath     string   // currently selected program for note mapping
	PGMFiles    []string // all PGM files in workspace for the picker
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
	names := s.padSampleNames(r.FormValue("pgm"))
	for i := range grid.BankAPadRows {
		grid.BankAPadRows[i].SampleName = names[i]
	}

	data := SequenceViewData{
		Path:       path,
		FileName:   filepath.Base(path),
		BPM:        sequence.BPM,
		Bars:       sequence.Bars,
		Version:    sequence.Version,
		CurrentBar: bar,
		Grid:       grid,
		PGMPath:    r.FormValue("pgm"),
		PGMFiles:   s.pgmFilesInWorkspace(),
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

	noteToPad := s.noteToPadMapFor(r.FormValue("pgm"))
	padForNote := func(note int) int {
		if noteToPad != nil {
			if idx, ok := noteToPad[note]; ok {
				return idx
			}
		}
		// Chromatic fallback: MPC default assigns note 35=A1, 36=A2, ... 98=D16.
		if idx := note - 35; idx >= 0 && idx < 64 {
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

func (s *Server) handleSequenceUpdate(w http.ResponseWriter, r *http.Request) {
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

	newBPM := sequence.BPM
	if v, err := strconv.ParseFloat(r.FormValue("bpm"), 64); err == nil && v >= 20 && v <= 300 {
		newBPM = v
	}
	newBars := sequence.Bars
	if v, err := strconv.Atoi(r.FormValue("bars")); err == nil && v >= 1 && v <= 999 {
		newBars = v
	}

	if newBPM != sequence.BPM || newBars != sequence.Bars {
		if err := seq.PatchFile(path, newBPM, newBars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Re-read the patched file so our in-memory state matches disk exactly.
		if s2, err2 := seq.Open(path); err2 == nil {
			sequence = s2
		} else {
			sequence.BPM = newBPM
			sequence.Bars = newBars
		}
		// Update DB catalog and auto-tags.
		workspace := s.session.WorkspacePath
		if relPath, err2 := filepath.Rel(workspace, path); err2 == nil {
			if f, err2 := s.queries.GetFileByPath(r.Context(), relPath); err2 == nil {
				_ = s.queries.UpsertSeqMeta(r.Context(), db.UpsertSeqMetaParams{
					FileID:  f.ID,
					Bpm:     sequence.BPM,
					Bars:    int64(sequence.Bars),
					Version: sequence.Version,
				})
				_ = s.queries.RemoveAutoTags(r.Context(), f.ID)
				if sequence.BPM > 0 {
					_ = s.queries.AddFileTag(r.Context(), db.AddFileTagParams{
						FileID: f.ID, TagKey: "bpm",
						TagValue: fmt.Sprintf("%d", int(math.Round(sequence.BPM))),
						Auto:     1,
					})
				}
				if sequence.Bars > 0 {
					_ = s.queries.AddFileTag(r.Context(), db.AddFileTagParams{
						FileID: f.ID, TagKey: "bars",
						TagValue: fmt.Sprintf("%d", sequence.Bars),
						Auto:     1,
					})
				}
			}
		}
	}

	bar := parseIntParam(r, "bar", 1)
	if bar < 1 {
		bar = 1
	}
	if bar > sequence.Bars {
		bar = sequence.Bars
	}

	grid := seq.BuildGrid(sequence, bar, s.noteToPadMap())
	names := s.padSampleNames(r.FormValue("pgm"))
	for i := range grid.BankAPadRows {
		grid.BankAPadRows[i].SampleName = names[i]
	}
	data := SequenceViewData{
		Path:       path,
		FileName:   filepath.Base(path),
		BPM:        sequence.BPM,
		Bars:       sequence.Bars,
		Version:    sequence.Version,
		CurrentBar: bar,
		Grid:       grid,
		PGMPath:    r.FormValue("pgm"),
		PGMFiles:   s.pgmFilesInWorkspace(),
	}
	s.renderTemplate(w, "sequence_grid.html", data)
}
