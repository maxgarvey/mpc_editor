package server

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

func (s *Server) handlePadSelect(w http.ResponseWriter, r *http.Request) {
	// Extract pad index from /pad/{index}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/pad/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "pad index required", http.StatusBadRequest)
		return
	}

	idx, err := strconv.Atoi(parts[0])
	if err != nil || idx < 0 || idx >= 64 {
		http.Error(w, "invalid pad index", http.StatusBadRequest)
		return
	}

	s.session.SelectedPad = idx

	// Return pad params partial
	s.renderTemplate(w, "pad_params.html", s.padParamsData())
}

func (s *Server) handlePadParams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	idx := s.session.SelectedPad
	pad := s.session.Program.Pad(idx)

	if v := r.FormValue("voice_overlap"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			pad.SetVoiceOverlap(n)
		}
	}
	if v := r.FormValue("mute_group"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			pad.SetMuteGroup(n)
		}
	}
	if v := r.FormValue("midi_note"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			pad.SetMIDINote(n)
		}
	}

	// Envelope
	env := pad.Envelope()
	if v := r.FormValue("attack"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			env.SetAttack(n)
		}
	}
	if v := r.FormValue("decay"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			env.SetDecay(n)
		}
	}
	if v := r.FormValue("decay_mode"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			env.SetDecayMode(n)
		}
	}

	// Filter 1
	f1 := pad.Filter1()
	if v := r.FormValue("filter1_type"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f1.SetType(n)
		}
	}
	if v := r.FormValue("filter1_freq"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f1.SetFrequency(n)
		}
	}
	if v := r.FormValue("filter1_res"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f1.SetResonance(n)
		}
	}

	// Mixer
	mx := pad.Mixer()
	if v := r.FormValue("mixer_level"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			mx.SetLevel(n)
		}
	}
	if v := r.FormValue("mixer_pan"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			mx.SetPan(n)
		}
	}

	if err := s.session.Program.Save(s.session.FilePath); err != nil {
		log.Printf("save program: %v", err)
	}

	s.renderTemplate(w, "pad_params.html", s.padParamsData())
}

func (s *Server) handleLayerUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Parse layer index from /pad/layer/{index}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/pad/layer/"), "/")
	if len(parts) == 0 {
		http.Error(w, "layer index required", http.StatusBadRequest)
		return
	}
	layerIdx, err := strconv.Atoi(parts[0])
	if err != nil || layerIdx < 0 || layerIdx >= 4 {
		http.Error(w, "invalid layer index", http.StatusBadRequest)
		return
	}

	padIdx := s.session.SelectedPad
	layer := s.session.Program.Pad(padIdx).Layer(layerIdx)

	if _, ok := r.Form["sample_name"]; ok {
		name := r.FormValue("sample_name")
		if len(name) > 16 {
			name = name[:16]
		}
		_ = layer.SetSampleName(name)
		if name == "" {
			s.session.Matrix.Set(padIdx, layerIdx, nil)
		} else {
			ref := pgm.FindSampleInDirs(name, s.session.SampleDir, s.session.WorkspacePath)
			s.session.Matrix.Set(padIdx, layerIdx, &ref)
		}
	}
	if v := r.FormValue("level"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			layer.SetLevel(n)
		}
	}
	if v := r.FormValue("tuning"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			layer.SetTuning(f)
		}
	}
	if v := r.FormValue("play_mode"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			layer.SetPlayMode(n)
		}
	}

	if err := s.session.Program.Save(s.session.FilePath); err != nil {
		log.Printf("save program: %v", err)
	}

	s.renderTemplate(w, "pad_params.html", s.padParamsData())
}

func (s *Server) handlePadGrid(w http.ResponseWriter, r *http.Request) {
	bank := parseIntParam(r, "bank", s.session.SelectedPad/16)
	s.renderTemplate(w, "pad_grid.html", s.padGridData(bank))
}

func (s *Server) handlePadParamsPartial(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "pad_params.html", s.padParamsData())
}
