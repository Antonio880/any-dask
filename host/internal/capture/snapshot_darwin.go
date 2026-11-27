//go:build darwin

package capture

import (
	"bytes"
	"image/jpeg"
	"os"
	"os/exec"
	"strconv"
)

// CaptureSnapshot captures the currently active macOS display (1-indexed for screencapture -D).
func CaptureSnapshot(maxWidth int) (SnapshotResult, error) {
	return CaptureSnapshotForDisplay(maxWidth, 0, 0, 0, 0, 0)
}

func CaptureSnapshotForDisplay(maxWidth, displayIndex, x, y, w, h int) (SnapshotResult, error) {
	tmpFile, err := os.CreateTemp("", "anydask-snap-*.jpg")
	if err != nil {
		return SnapshotResult{}, err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	dispArg := strconv.Itoa(displayIndex + 1)
	if err := exec.Command("screencapture", "-x", "-D", dispArg, "-t", "jpg", tmpPath).Run(); err != nil {
		if err2 := exec.Command("screencapture", "-x", "-t", "jpg", tmpPath).Run(); err2 != nil {
			return SnapshotResult{}, err2
		}
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
