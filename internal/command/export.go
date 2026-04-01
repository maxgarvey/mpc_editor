package command

import (
	"fmt"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// ExportResult tracks counts from a program export operation.
type ExportResult struct {
	Expected int
	Exported int
	Errors   []string
}

// HasError returns true if any samples failed to export.
func (r ExportResult) HasError() bool {
	return r.Exported != r.Expected
}

// Report returns a human-readable summary of the export.
func (r ExportResult) Report() string {
	if !r.HasError() {
		return fmt.Sprintf("Exported every %d sample files successfully", r.Exported)
	}
	return fmt.Sprintf("Exported %d sample files out of %d (invalid files or files not found)", r.Exported, r.Expected)
}

// ExportProgram saves the program and copies all referenced samples to destDir.
func ExportProgram(prog *pgm.Program, matrix *pgm.SampleMatrix, destDir, pgmName string) ExportResult {
	var result ExportResult

	// Save the .pgm file
	pgmPath := destDir + "/" + pgmName
	if err := prog.Save(pgmPath); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("save pgm: %v", err))
	}

	// Copy all referenced samples
	refs := matrix.CollectAll()
	for _, ref := range refs {
		result.Expected++
		if err := pgm.CopySample(ref, destDir); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("copy %s: %v", ref.Name, err))
		} else {
			result.Exported++
		}
	}

	return result
}
