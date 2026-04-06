package server

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// resolvePath resolves a user-provided path. Empty strings are returned as-is.
// Absolute paths are returned as-is. Relative paths are joined with the workspace directory.
func (s *Server) resolvePath(input string) string {
	if input == "" {
		return ""
	}
	if filepath.IsAbs(input) {
		return input
	}
	if s.session.WorkspacePath != "" {
		return filepath.Join(s.session.WorkspacePath, input)
	}
	return input
}

// validateWithinWorkspace checks that target is inside the workspace directory.
// Returns an error if the path escapes the workspace boundary.
func (s *Server) validateWithinWorkspace(target string) error {
	workspace, err := filepath.Abs(filepath.Clean(s.session.WorkspacePath))
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	abs, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}
	if !strings.HasPrefix(abs, workspace+string(filepath.Separator)) && abs != workspace {
		return fmt.Errorf("path %q is outside workspace", target)
	}
	return nil
}

// copyToWorkspace copies a file into the workspace directory.
// If the file is already inside the workspace, it returns the existing path.
// Files are copied to the same directory as the current program, or the
// workspace root if no program is open.
func (s *Server) copyToWorkspace(srcPath string) (string, error) {
	if srcPath == "" {
		return "", fmt.Errorf("empty source path")
	}

	absSrc, err := filepath.Abs(srcPath)
	if err != nil {
		return "", err
	}
	absWorkspace, err := filepath.Abs(s.session.WorkspacePath)
	if err != nil {
		return "", err
	}

	// Already in workspace — no copy needed.
	if strings.HasPrefix(absSrc, absWorkspace+string(filepath.Separator)) {
		return absSrc, nil
	}

	// Determine destination directory: same as current program, or workspace root.
	destDir := absWorkspace
	if s.session.FilePath != "" {
		pgmDir, err := filepath.Abs(filepath.Dir(s.session.FilePath))
		if err == nil && strings.HasPrefix(pgmDir, absWorkspace) {
			destDir = pgmDir
		}
	}

	destPath := filepath.Join(destDir, filepath.Base(srcPath))

	// Don't overwrite if destination already exists.
	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil
	}

	src, err := os.Open(absSrc)
	if err != nil {
		return "", err
	}
	defer src.Close() //nolint:errcheck // best-effort close on read-only file

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close() //nolint:errcheck // best-effort close after copy

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return destPath, nil
}

// colocateSamples copies all referenced samples into targetDir (next to the .pgm)
// so the MPC 1000 can find them. Returns the number of files copied.
func (s *Server) colocateSamples(targetDir string) int {
	var copied int
	for i := 0; i < 64; i++ {
		for j := 0; j < 4; j++ {
			ref := s.session.Matrix.Get(i, j)
			if ref == nil || ref.FilePath == "" {
				continue
			}

			absRef, err := filepath.Abs(ref.FilePath)
			if err != nil {
				continue
			}
			absTarget, err := filepath.Abs(targetDir)
			if err != nil {
				continue
			}

			// Already in the target directory — nothing to do.
			if filepath.Dir(absRef) == absTarget {
				continue
			}

			destPath := filepath.Join(absTarget, filepath.Base(absRef))
			if _, err := os.Stat(destPath); err == nil {
				// Destination already exists — update reference but don't overwrite.
				ref.FilePath = destPath
				continue
			}

			src, err := os.Open(absRef)
			if err != nil {
				log.Printf("colocate open %s: %v", absRef, err)
				continue
			}
			dst, err := os.Create(destPath)
			if err != nil {
				_ = src.Close()
				log.Printf("colocate create %s: %v", destPath, err)
				continue
			}
			_, cpErr := io.Copy(dst, src)
			_ = src.Close()
			_ = dst.Close()
			if cpErr != nil {
				log.Printf("colocate copy %s: %v", destPath, cpErr)
				continue
			}

			ref.FilePath = destPath
			copied++
		}
	}
	return copied
}

// copySamplesToWorkspace copies each imported sample into the workspace
// and updates the SampleRef.FilePath to point to the workspace copy.
func (s *Server) copySamplesToWorkspace(samples []*pgm.SampleRef) {
	if s.session.WorkspacePath == "" {
		return
	}
	for _, ref := range samples {
		if ref == nil || ref.FilePath == "" || ref.Status == pgm.SampleRejected {
			continue
		}
		newPath, err := s.copyToWorkspace(ref.FilePath)
		if err != nil {
			log.Printf("copy sample to workspace: %v", err)
			continue
		}
		ref.FilePath = newPath
	}
}
