package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/db"
	"github.com/maxgarvey/mpc_editor/internal/pgm"
	"github.com/maxgarvey/mpc_editor/internal/seq"
)

// sessionPgmRelPath returns the workspace-relative path of the currently loaded session program,
// or "" if no program is loaded or the path cannot be made relative.
func (s *Server) sessionPgmRelPath() string {
	if s.session.FilePath == "" || s.session.Program == nil {
		return ""
	}
	if rel, err := filepath.Rel(s.session.WorkspacePath, s.session.FilePath); err == nil {
		return rel
	}
	return ""
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

// allPadSampleNames returns the first non-empty layer sample name for all 64 pads
// from the explicitly selected program only. Returns empty names if pgmRelPath is "".
func (s *Server) allPadSampleNames(pgmRelPath string) []string {
	names := make([]string, 64)
	if pgmRelPath == "" {
		return names
	}
	var prog *pgm.Program
	if p, err := pgm.OpenProgram(s.resolvePath(pgmRelPath)); err == nil {
		prog = p
	}
	if prog == nil {
		return names
	}
	n := prog.PadCount()
	if n > 64 {
		n = 64
	}
	for i := 0; i < n; i++ {
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

// padSampleNames returns the first non-empty layer sample name for each of the 16 Bank A pads
// from the explicitly selected program (pgmRelPath) or the session program as a fallback.
func (s *Server) padSampleNames(pgmRelPath string) [16]string {
	var names [16]string
	if pgmRelPath == "" {
		return names
	}
	var prog *pgm.Program
	if p, err := pgm.OpenProgram(s.resolvePath(pgmRelPath)); err == nil {
		prog = p
	}
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
			if idx := int(note) - 35; idx >= 0 && idx < 64 {
				return idx
			}
			return 0
		}
	}
	return func(note byte) int {
		if idx := int(note) - 35; idx >= 0 && idx < 64 {
			return idx
		}
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
	pgmRelPath := r.FormValue("pgm")
	action := r.FormValue("action")
	gp := parseGridParams(r)

	sequence, err := seq.Open(seqPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	padForNote := s.padForNoteFunc(pgmRelPath)

	// tickForBarStep converts a 1-indexed display bar and 0-indexed step to a sequence tick.
	tickForBarStep := func(bar, step int) uint32 {
		if bar < 1 {
			bar = 1
		}
		return uint32((bar-1)*gp.TicksPerBar) + uint32(step*gp.TicksPerStep)
	}

	switch action {
	case "toggle":
		bar := parseIntParam(r, "bar", 1)
		padIndex := parseIntParam(r, "pad", 0)
		step := parseIntParam(r, "step", 0)
		velocity := parseIntParam(r, "velocity", 100)
		duration := parseIntParam(r, "duration", 23)
		targetTick := tickForBarStep(bar, step)
		if rawTick := parseIntParam(r, "tick", -1); rawTick >= 0 {
			targetTick = uint32(rawTick)
		}

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
		fromBar := parseIntParam(r, "from_bar", 1)
		fromPad := parseIntParam(r, "from_pad", 0)
		fromStep := parseIntParam(r, "from_step", 0)
		toBar := parseIntParam(r, "to_bar", 1)
		toPad := parseIntParam(r, "to_pad", 0)
		toStep := parseIntParam(r, "to_step", 0)
		fromTick := tickForBarStep(fromBar, fromStep)
		toTick := tickForBarStep(toBar, toStep)
		if rawFrom := parseIntParam(r, "from_tick", -1); rawFrom >= 0 {
			fromTick = uint32(rawFrom)
		}
		if rawTo := parseIntParam(r, "to_tick", -1); rawTo >= 0 {
			toTick = uint32(rawTo)
		}
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
		bar := parseIntParam(r, "bar", 1)
		padIndex := parseIntParam(r, "pad", 0)
		step := parseIntParam(r, "step", 0)
		targetTick := tickForBarStep(bar, step)

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
		bar := parseIntParam(r, "bar", 1)
		padIndex := parseIntParam(r, "pad", 0)
		step := parseIntParam(r, "step", 0)
		velocity := parseIntParam(r, "velocity", 100)
		duration := parseIntParam(r, "duration", 23)
		targetTick := tickForBarStep(bar, step)

		for i, ev := range sequence.Events {
			if ev.Tick == targetTick && padForNote(ev.Note) == padIndex {
				sequence.Events[i].Velocity = byte(velocity)
				sequence.Events[i].Duration = uint16(duration)
				break
			}
		}

	case "multi_delete":
		var targets []struct {
			Pad        int `json:"pad"`
			GlobalStep int `json:"global_step"`
		}
		if err := json.Unmarshal([]byte(r.FormValue("events")), &targets); err != nil {
			http.Error(w, "events: "+err.Error(), http.StatusBadRequest)
			return
		}
		type delKey struct {
			tick uint32
			pad  int
		}
		toDelete := make(map[delKey]int, len(targets))
		for _, t := range targets {
			toDelete[delKey{uint32(t.GlobalStep * gp.TicksPerStep), t.Pad}]++
		}
		staying := make([]seq.Event, 0, len(sequence.Events))
		for _, ev := range sequence.Events {
			k := delKey{ev.Tick, padForNote(ev.Note)}
			if toDelete[k] > 0 {
				toDelete[k]--
				continue
			}
			staying = append(staying, ev)
		}
		sequence.Events = staying

	case "multi_move":
		var targets []struct {
			Pad          int  `json:"pad"`
			GlobalStep   int  `json:"global_step"`
			ToPad        int  `json:"to_pad"`
			ToGlobalStep int  `json:"to_global_step"`
			FromTick     *int `json:"from_tick,omitempty"`
			ToTick       *int `json:"to_tick,omitempty"`
		}
		if err := json.Unmarshal([]byte(r.FormValue("events")), &targets); err != nil {
			http.Error(w, "events: "+err.Error(), http.StatusBadRequest)
			return
		}
		type mvKey struct {
			tick uint32
			pad  int
		}
		type mvDest struct {
			toTick uint32
			toNote byte
		}
		moveMap := make(map[mvKey]mvDest, len(targets))
		destSet := make(map[mvKey]bool, len(targets))
		for _, t := range targets {
			fromTick := uint32(t.GlobalStep * gp.TicksPerStep)
			if t.FromTick != nil {
				fromTick = uint32(*t.FromTick)
			}
			toTick := uint32(t.ToGlobalStep * gp.TicksPerStep)
			if t.ToTick != nil {
				toTick = uint32(*t.ToTick)
			}
			moveMap[mvKey{fromTick, t.Pad}] = mvDest{toTick, s.padToNote(t.ToPad, pgmRelPath)}
			destSet[mvKey{toTick, t.ToPad}] = true
		}
		newEvents := make([]seq.Event, 0, len(sequence.Events))
		for _, ev := range sequence.Events {
			k := mvKey{ev.Tick, padForNote(ev.Note)}
			if dest, ok := moveMap[k]; ok {
				ev.Tick = dest.toTick
				ev.Note = dest.toNote
				delete(moveMap, k)
				newEvents = append(newEvents, ev)
			} else if destSet[k] {
				// discard existing event at a destination that isn't in our source set
				continue
			} else {
				newEvents = append(newEvents, ev)
			}
		}
		sequence.Events = newEvents

	case "multi_update":
		var targets []struct {
			Pad        int `json:"pad"`
			GlobalStep int `json:"global_step"`
		}
		if err := json.Unmarshal([]byte(r.FormValue("events")), &targets); err != nil {
			http.Error(w, "events: "+err.Error(), http.StatusBadRequest)
			return
		}
		velocity := parseIntParam(r, "velocity", 100)
		duration := parseIntParam(r, "duration", 23)
		type updKey struct {
			tick uint32
			pad  int
		}
		toUpdate := make(map[updKey]bool, len(targets))
		for _, t := range targets {
			toUpdate[updKey{uint32(t.GlobalStep * gp.TicksPerStep), t.Pad}] = true
		}
		for i, ev := range sequence.Events {
			if toUpdate[updKey{ev.Tick, padForNote(ev.Note)}] {
				sequence.Events[i].Velocity = byte(velocity)
				sequence.Events[i].Duration = uint16(duration)
			}
		}

	case "quantize":
		bar := parseIntParam(r, "bar", 1)
		padIndex := parseIntParam(r, "pad", 0)
		step := parseIntParam(r, "step", 0)
		qTicks := parseIntParam(r, "quantize_ticks", seq.TicksPerStep)
		if qTicks < 1 {
			qTicks = seq.TicksPerStep
		}
		sourceTick := tickForBarStep(bar, step)
		for i, ev := range sequence.Events {
			if ev.Tick == sourceTick && padForNote(ev.Note) == padIndex {
				sequence.Events[i].Tick = quantizeTick(ev.Tick, qTicks)
				break
			}
		}

	case "multi_quantize":
		var targets []struct {
			Pad        int `json:"pad"`
			GlobalStep int `json:"global_step"`
		}
		if err := json.Unmarshal([]byte(r.FormValue("events")), &targets); err != nil {
			http.Error(w, "events: "+err.Error(), http.StatusBadRequest)
			return
		}
		qTicks := parseIntParam(r, "quantize_ticks", seq.TicksPerStep)
		if qTicks < 1 {
			qTicks = seq.TicksPerStep
		}
		type qKey struct {
			tick uint32
			pad  int
		}
		toQuantize := make(map[qKey]bool, len(targets))
		for _, t := range targets {
			toQuantize[qKey{uint32(t.GlobalStep * gp.TicksPerStep), t.Pad}] = true
		}
		for i, ev := range sequence.Events {
			if toQuantize[qKey{ev.Tick, padForNote(ev.Note)}] {
				sequence.Events[i].Tick = quantizeTick(ev.Tick, qTicks)
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

	grid := seq.BuildGrid(sequence, s.noteToPadMapFor(pgmRelPath), gp)
	names := s.padSampleNames(pgmRelPath)
	for i := range grid.BankAPadRows {
		grid.BankAPadRows[i].SampleName = names[i]
	}

	tsig := r.FormValue("tsig")
	if tsig == "" {
		tsig = "4_4"
	}
	division := r.FormValue("division")
	if division == "" {
		division = "24"
	}
	data := SequenceViewData{
		Path:     seqPath,
		FileName: filepath.Base(seqPath),
		BPM:      sequence.BPM,
		Bars:     sequence.Bars,
		Loop:     sequence.Loop,
		Version:  sequence.Version,
		Grid:     grid,
		PGMPath:  pgmRelPath,
		PGMFiles: s.pgmFilesInWorkspace(),
		TSig:     tsig,
		Division: division,
	}
	s.renderTemplate(w, "sequence_grid.html", data)
}

// quantizeTick rounds tick to the nearest multiple of qTicks.
func quantizeTick(tick uint32, qTicks int) uint32 {
	q := uint32(qTicks)
	return uint32(math.Round(float64(tick)/float64(q))) * q
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
	Path     string
	FileName string // base name only, e.g. "Sequence01.SEQ"
	Error    string
	BPM      float64
	Bars     int
	Loop     bool
	Version  string
	Grid     *seq.StepGrid
	FileID   int64
	Tags     []db.FileTag
	PGMPath  string   // currently selected program for note mapping
	PGMFiles []string // all PGM files in workspace for the picker
	TSig     string   // time signature, e.g. "4_4"
	Division string   // step division in ticks, e.g. "24" for 16th notes
}

// parseGridParams extracts time signature and step division from the request.
// Defaults to 4/4 time with 16th-note (24-tick) steps.
func parseGridParams(r *http.Request) seq.GridParams {
	tsig := r.FormValue("tsig")
	if tsig == "" {
		tsig = "4_4"
	}
	var num, denom int
	if _, err := fmt.Sscanf(tsig, "%d_%d", &num, &denom); err != nil || num < 1 || denom < 1 {
		num, denom = 4, 4
	}
	divStr := r.FormValue("division")
	if divStr == "" {
		divStr = "24"
	}
	divTicks, err := strconv.Atoi(divStr)
	if err != nil || divTicks < 1 {
		divTicks = seq.TicksPerStep
	}
	return seq.NewGridParams(num, denom, divTicks)
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

	pgmRelPath := r.FormValue("pgm")
	gp := parseGridParams(r)
	grid := seq.BuildGrid(sequence, s.noteToPadMapFor(pgmRelPath), gp)
	names := s.padSampleNames(pgmRelPath)
	for i := range grid.BankAPadRows {
		grid.BankAPadRows[i].SampleName = names[i]
	}

	tsig := r.FormValue("tsig")
	if tsig == "" {
		tsig = "4_4"
	}
	division := r.FormValue("division")
	if division == "" {
		division = "24"
	}
	data := SequenceViewData{
		Path:     path,
		FileName: filepath.Base(path),
		BPM:      sequence.BPM,
		Bars:     sequence.Bars,
		Loop:     sequence.Loop,
		Version:  sequence.Version,
		Grid:     grid,
		PGMPath:  pgmRelPath,
		PGMFiles: s.pgmFilesInWorkspace(),
		TSig:     tsig,
		Division: division,
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
	Tick          int `json:"tick"`          // exact tick offset from range start
	DurationTicks int `json:"durationTicks"` // exact duration in ticks
}

// sequenceEventsResponse is the JSON payload for /sequence/events.
type sequenceEventsResponse struct {
	BPM            float64             `json:"bpm"`
	StepsPerBar    int                 `json:"stepsPerBar"`
	TicksPerStep   int                 `json:"ticksPerStep"`
	BeatsPerBar    int                 `json:"beatsPerBar"`
	Bars           int                 `json:"bars"`
	CurrentBar     int                 `json:"currentBar"`
	TotalTicks     int                 `json:"totalTicks"`
	Events         []sequenceEventJSON `json:"events"`
	PadSampleNames []string            `json:"padSampleNames,omitempty"`
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

	gp := parseGridParams(r)

	// bar=0 means "all bars"; any other value plays that specific bar only.
	bar := parseIntParam(r, "bar", 0)
	fileTotalTicks := sequence.Bars * seq.TicksPerBar
	displayBars := fileTotalTicks / gp.TicksPerBar
	if fileTotalTicks%gp.TicksPerBar != 0 {
		displayBars++
	}
	if bar < 0 || bar > displayBars {
		bar = 0
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

	var tickStart, tickEnd uint32
	if bar == 0 {
		tickStart = 0
		tickEnd = uint32(fileTotalTicks)
	} else {
		tickStart = uint32((bar - 1) * gp.TicksPerBar)
		tickEnd = tickStart + uint32(gp.TicksPerBar)
	}

	var events []sequenceEventJSON
	for _, ev := range sequence.Events {
		if ev.Type != seq.EventNoteOn {
			continue
		}
		if ev.Tick < tickStart || ev.Tick >= tickEnd {
			continue
		}
		// When playing all bars, step is the global step index across the whole sequence.
		// When playing a single bar, step is the 0-indexed step within that bar.
		step := int(ev.Tick-tickStart) / gp.TicksPerStep
		durSteps := int(ev.Duration) / gp.TicksPerStep
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
			Tick:          int(ev.Tick - tickStart),
			DurationTicks: int(ev.Duration),
		})
	}

	resp := sequenceEventsResponse{
		BPM:            sequence.BPM,
		StepsPerBar:    gp.StepsPerBar,
		TicksPerStep:   gp.TicksPerStep,
		BeatsPerBar:    gp.BeatsPerBar,
		Bars:           displayBars,
		CurrentBar:     bar,
		TotalTicks:     int(tickEnd - tickStart),
		Events:         events,
		PadSampleNames: s.allPadSampleNames(r.FormValue("pgm")),
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

	pgmRelPath := r.FormValue("pgm")
	gp := parseGridParams(r)
	grid := seq.BuildGrid(sequence, s.noteToPadMapFor(pgmRelPath), gp)
	names := s.padSampleNames(pgmRelPath)
	for i := range grid.BankAPadRows {
		grid.BankAPadRows[i].SampleName = names[i]
	}
	tsig := r.FormValue("tsig")
	if tsig == "" {
		tsig = "4_4"
	}
	division := r.FormValue("division")
	if division == "" {
		division = "24"
	}
	data := SequenceViewData{
		Path:     path,
		FileName: filepath.Base(path),
		BPM:      sequence.BPM,
		Bars:     sequence.Bars,
		Loop:     sequence.Loop,
		Version:  sequence.Version,
		Grid:     grid,
		PGMPath:  pgmRelPath,
		PGMFiles: s.pgmFilesInWorkspace(),
		TSig:     tsig,
		Division: division,
	}
	s.renderTemplate(w, "sequence_grid.html", data)
}

func (s *Server) handleSequenceToggleLoop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := s.resolvePath(r.FormValue("path"))
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	sequence, err := seq.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newLoop := !sequence.Loop
	if err := seq.PatchLoop(path, newLoop); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if newLoop {
		fmt.Fprint(w, `{"loop":true}`)
	} else {
		fmt.Fprint(w, `{"loop":false}`)
	}
}

func (s *Server) handleSequenceNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	if len(name) > 16 {
		http.Error(w, "name too long (max 16 characters)", http.StatusBadRequest)
		return
	}
	dir := strings.TrimSpace(r.FormValue("dir"))
	if dir == "" {
		if s.session.FilePath != "" {
			dir = filepath.Dir(s.session.FilePath)
		} else {
			dir = s.session.WorkspacePath
		}
	} else {
		dir = s.resolvePath(dir)
	}
	if err := s.validateWithinWorkspace(dir); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, fmt.Sprintf("create dir: %v", err), http.StatusInternalServerError)
		return
	}
	seqPath := filepath.Join(dir, name+".SEQ")
	pgmName := ""
	if s.session.FilePath != "" {
		pgmName = filepath.Base(s.session.FilePath)
	}
	data := seq.Create(120.0, 1, name, pgmName, false, nil)
	if err := os.WriteFile(seqPath, data, 0o644); err != nil {
		http.Error(w, fmt.Sprintf("write sequence: %v", err), http.StatusInternalServerError)
		return
	}
	go func() {
		if _, err := s.scanner.ScanWorkspace(s.session.WorkspacePath); err != nil {
			log.Printf("post-sequence-new scan: %v", err)
		}
	}()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"seq_abs":%q}`, seqPath)
}
