package server

import (
	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// Session holds the in-memory state for the current editing session.
// Single-user (local app), so one global session is fine.
type Session struct {
	Program     *pgm.Program
	Matrix      pgm.SampleMatrix
	FilePath    string // path to the current .pgm file (empty if new)
	SelectedPad int    // currently selected pad index (0-63)
	Profile     pgm.Profile
	SampleDir   string // directory where samples are located
	Slicer      *audio.Slicer // active slicer (nil if none)
	SlicerPath  string        // path to WAV loaded in slicer
}

// NewSession creates a session with a blank program.
func NewSession() *Session {
	return &Session{
		Program:     pgm.NewProgram(),
		SelectedPad: 0,
		Profile:     pgm.ProfileMPC1000,
	}
}

// PadName returns the display name for a pad (the first layer's sample name, or empty).
func (s *Session) PadName(index int) string {
	return s.Program.Pad(index).Layer(0).GetSampleName()
}

// HasProgram returns true if a program is loaded (always true since we start with a blank).
func (s *Session) HasProgram() bool {
	return s.Program != nil
}
