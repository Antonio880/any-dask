package capture

import (
	"image"
	"image/color"
	"testing"
)

func TestAnnexBStartCodeAndJPEGResize(t *testing.T) {
	stream := []byte{0, 0, 0, 1, 0x67, 0x42, 0, 0, 1, 0x68, 0xCE}
	idx, ln := findStartCode(stream, 0)
	if idx != 0 || ln != 4 {
		t.Fatalf("expected start code at 0 len 4, got %d len %d", idx, ln)
	}
	idx2, ln2 := findStartCode(stream, 4)
	if idx2 != 6 || ln2 != 3 {
		t.Fatalf("expected second start code at 6 len 3, got %d len %d", idx2, ln2)
	}

	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	img.Set(10, 10, color.RGBA{R: 100, G: 180, B: 255, A: 255})
	jpegBytes, w, h, err := ResizeAndEncodeJPEG(img, 1280, 80)
	if err != nil {
		t.Fatalf("ResizeAndEncodeJPEG failed: %v", err)
	}
	if w != 1280 || h != 720 {
		t.Fatalf("expected 1280x720, got %dx%d", w, h)
	}
	if len(jpegBytes) < 100 {
		t.Fatalf("unexpectedly small JPEG output: %d bytes", len(jpegBytes))
	}
}
