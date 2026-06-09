package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/db"
)

// handleLibraryCheck checks the sync status of a library copy and re-renders the
// WAV detail panel with updated status. POST /library/check with form field "path"
// (absolute path of the copy file).
func (s *Server) handleLibraryCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	absPath := r.FormValue("path")
	if absPath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	ws := s.session.WorkspacePath
	relPath, err := filepath.Rel(ws, absPath)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	s.checkSyncStatus(r.Context(), relPath)

	// Re-render the full WAV detail so the UI shows the new status.
	s.renderDetailWAV(w, absPath)
}

// handleLibraryUpdate re-copies the library source to the workspace copy location,
// updates the link checksum, and re-renders the WAV detail panel.
// POST /library/update with form field "path" (absolute path of the copy file).
func (s *Server) handleLibraryUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	absPath := r.FormValue("path")
	if absPath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	ws := s.session.WorkspacePath
	relPath, err := filepath.Rel(ws, absPath)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	link, err := s.queries.GetSampleLinkByCopyPath(ctx, relPath)
	if err != nil {
		http.Error(w, "no library link found for this file", http.StatusBadRequest)
		return
	}

	srcAbs := filepath.Join(ws, link.LibraryPath)
	if _, err := os.Stat(srcAbs); err != nil {
		http.Error(w, fmt.Sprintf("library source not found: %s", link.LibraryPath), http.StatusBadRequest)
		return
	}

	// Re-copy: overwrite the existing file with a freshly normalized copy.
	if err := audio.NormalizeWAVForMPC(srcAbs, absPath); err != nil {
		log.Printf("library update: normalize %s: %v", relPath, err)
		http.Error(w, "update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record the new checksum and mark as in sync.
	newChecksum, err := checksumFile(srcAbs)
	if err != nil {
		newChecksum = ""
	}
	var srcSize, srcModTime int64
	if info, err := os.Stat(srcAbs); err == nil {
		srcSize = info.Size()
		srcModTime = info.ModTime().Unix()
	}
	if err := s.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    relPath,
		LibraryPath: link.LibraryPath,
		Checksum:    newChecksum,
		CopiedAt:    time.Now().Unix(),
		SrcSize:     srcSize,
		SrcModTime:  srcModTime,
	}); err != nil {
		log.Printf("library update: upsert link: %v", err)
	}

	// Trigger a rescan so wav_meta stays current.
	if _, err := s.scanner.ScanWorkspace(ws); err != nil {
		log.Printf("library update: rescan: %v", err)
	}
	s.refreshAllLibraryLinks(context.Background())

	// Re-render WAV detail with updated state.
	s.renderDetailWAV(w, absPath)
}
