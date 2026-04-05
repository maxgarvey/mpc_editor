package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) handleWorkspaceImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
		return
	}

	dest := r.FormValue("dest")
	if dest == "" {
		dest = s.session.WorkspacePath
	}
	dest = filepath.Clean(dest)

	if err := s.validateWithinWorkspace(dest); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files uploaded", http.StatusBadRequest)
		return
	}

	var imported int
	for _, fh := range files {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if !isAllowedImportExt(ext) {
			continue
		}

		// Reject path separators in filename.
		base := filepath.Base(fh.Filename)
		if base == "." || base == ".." {
			continue
		}

		src, err := fh.Open()
		if err != nil {
			log.Printf("import open %s: %v", fh.Filename, err)
			continue
		}

		destPath := filepath.Join(dest, base)
		dst, err := os.Create(destPath)
		if err != nil {
			_ = src.Close()
			log.Printf("import create %s: %v", destPath, err)
			continue
		}

		_, cpErr := io.Copy(dst, src)
		_ = src.Close()
		_ = dst.Close()
		if cpErr != nil {
			log.Printf("import copy %s: %v", destPath, cpErr)
			continue
		}
		imported++
	}

	// Trigger background scan so new files appear in the catalog.
	go func() {
		if result, err := s.scanner.ScanWorkspace(s.session.WorkspacePath); err != nil {
			log.Printf("post-import scan: %v", err)
		} else {
			log.Printf("post-import scan: found=%d scanned=%d",
				result.FilesFound, result.FilesScanned)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"imported":%d}`, imported)
}

func isAllowedImportExt(ext string) bool {
	switch ext {
	case ".wav", ".pgm", ".seq", ".mid", ".sng", ".all":
		return true
	}
	return false
}
