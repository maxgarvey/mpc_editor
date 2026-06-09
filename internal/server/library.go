package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maxgarvey/mpc_editor/internal/db"
)

// libDir returns the absolute path of the sample_library directory for the workspace.
func (s *Server) libDir() string {
	return filepath.Join(s.session.WorkspacePath, "sample_library")
}

// isUnderLibrary reports whether the given absolute path is inside sample_library/.
func (s *Server) isUnderLibrary(absPath string) bool {
	lib := s.libDir()
	rel, err := filepath.Rel(lib, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// recordLibraryLinkIfApplicable records a sample_links row when srcAbs is under
// sample_library/ and dstAbs is the workspace copy that was just written.
// Both paths are stored relative to the workspace root so the record survives
// moving the workspace to another machine.
// This is best-effort — errors are logged but not returned.
func (s *Server) recordLibraryLinkIfApplicable(ctx context.Context, srcAbs, dstAbs string) {
	if !s.isUnderLibrary(srcAbs) {
		return
	}
	ws := s.session.WorkspacePath

	srcRel, err := filepath.Rel(ws, srcAbs)
	if err != nil {
		log.Printf("library link: rel src %s: %v", srcAbs, err)
		return
	}
	dstRel, err := filepath.Rel(ws, dstAbs)
	if err != nil {
		log.Printf("library link: rel dst %s: %v", dstAbs, err)
		return
	}

	checksum, err := checksumFile(srcAbs)
	if err != nil {
		log.Printf("library link: checksum %s: %v", srcAbs, err)
		checksum = ""
	}

	var srcSize, srcModTime int64
	if info, err := os.Stat(srcAbs); err == nil {
		srcSize = info.Size()
		srcModTime = info.ModTime().Unix()
	}

	if err := s.queries.UpsertSampleLink(ctx, db.UpsertSampleLinkParams{
		CopyPath:    dstRel,
		LibraryPath: srcRel,
		Checksum:    checksum,
		CopiedAt:    time.Now().Unix(),
		SrcSize:     srcSize,
		SrcModTime:  srcModTime,
	}); err != nil {
		log.Printf("library link: upsert %s: %v", dstRel, err)
	}
}

// checksumFile returns the hex SHA-256 digest of the file at path.
func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only, close errors are not actionable
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// sampleLinkedMap returns a set of relative copy_paths that are library links,
// restricted to those under the given relative directory prefix. The prefix
// must end in "/" or be "" for the workspace root.
// The map value is the relative library_path.
func (s *Server) sampleLinkedMap(ctx context.Context, relDirPrefix string) map[string]string {
	links, err := s.queries.ListSampleLinksForDir(ctx, relDirPrefix+"%")
	if err != nil {
		return nil
	}
	m := make(map[string]string, len(links))
	for _, l := range links {
		m[l.CopyPath] = l.LibraryPath
	}
	return m
}

// checkSyncStatus re-checksums the library source for the given relative copy path,
// updates sample_links.sync_status, and returns the new status string.
// Possible values: "ok", "outdated", "source_missing".
func (s *Server) checkSyncStatus(ctx context.Context, relCopyPath string) string {
	link, err := s.queries.GetSampleLinkByCopyPath(ctx, relCopyPath)
	if err != nil {
		return ""
	}

	srcAbs := filepath.Join(s.session.WorkspacePath, link.LibraryPath)
	info, err := os.Stat(srcAbs)
	if os.IsNotExist(err) {
		_ = s.queries.UpdateSampleLinkSync(ctx, db.UpdateSampleLinkSyncParams{
			SyncStatus: "source_missing",
			Checksum:   link.Checksum,
			SrcSize:    link.SrcSize,
			SrcModTime: link.SrcModTime,
			CopyPath:   relCopyPath,
		})
		return "source_missing"
	}
	if err != nil {
		return ""
	}

	// Fast path: source unchanged (same size and mod time) since the last
	// check — keep the recorded status without re-reading the file.
	if link.SyncStatus != "" && link.SyncStatus != "source_missing" &&
		info.Size() == link.SrcSize && info.ModTime().Unix() == link.SrcModTime {
		return link.SyncStatus
	}

	newChecksum, err := checksumFile(srcAbs)
	if err != nil {
		return ""
	}

	status := "ok"
	if newChecksum != link.Checksum {
		status = "outdated"
	}
	_ = s.queries.UpdateSampleLinkSync(ctx, db.UpdateSampleLinkSyncParams{
		SyncStatus: status,
		Checksum:   link.Checksum, // keep stored checksum unchanged; update on actual copy
		SrcSize:    info.Size(),
		SrcModTime: info.ModTime().Unix(),
		CopyPath:   relCopyPath,
	})
	return status
}

// refreshAllLibraryLinks checks sync status for every known sample link.
// Called after workspace scans to keep status current.
func (s *Server) refreshAllLibraryLinks(ctx context.Context) {
	links, err := s.queries.ListAllSampleLinks(ctx)
	if err != nil {
		log.Printf("library sync: list links: %v", err)
		return
	}
	for _, l := range links {
		s.checkSyncStatus(ctx, l.CopyPath) //nolint:errcheck // best-effort background refresh
	}
}
