package server

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Preferences stores user settings that persist across sessions.
type Preferences struct {
	Profile      string `json:"profile"`       // "MPC1000" or "MPC500"
	LastPGMPath  string `json:"lastPgmPath"`   // last opened .pgm path
	LastWAVPath  string `json:"lastWavPath"`   // last loaded WAV in slicer
	AuditionMode string `json:"auditionMode"`  // "layer0", "none"
}

// DefaultPreferences returns the default preferences.
func DefaultPreferences() Preferences {
	return Preferences{
		Profile:      "MPC1000",
		AuditionMode: "layer0",
	}
}

func prefsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mpc_editor", "preferences.json")
}

// LoadPreferences reads preferences from disk, or returns defaults.
func LoadPreferences() Preferences {
	p := DefaultPreferences()
	path := prefsPath()
	if path == "" {
		return p
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}
	json.Unmarshal(data, &p)
	return p
}

// SavePreferences writes preferences to disk.
func SavePreferences(p Preferences) error {
	path := prefsPath()
	if path == "" {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
