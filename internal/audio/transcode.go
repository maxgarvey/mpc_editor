package audio

import (
	"fmt"
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
		"-acodec", "pcm_s16le", // 16-bit signed little-endian PCM
		"-ar", "44100", // 44.1kHz sample rate (MPC standard)
		"-y",     // overwrite output
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

func checkFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH; install ffmpeg to import non-WAV audio files")
	}
	return nil
}
