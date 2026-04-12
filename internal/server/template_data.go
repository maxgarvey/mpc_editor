package server

// PadInfo holds display data for a single pad in the grid.
type PadInfo struct {
	Index      int
	Name       string
	Selected   bool
	Display    int
	HasSample  bool
	LayerCount int
}

// padGridData builds template data for the pad grid.
func (s *Server) padGridData(bank int) map[string]any {
	start := bank * 16
	pads := make([]PadInfo, 16)
	for i := range pads {
		idx := start + i
		name := s.session.PadName(idx)
		layerCount := 0
		for j := range 4 {
			layerName := s.session.Program.Pad(idx).Layer(j).GetSampleName()
			if layerName != "" {
				layerCount++
			}
		}
		pads[i] = PadInfo{
			Index:      idx,
			Name:       name,
			Selected:   idx == s.session.SelectedPad,
			Display:    i + 1,
			HasSample:  name != "",
			LayerCount: layerCount,
		}
	}

	return map[string]any{
		"Pads":        pads,
		"Bank":        bank,
		"BankLabel":   string(rune('A' + bank)),
		"SelectedPad": s.session.SelectedPad,
	}
}

// LayerInfo holds display data for a sample layer.
type LayerInfo struct {
	Index      int
	SampleName string
	Level      int
	Tuning     float64
	PlayMode   int
}

// padParamsData builds template data for the pad parameter panel.
func (s *Server) padParamsData() map[string]any {
	idx := s.session.SelectedPad
	pad := s.session.Program.Pad(idx)

	layers := make([]LayerInfo, 4)
	for i := range layers {
		l := pad.Layer(i)
		layers[i] = LayerInfo{
			Index:      i,
			SampleName: l.GetSampleName(),
			Level:      l.GetLevel(),
			Tuning:     l.GetTuning(),
			PlayMode:   l.GetPlayMode(),
		}
	}

	layerCountTotal := 0
	for _, l := range layers {
		if l.SampleName != "" {
			layerCountTotal++
		}
	}

	return map[string]any{
		"PadIndex":        idx,
		"PadDisplay":      (idx % 16) + 1,
		"BankLabel":       string(rune('A' + idx/16)),
		"Layers":          layers,
		"LayerCountTotal": layerCountTotal,
		"VoiceOverlap": pad.GetVoiceOverlap(),
		"MuteGroup":    pad.GetMuteGroup(),
		"MIDINote":     pad.GetMIDINote(),
		"Attack":       pad.Envelope().GetAttack(),
		"Decay":        pad.Envelope().GetDecay(),
		"DecayMode":    pad.Envelope().GetDecayMode(),
		"Filter1Type":  pad.Filter1().GetType(),
		"Filter1Freq":  pad.Filter1().GetFrequency(),
		"Filter1Res":   pad.Filter1().GetResonance(),
		"Filter2Type":  pad.Filter2().GetType(),
		"Filter2Freq":  pad.Filter2().GetFrequency(),
		"Filter2Res":   pad.Filter2().GetResonance(),
		"MixerLevel":   pad.Mixer().GetLevel(),
		"MixerPan":     pad.Mixer().GetPan(),
		"MixerOutput":  pad.Mixer().GetOutput(),
	}
}
