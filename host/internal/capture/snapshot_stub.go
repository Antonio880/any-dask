//go:build !windows && !darwin

package capture

import "errors"

func CaptureSnapshot(maxWidth int) (SnapshotResult, error) {
	return SnapshotResult{}, errors.New("snapshot capture not supported on this OS")
}

func CaptureSnapshotForDisplay(maxWidth, displayIndex, x, y, w, h int) (SnapshotResult, error) {
	return SnapshotResult{}, errors.New("snapshot capture not supported on this OS")
}
