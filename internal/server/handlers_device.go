package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s *Server) handleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	dev := s.detector.Current()
	s.renderTemplate(w, "device_status.html", map[string]any{
		"Device": dev,
	})
}

func (s *Server) handleDeviceDetect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dev := s.detector.Scan()
	s.renderTemplate(w, "device_status.html", map[string]any{
		"Device": dev,
	})
}

// handleDeviceLs returns a JSON directory listing for either the MPC or local workspace.
func (s *Server) handleDeviceLs(w http.ResponseWriter, r *http.Request) {
	root := r.FormValue("root") // "mpc" or "workspace"
	relDir := r.FormValue("dir")

	basePath, err := s.deviceRoot(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	absDir := filepath.Clean(filepath.Join(basePath, relDir))
	if !isWithin(absDir, basePath) {
		http.Error(w, "path outside root", http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type lsEntry struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
		Rel   string `json:"rel"`
	}

	var result []lsEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		rel := e.Name()
		if relDir != "" {
			rel = filepath.Join(relDir, e.Name())
		}
		var size int64
		if !e.IsDir() {
			if info, err := e.Info(); err == nil {
				size = info.Size()
			}
		}
		result = append(result, lsEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  size,
			Rel:   rel,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// handleDeviceMkdir creates a directory under the given root.
func (s *Server) handleDeviceMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root := r.FormValue("root")
	relDir := r.FormValue("dir")
	if relDir == "" {
		http.Error(w, "dir is required", http.StatusBadRequest)
		return
	}

	basePath, err := s.deviceRoot(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	absDir := filepath.Clean(filepath.Join(basePath, relDir))
	if !isWithin(absDir, basePath) {
		http.Error(w, "path outside root", http.StatusForbidden)
		return
	}

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleDeviceTransfer copies selected files/directories from one root to another.
func (s *Server) handleDeviceTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	direction := r.FormValue("direction") // "to_mpc" or "from_mpc"
	destRel := r.FormValue("dest_dir")
	createDest := r.FormValue("create_dest") == "true"
	srcPaths := r.Form["src"]

	if len(srcPaths) == 0 {
		http.Error(w, "no source paths provided", http.StatusBadRequest)
		return
	}

	dev := s.detector.Current()
	if dev == nil {
		http.Error(w, "no MPC device detected", http.StatusBadRequest)
		return
	}

	var srcRoot, destRoot string
	if direction == "to_mpc" {
		srcRoot = s.session.WorkspacePath
		destRoot = dev.MountPath
	} else {
		srcRoot = dev.MountPath
		destRoot = s.session.WorkspacePath
	}

	destAbs := filepath.Clean(filepath.Join(destRoot, destRel))
	if !isWithin(destAbs, destRoot) {
		http.Error(w, "destination outside root", http.StatusForbidden)
		return
	}

	if createDest {
		if err := os.MkdirAll(destAbs, 0o755); err != nil {
			http.Error(w, fmt.Sprintf("create destination: %v", err), http.StatusInternalServerError)
			return
		}
	} else if _, err := os.Stat(destAbs); err != nil {
		http.Error(w, fmt.Sprintf("destination does not exist: %s", destRel), http.StatusBadRequest)
		return
	}

	var transferred, failed int
	var errMsgs []string

	for _, relPath := range srcPaths {
		srcAbs := filepath.Clean(filepath.Join(srcRoot, relPath))
		if !isWithin(srcAbs, srcRoot) {
			failed++
			errMsgs = append(errMsgs, fmt.Sprintf("path outside root: %s", relPath))
			continue
		}
		dest := filepath.Join(destAbs, filepath.Base(srcAbs))
		if err := copyPath(srcAbs, dest); err != nil {
			failed++
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", filepath.Base(srcAbs), err))
			log.Printf("transfer %s -> %s: %v", srcAbs, dest, err)
		} else {
			transferred++
		}
	}

	// Rescan workspace if files were written to it.
	if direction == "from_mpc" {
		go func() {
			if _, err := s.scanner.ScanWorkspace(s.session.WorkspacePath); err != nil {
				log.Printf("post-transfer scan: %v", err)
			}
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"transferred": transferred,
		"failed":      failed,
		"messages":    errMsgs,
	})
}

// deviceRoot returns the base filesystem path for the given root identifier.
func (s *Server) deviceRoot(root string) (string, error) {
	switch root {
	case "mpc":
		dev := s.detector.Current()
		if dev == nil {
			return "", fmt.Errorf("no MPC device detected")
		}
		return dev.MountPath, nil
	case "workspace":
		if s.session.WorkspacePath == "" {
			return "", fmt.Errorf("no workspace configured")
		}
		return s.session.WorkspacePath, nil
	default:
		return "", fmt.Errorf("unknown root: %q", root)
	}
}

// isWithin reports whether target is the same as base or a child of it.
func isWithin(target, base string) bool {
	return target == base || strings.HasPrefix(target, base+string(filepath.Separator))
}

// copyPath copies a file or directory recursively from src to dst.
func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
