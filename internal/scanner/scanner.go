package scanner

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maxgarvey/mpc_editor/internal/audio"
	"github.com/maxgarvey/mpc_editor/internal/db"
	"github.com/maxgarvey/mpc_editor/internal/pgm"
)

// recognizedExtensions maps lowercase file extensions to their catalog type.
var recognizedExtensions = map[string]string{
	".pgm": "pgm",
	".wav": "wav",
	".mid": "mid",
	".seq": "seq",
	".sng": "sng",
	".all": "all",
}

// ScanResult holds statistics from a workspace scan.
type ScanResult struct {
	FilesFound   int
	FilesScanned int // parsed (new or changed)
	FilesRemoved int // stale entries pruned
	Errors       []string
}

// Scanner catalogs MPC files in the workspace and extracts metadata.
type Scanner struct {
	queries *db.Queries
	sqlDB   *sql.DB
}

// New creates a Scanner.
func New(sqlDB *sql.DB, queries *db.Queries) *Scanner {
	return &Scanner{queries: queries, sqlDB: sqlDB}
}

// ScanWorkspace walks the workspace directory, catalogs files, extracts
// metadata from parseable types, and prunes stale entries.
func (s *Scanner) ScanWorkspace(workspace string) (*ScanResult, error) {
	ctx := context.Background()
	result := &ScanResult{}

	// Collect all paths found on disk.
	found := make(map[string]bool)

	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		fileType, ok := recognizedExtensions[ext]
		if !ok {
			return nil
		}

		relPath, err := filepath.Rel(workspace, path)
		if err != nil {
			return nil
		}
		found[relPath] = true
		result.FilesFound++

		modTime := info.ModTime().Unix()

		// Check if file has changed since last scan.
		existing, dbErr := s.queries.GetFileByPath(ctx, relPath)
		if dbErr == nil && existing.ModTime == modTime && existing.Scanned > 0 {
			return nil // unchanged, skip
		}

		// Upsert the file entry (scanned=0 initially for unparseable types).
		fileID, uErr := s.queries.UpsertFile(ctx, db.UpsertFileParams{
			Path:     relPath,
			FileType: fileType,
			Size:     info.Size(),
			ModTime:  modTime,
			Scanned:  0,
		})
		if uErr != nil {
			result.Errors = append(result.Errors, relPath+": "+uErr.Error())
			return nil
		}

		// Parse metadata for supported types.
		var parseErr error
		switch fileType {
		case "pgm":
			parseErr = s.scanPGM(ctx, fileID, path)
		case "wav":
			parseErr = s.scanWAV(ctx, fileID, path)
		}

		if parseErr != nil {
			result.Errors = append(result.Errors, relPath+": "+parseErr.Error())
		} else {
			// Mark as scanned.
			_, _ = s.queries.UpsertFile(ctx, db.UpsertFileParams{
				Path:     relPath,
				FileType: fileType,
				Size:     info.Size(),
				ModTime:  modTime,
				Scanned:  time.Now().Unix(),
			})
			result.FilesScanned++
		}

		return nil
	})
	if err != nil {
		return result, err
	}

	// Prune stale entries.
	allFiles, _ := s.queries.ListAllFiles(ctx)
	for _, f := range allFiles {
		if !found[f.Path] {
			_ = s.queries.DeleteFile(ctx, f.ID)
			result.FilesRemoved++
		}
	}

	// Resolution pass: link unresolved sample references to wav files.
	if err := s.queries.ResolveUnlinkedSamples(ctx); err != nil {
		log.Printf("resolve unlinked samples: %v", err)
	}

	return result, nil
}

// scanPGM extracts metadata and sample references from a .pgm file.
func (s *Scanner) scanPGM(ctx context.Context, fileID int64, path string) error {
	prog, err := pgm.OpenProgram(path)
	if err != nil {
		return err
	}

	// Store PGM metadata.
	if err := s.queries.UpsertPgmMeta(ctx, db.UpsertPgmMetaParams{
		FileID:        fileID,
		MidiPgmChange: int64(prog.GetMIDIProgramChange()),
	}); err != nil {
		return err
	}

	// Clear old sample references and re-insert.
	if err := s.queries.DeletePgmSamples(ctx, fileID); err != nil {
		return err
	}

	for padIdx := 0; padIdx < prog.PadCount(); padIdx++ {
		pad := prog.Pad(padIdx)
		for layerIdx := 0; layerIdx < pad.LayerCount(); layerIdx++ {
			layer := pad.Layer(layerIdx)
			name := layer.GetSampleName()
			if name == "" {
				continue
			}

			// Try to resolve the sample to a cataloged wav file.
			var sampleFileID sql.NullInt64
			wavFile, err := s.findWavByName(ctx, name)
			if err == nil {
				sampleFileID = sql.NullInt64{Int64: wavFile.ID, Valid: true}
			}

			if err := s.queries.InsertPgmSample(ctx, db.InsertPgmSampleParams{
				PgmFileID:    fileID,
				Pad:          int64(padIdx),
				Layer:        int64(layerIdx),
				SampleName:   name,
				SampleFileID: sampleFileID,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// scanWAV extracts format metadata from a .wav file header.
func (s *Scanner) scanWAV(ctx context.Context, fileID int64, path string) error {
	format, frameCount, err := audio.ReadWAVHeader(path)
	if err != nil {
		return err
	}

	return s.queries.UpsertWavMeta(ctx, db.UpsertWavMetaParams{
		FileID:        fileID,
		SampleRate:    int64(format.SampleRate),
		Channels:      int64(format.Channels),
		BitsPerSample: int64(format.BitsPerSample),
		FrameCount:    int64(frameCount),
		Source:        "",
	})
}

// findWavByName searches for a .wav file in the catalog matching the sample name.
func (s *Scanner) findWavByName(ctx context.Context, name string) (db.File, error) {
	// Try common path patterns: name.wav, name.WAV
	wavFiles, err := s.queries.ListFilesByType(ctx, "wav")
	if err != nil {
		return db.File{}, err
	}

	lowerName := strings.ToLower(name)
	for _, f := range wavFiles {
		base := filepath.Base(f.Path)
		baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))
		if strings.ToLower(baseNoExt) == lowerName {
			return f, nil
		}
	}

	return db.File{}, sql.ErrNoRows
}
