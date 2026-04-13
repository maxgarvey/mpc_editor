package server

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/db"
	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// Session holds the in-memory state for the current editing session.
// Single-user (local app), so one global session is fine.
type Session struct {
	Program            *pgm.Program
	Matrix             pgm.SampleMatrix
	FilePath           string // path to the current .pgm file (empty if new)
	SelectedPad        int    // currently selected pad index (0-63)
	Profile            pgm.Profile
	SampleDir          string        // directory where samples are located
	WorkspacePath      string        // root directory for MPC files
	Slicer             *audio.Slicer // active slicer (nil if none)
	SlicerPath         string        // path to WAV loaded in slicer
	Prefs              Preferences
	SelectedDetailPath string // path of the file shown in the detail panel
}

// NewSession creates a session with a blank program and loads saved preferences.
func NewSession(queries *db.Queries) *Session {
	prefs := loadPrefsFromDB(queries)
	profile := pgm.ProfileMPC1000
	if prefs.Profile == "MPC500" {
		profile = pgm.ProfileMPC500
	}

	workspace := prefs.WorkspacePath
	if workspace == "" {
		workspace = defaultWorkspacePath()
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		log.Printf("create workspace %s: %v", workspace, err)
	}
	ensureWorkspaceDirs(workspace)

	sess := &Session{
		Program:       pgm.NewProgram(),
		SelectedPad:   0,
		Profile:       profile,
		WorkspacePath: workspace,
		SampleDir:     workspace,
		Prefs:         prefs,
	}

	// Restore the last opened program if the file still exists.
	if prefs.LastPGMPath != "" {
		if _, err := os.Stat(prefs.LastPGMPath); err == nil {
			if prog, err := pgm.OpenProgram(prefs.LastPGMPath); err == nil {
				sess.Program = prog
				sess.FilePath = prefs.LastPGMPath
				pgmDir := filepath.Dir(prefs.LastPGMPath)
				sess.SampleDir = pgmDir
				samplesDir := filepath.Join(pgmDir, "samples")
				// Populate sample matrix from program.
				for i := 0; i < 64; i++ {
					pad := prog.Pad(i)
					for j := 0; j < 4; j++ {
						name := pad.Layer(j).GetSampleName()
						if name != "" {
							ref := pgm.FindSampleInDirs(name, samplesDir, pgmDir, workspace)
							sess.Matrix.Set(i, j, &ref)
						}
					}
				}
				log.Printf("restored program: %s", prefs.LastPGMPath)
			} else {
				log.Printf("restore program %s: %v", prefs.LastPGMPath, err)
			}
		}
	}

	// Restore the last viewed detail path.
	if prefs.LastDetailPath != "" {
		sess.SelectedDetailPath = prefs.LastDetailPath
	}

	return sess
}

// ensureWorkspaceDirs creates the standard workspace subdirectories if they don't exist.
func ensureWorkspaceDirs(workspace string) {
	for _, sub := range []string{"sample_library", "programs"} {
		dir := filepath.Join(workspace, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("create workspace dir %s: %v", dir, err)
		}
	}
}

func defaultWorkspacePath() string {
	// Default to the "workspace" subdirectory next to the binary.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if !isTempDir(dir) {
			return filepath.Join(dir, "workspace")
		}
	}
	// Fallback: "workspace" subdirectory in the current working directory.
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, "workspace")
	}
	return filepath.Join(".", "workspace")
}

func isTempDir(dir string) bool {
	tmp := os.TempDir()
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absTmp, err := filepath.Abs(tmp)
	if err != nil {
		return false
	}
	return len(abs) >= len(absTmp) && abs[:len(absTmp)] == absTmp
}

// loadPrefsFromDB reads preferences from the database, falling back to defaults.
func loadPrefsFromDB(queries *db.Queries) Preferences {
	row, err := queries.GetPreferences(context.Background())
	if err != nil {
		log.Printf("load preferences from db: %v", err)
		return DefaultPreferences()
	}
	return Preferences{
		Profile:        row.Profile,
		LastPGMPath:    row.LastPgmPath,
		LastWAVPath:    row.LastWavPath,
		AuditionMode:   row.AuditionMode,
		WorkspacePath:  row.WorkspacePath,
		LastDetailPath: row.LastDetailPath,
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
