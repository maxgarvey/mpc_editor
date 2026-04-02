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
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return destPath, nil
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
