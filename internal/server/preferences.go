package server

// Preferences stores user settings that persist across sessions.
type Preferences struct {
	Profile        string `json:"profile"`        // "MPC1000" or "MPC500"
	LastPGMPath    string `json:"lastPgmPath"`    // last opened .pgm path
	LastWAVPath    string `json:"lastWavPath"`    // last loaded WAV in slicer
	AuditionMode   string `json:"auditionMode"`   // "layer0", "none"
	WorkspacePath  string `json:"workspacePath"`  // root directory for MPC files
	LastDetailPath string `json:"lastDetailPath"` // last viewed file in detail panel
}

// DefaultPreferences returns the default preferences.
func DefaultPreferences() Preferences {
	return Preferences{
		Profile:      "MPC1000",
		AuditionMode: "layer0",
	}
}
