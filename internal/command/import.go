package command

import (
	"fmt"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// ImportResult tracks counts from a sample import operation.
type ImportResult struct {
	Imported int
	Renamed  int
	Rejected int
}

// HasError returns true if any files were renamed or rejected.
func (r ImportResult) HasError() bool {
	return r.Renamed > 0 || r.Rejected > 0
}

// Report returns a human-readable summary of the import.
func (r ImportResult) Report() string {
	if !r.HasError() {
		return fmt.Sprintf("Imported %d files", r.Imported)
	}
	msg := fmt.Sprintf("Imported %d files, of which ", r.Imported)
	if r.Renamed > 0 {
		msg += fmt.Sprintf("%d have been renamed (name too long)", r.Renamed)
	}
	if r.Rejected > 0 {
		if r.Renamed > 0 {
			msg += ", and "
		}
		msg += fmt.Sprintf("%d have been ignored (invalid format)", r.Rejected)
	}
	return msg
}

// ImportSamples validates a list of file paths and returns valid SampleRefs.
// Paths with non-.wav extensions are rejected. Names longer than 16 chars are truncated.
func ImportSamples(paths []string) ([]*pgm.SampleRef, ImportResult) {
	var samples []*pgm.SampleRef
	var result ImportResult

	for _, path := range paths {
		ref := pgm.ImportSample(path)
		result.Imported++

		switch ref.Status {
		case pgm.SampleRejected:
			result.Rejected++
		case pgm.SampleRenamed:
			result.Renamed++
			r := ref // copy
			samples = append(samples, &r)
		case pgm.SampleOK:
			r := ref // copy
			samples = append(samples, &r)
		}
	}
	return samples, result
}
