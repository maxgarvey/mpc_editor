package audio

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// transcodableExtensions lists audio formats that can be converted to WAV.
var transcodableExtensions = map[string]bool{
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".aif":  true,
	".aiff": true,
	".m4a":  true,
	".wma":  true,
	".opus": true,
}

// IsTranscodable returns true if the file extension is a known audio format
// that can be transcoded to WAV.
func IsTranscodable(ext string) bool {
	return transcodableExtensions[strings.ToLower(ext)]
}

// TranscodeToWAV converts an audio file to 16-bit PCM WAV using ffmpeg.
// The output file is placed in destDir with a .wav extension.
// If outputName is non-empty, it is used as the base name (without extension)
// for the output file; otherwise the source file's base name is used.
// Returns the path to the new WAV file.
func TranscodeToWAV(srcPath, destDir string, outputName ...string) (string, error) {
	if err := checkFFmpeg(); err != nil {
		return "", err
	}

	var name string
	if len(outputName) > 0 && outputName[0] != "" {
		name = outputName[0]
	} else {
		base := filepath.Base(srcPath)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	wavPath := filepath.Join(destDir, name+".wav")

	cmd := exec.Command("ffmpeg",
		"-i", srcPath,
		"-acodec", "pcm_s16le",
		"-ar", "44100",
		"-y",
		wavPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg: %w: %s", err, string(output))
	}

	return wavPath, nil
}

// CheckFFmpegAvailable returns nil if ffmpeg is installed, or an error.
func CheckFFmpegAvailable() error {
	return checkFFmpeg()
}

// NormalizeWAVForMPC copies srcPath to destPath, converting to 16-bit PCM
// 44100 Hz via ffmpeg if the source is not already in that format.
// Returns an error if conversion is needed but ffmpeg is unavailable.
func NormalizeWAVForMPC(srcPath, destPath string) error {
	format, _, err := ReadWAVHeader(srcPath)
	if err == nil && format.BitsPerSample == 16 && format.SampleRate == 44100 {
		return copyFileRaw(srcPath, destPath)
	}
	// Header unreadable or wrong format — convert via ffmpeg.
	destDir := filepath.Dir(destPath)
	destBase := strings.TrimSuffix(filepath.Base(destPath), filepath.Ext(destPath))
	_, transcodeErr := TranscodeToWAV(srcPath, destDir, destBase)
	return transcodeErr
}

func copyFileRaw(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only file
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

func checkFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH; install ffmpeg to import non-WAV audio files")
	}
	return nil
}
