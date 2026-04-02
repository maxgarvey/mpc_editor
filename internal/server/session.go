package server

import (
	"context"
	"log"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/db"
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
	SampleDir   string        // directory where samples are located
	Slicer      *audio.Slicer // active slicer (nil if none)
	SlicerPath  string        // path to WAV loaded in slicer
	Prefs       Preferences
}

// NewSession creates a session with a blank program and loads saved preferences.
func NewSession(queries *db.Queries) *Session {
	prefs := loadPrefsFromDB(queries)
	profile := pgm.ProfileMPC1000
	if prefs.Profile == "MPC500" {
		profile = pgm.ProfileMPC500
	}
	return &Session{
		Program:     pgm.NewProgram(),
		SelectedPad: 0,
		Profile:     profile,
		Prefs:       prefs,
	}
}

// loadPrefsFromDB reads preferences from the database, falling back to defaults.
func loadPrefsFromDB(queries *db.Queries) Preferences {
	row, err := queries.GetPreferences(context.Background())
	if err != nil {
		log.Printf("load preferences from db: %v", err)
		return DefaultPreferences()
	}
	return Preferences{
		Profile:      row.Profile,
		LastPGMPath:  row.LastPgmPath,
		LastWAVPath:  row.LastWavPath,
		AuditionMode: row.AuditionMode,
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
