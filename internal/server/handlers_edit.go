package server

import (
	"log"
	"net/http"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// handleRemoveAllSamples clears all sample names from all pads/layers.
func (s *Server) handleRemoveAllSamples(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prog := s.session.Program
	for i := 0; i < prog.PadCount(); i++ {
		pad := prog.Pad(i)
		for j := 0; j < 4; j++ {
			pad.Layer(j).SetSampleName("")
		}
	}
	s.session.Matrix.Clear()

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleChromaticLayout assigns consecutive MIDI notes to pads (B0, C1, C#1, ...).
func (s *Server) handleChromaticLayout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prog := s.session.Program
	for i := 0; i < prog.PadCount(); i++ {
		prog.Pad(i).SetMIDINote(35 + i) // B0 = 35, C1 = 36, ...
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleCopySettingsToAll copies the selected pad's parameters to all other pads.
// Excludes: sample name, tuning, MIDI note.
func (s *Server) handleCopySettingsToAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prog := s.session.Program
	src := prog.Pad(s.session.SelectedPad)

	ignore := map[string]bool{
		pgm.LayerSampleName.Label: true,
		pgm.LayerTuning.Label:     true,
	}

	for i := 0; i < prog.PadCount(); i++ {
		if i == s.session.SelectedPad {
			continue
		}
		dst := prog.Pad(i)
		dst.CopyFrom(src, ignore)
		// Preserve original MIDI note
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// handleProfileSwitch changes the active MPC profile.
func (s *Server) handleProfileSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	profile := r.FormValue("profile")
	switch profile {
	case "MPC500":
		s.session.Profile = pgm.ProfileMPC500
	default:
		s.session.Profile = pgm.ProfileMPC1000
	}

	s.session.Prefs.Profile = s.session.Profile.Name
	if err := SavePreferences(s.session.Prefs); err != nil {
		log.Printf("save preferences: %v", err)
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}
