package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/db"
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
		dest = filepath.Join(s.session.WorkspacePath, "sample_library")
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

	source := r.FormValue("source")

	var imported, transcoded int
	var importedWAVs []string // relative paths of imported WAV files for source attribution
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

		if audio.IsTranscodable(ext) {
			// Save to temp file, then transcode to WAV in destination.
			tmpFile, err := os.CreateTemp("", "mpc-transcode-*"+ext)
			if err != nil {
				_ = src.Close()
				log.Printf("import temp %s: %v", fh.Filename, err)
				continue
			}
			_, cpErr := io.Copy(tmpFile, src)
			_ = src.Close()
			tmpPath := tmpFile.Name()
			_ = tmpFile.Close()
			if cpErr != nil {
				_ = os.Remove(tmpPath)
				log.Printf("import copy temp %s: %v", fh.Filename, cpErr)
				continue
			}

			// Use the original filename (without extension) for the output WAV.
			origName := strings.TrimSuffix(base, filepath.Ext(base))
			wavPath, err := audio.TranscodeToWAV(tmpPath, dest, origName)
			_ = os.Remove(tmpPath)
			if err != nil {
				log.Printf("import transcode %s: %v", fh.Filename, err)
				continue
			}
			log.Printf("transcoded %s -> %s", fh.Filename, filepath.Base(wavPath))
			if source != "" {
				if rel, err := filepath.Rel(s.session.WorkspacePath, wavPath); err == nil {
					importedWAVs = append(importedWAVs, rel)
				}
			}
			imported++
			transcoded++
		} else {
			// Direct copy for native formats.
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
			if source != "" && strings.ToLower(filepath.Ext(base)) == ".wav" {
				if rel, err := filepath.Rel(s.session.WorkspacePath, destPath); err == nil {
					importedWAVs = append(importedWAVs, rel)
				}
			}
			imported++
		}
	}

	// Trigger background scan so new files appear in the catalog,
	// then apply source attribution to imported WAVs.
	go func() {
		if result, err := s.scanner.ScanWorkspace(s.session.WorkspacePath); err != nil {
			log.Printf("post-import scan: %v", err)
		} else {
			log.Printf("post-import scan: found=%d scanned=%d",
				result.FilesFound, result.FilesScanned)
		}

		if source != "" && len(importedWAVs) > 0 {
			ctx := context.Background()
			for _, relPath := range importedWAVs {
				f, err := s.queries.GetFileByPath(ctx, relPath)
				if err != nil {
					log.Printf("import attribution lookup %s: %v", relPath, err)
					continue
				}
				if err := s.queries.UpdateWavSource(ctx, db.UpdateWavSourceParams{
					Source: source,
					FileID: f.ID,
				}); err != nil {
					log.Printf("import attribution set %s: %v", relPath, err)
				}
			}
			log.Printf("applied source %q to %d imported WAVs", source, len(importedWAVs))
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"imported":%d,"transcoded":%d}`, imported, transcoded)
}

func isAllowedImportExt(ext string) bool {
	switch ext {
	case ".wav", ".pgm", ".seq", ".mid", ".sng", ".all":
		return true
	}
	return audio.IsTranscodable(ext)
}
