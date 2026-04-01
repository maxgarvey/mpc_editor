package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// BatchResult tracks the outcome of a batch program creation.
type BatchResult struct {
	Created  int
	Skipped  int
	Errors   []string
	Programs []string // paths to created .pgm files
}

// Report returns a human-readable summary.
func (r BatchResult) Report() string {
	msg := fmt.Sprintf("Created %d programs", r.Created)
	if r.Skipped > 0 {
		msg += fmt.Sprintf(", skipped %d directories", r.Skipped)
	}
	if len(r.Errors) > 0 {
		msg += fmt.Sprintf(", %d errors", len(r.Errors))
	}
	return msg
}

// BatchCreate walks a directory tree and creates a .pgm file in each directory
// that contains WAV files but no existing .pgm file.
// Samples are assigned one-per-pad starting at pad 0.
func BatchCreate(rootDir string) BatchResult {
	var result BatchResult

	_ = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("walk %s: %v", path, err))
			return nil
		}
		if !info.IsDir() {
			return nil
		}

		// Find WAV files in this directory (non-recursive)
		entries, err := os.ReadDir(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("readdir %s: %v", path, err))
			return nil
		}

		var wavPaths []string
		hasPGM := false
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".wav" {
				wavPaths = append(wavPaths, filepath.Join(path, name))
			}
			if ext == ".pgm" {
				hasPGM = true
			}
		}

		if len(wavPaths) == 0 {
			return nil // no WAVs in this directory
		}
		if hasPGM {
			result.Skipped++
			return nil // already has a .pgm
		}

		// Create a new program
		prog := pgm.NewProgram()
		var matrix pgm.SampleMatrix

		samples, _ := ImportSamples(wavPaths)
		SimpleAssign(prog, &matrix, samples, 0, AssignPerPad)

		// Save using directory name as .pgm filename
		pgmName := filepath.Base(path) + ".pgm"
		pgmPath := filepath.Join(path, pgmName)
		if err := prog.Save(pgmPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("save %s: %v", pgmPath, err))
			return nil
		}

		result.Created++
		result.Programs = append(result.Programs, pgmPath)
		return nil
	})

	return result
}
