//go:build darwin

package capture

import (
	"bytes"
	"image/jpeg"
	"os"
	"os/exec"
)

// CaptureSnapshot uses native macOS screencapture and resizes to maxWidth for Vision LLMs.
func CaptureSnapshot(maxWidth int) (SnapshotResult, error) {
	tmpFile, err := os.CreateTemp("", "anydask-snap-*.jpg")
	if err != nil {
		return SnapshotResult{}, err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := exec.Command("screencapture", "-x", "-t", "jpg", tmpPath).Run(); err != nil {
		return SnapshotResult{}, err
	}
	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return SnapshotResult{}, err
	}
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return SnapshotResult{}, err
	}
	sb := img.Bounds()
	jpegBytes, tw, th, err := ResizeAndEncodeJPEG(img, maxWidth, 82)
	if err != nil {
		return SnapshotResult{}, err
	}
	return SnapshotResult{
		JPEGBytes: jpegBytes,
		Width:     tw,
		Height:    th,
		ScreenW:   sb.Dx(),
		ScreenH:   sb.Dy(),
	}, nil
}
