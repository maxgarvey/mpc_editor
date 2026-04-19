package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
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
	flatten := r.FormValue("flatten") == "1"

	if err := os.MkdirAll(dest, 0o755); err != nil {
		http.Error(w, fmt.Sprintf("mkdir dest: %v", err), http.StatusInternalServerError)
		return
	}

	var imported, transcoded int
	var importedWAVs []string // relative paths of imported WAV files for source attribution
	for _, fh := range files {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if !isAllowedImportExt(ext) {
			continue
		}

		// Reject path separators that could escape the destination.
		base := filepath.Base(fh.Filename)
		if base == "." || base == ".." {
			continue
		}

		// Determine the target directory, preserving relative subdirs unless flattening.
		fileDestDir := dest
		if !flatten && strings.ContainsAny(fh.Filename, "/\\") {
			relDir := filepath.Dir(filepath.ToSlash(fh.Filename))
			fileDestDir = filepath.Join(dest, filepath.FromSlash(relDir))
			// Security: ensure the resolved path stays within dest.
			cleanDest := filepath.Clean(dest)
			if cleanFileDestDir := filepath.Clean(fileDestDir); !strings.HasPrefix(cleanFileDestDir, cleanDest+string(filepath.Separator)) && cleanFileDestDir != cleanDest {
				log.Printf("import: relative path %q escapes dest, skipping", fh.Filename)
				continue
			}
			if err := os.MkdirAll(fileDestDir, 0o755); err != nil {
				log.Printf("import mkdir %s: %v", fileDestDir, err)
				continue
			}
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
			wavPath, err := audio.TranscodeToWAV(tmpPath, fileDestDir, origName)
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
			destPath := filepath.Join(fileDestDir, base)
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

// handleImportFormats returns the lists of supported import formats as JSON.
func (s *Server) handleImportFormats(w http.ResponseWriter, r *http.Request) {
	type formatList struct {
		Audio   []string `json:"audio"`
		Project []string `json:"project"`
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(formatList{
		Audio:   audio.SupportedAudioExts,
		Project: []string{".pgm", ".seq", ".mid", ".sng", ".all"},
	})
}

// dirScanResult is returned by handleImportDirScan.
type dirScanResult struct {
	Dir   string         `json:"dir"`
	Count int            `json:"count"`
	ByExt map[string]int `json:"by_ext"`
	Files []string       `json:"files"`
}

// handleImportDirScan walks a directory and reports all importable files.
// GET /workspace/import/scan?dir=...
func (s *Server) handleImportDirScan(w http.ResponseWriter, r *http.Request) {
	dir := filepath.Clean(r.URL.Query().Get("dir"))
	if dir == "" || dir == "." {
		http.Error(w, "dir required", http.StatusBadRequest)
		return
	}
	if err := s.validateWithinWorkspace(dir); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	result := dirScanResult{Dir: dir, ByExt: make(map[string]int)}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !isAllowedImportExt(ext) {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		result.Files = append(result.Files, rel)
		result.ByExt[ext]++
		result.Count++
		return nil
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("scan: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// handleImportDirExecute copies/transcodes all importable files from a source
// directory into the workspace destination.
// POST /workspace/import/dir  params: src_dir, dest, flatten (1/0), source
func (s *Server) handleImportDirExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("parse form: %v", err), http.StatusBadRequest)
		return
	}

	srcDir := filepath.Clean(r.FormValue("src_dir"))
	if srcDir == "" || srcDir == "." {
		http.Error(w, "src_dir required", http.StatusBadRequest)
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

	flatten := r.FormValue("flatten") == "1"
	source := r.FormValue("source")

	if err := os.MkdirAll(dest, 0o755); err != nil {
		http.Error(w, fmt.Sprintf("mkdir dest: %v", err), http.StatusInternalServerError)
		return
	}

	var imported, transcoded int
	var importedWAVs []string

	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !isAllowedImportExt(ext) {
			return nil
		}

		rel, _ := filepath.Rel(srcDir, path)

		var targetDir string
		if flatten {
			targetDir = dest
		} else {
			targetDir = filepath.Join(dest, filepath.Dir(rel))
			if err := os.MkdirAll(targetDir, 0o755); err != nil {
				log.Printf("import dir mkdir %s: %v", targetDir, err)
				return nil
			}
		}

		baseName := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))

		if audio.IsTranscodable(ext) {
			outName := uniqueBaseName(targetDir, baseName, ".wav")
			wavPath, err := audio.TranscodeToWAV(path, targetDir, outName)
			if err != nil {
				log.Printf("import dir transcode %s: %v", path, err)
				return nil
			}
			if source != "" {
				if relWav, err := filepath.Rel(s.session.WorkspacePath, wavPath); err == nil {
					importedWAVs = append(importedWAVs, relWav)
				}
			}
			imported++
			transcoded++
		} else {
			outName := uniqueBaseName(targetDir, baseName, ext)
			destPath := filepath.Join(targetDir, outName+ext)
			if err := copyFileImport(path, destPath); err != nil {
				log.Printf("import dir copy %s: %v", path, err)
				return nil
			}
			if source != "" && ext == ".wav" {
				if relWav, err := filepath.Rel(s.session.WorkspacePath, destPath); err == nil {
					importedWAVs = append(importedWAVs, relWav)
				}
			}
			imported++
		}
		return nil
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("walk: %v", err), http.StatusInternalServerError)
		return
	}

	go func() {
		if result, err := s.scanner.ScanWorkspace(s.session.WorkspacePath); err != nil {
			log.Printf("post-import-dir scan: %v", err)
		} else {
			log.Printf("post-import-dir scan: found=%d scanned=%d", result.FilesFound, result.FilesScanned)
		}

		if source != "" && len(importedWAVs) > 0 {
			ctx := context.Background()
			for _, relPath := range importedWAVs {
				f, err := s.queries.GetFileByPath(ctx, relPath)
				if err != nil {
					log.Printf("import dir attribution lookup %s: %v", relPath, err)
					continue
				}
				if err := s.queries.UpdateWavSource(ctx, db.UpdateWavSourceParams{
					Source: source,
					FileID: f.ID,
				}); err != nil {
					log.Printf("import dir attribution set %s: %v", relPath, err)
				}
			}
			log.Printf("applied source %q to %d imported WAVs", source, len(importedWAVs))
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"imported":%d,"transcoded":%d}`, imported, transcoded)
}

// uniqueBaseName returns a base name (without extension) that does not collide
// with existing files in dir. If dir/base+ext already exists, it appends _2,
// _3, etc. until a free slot is found.
func uniqueBaseName(dir, base, ext string) string {
	if _, err := os.Stat(filepath.Join(dir, base+ext)); os.IsNotExist(err) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", base, i)
		if _, err := os.Stat(filepath.Join(dir, candidate+ext)); os.IsNotExist(err) {
			return candidate
		}
	}
}

func copyFileImport(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		return cpErr
	}
	return closeErr
}
