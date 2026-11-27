//go:build windows

package capture

import (
	"errors"
	"image"
	"syscall"
	"unsafe"
)

var (
	user32Cap            = syscall.NewLazyDLL("user32.dll")
	gdi32Cap             = syscall.NewLazyDLL("gdi32.dll")
	procGetDC            = user32Cap.NewProc("GetDC")
	procReleaseDC        = user32Cap.NewProc("ReleaseDC")
	procGetSysMetricsCap = user32Cap.NewProc("GetSystemMetrics")
	procCreateCompatDC   = gdi32Cap.NewProc("CreateCompatibleDC")
	procCreateCompatBmp  = gdi32Cap.NewProc("CreateCompatibleBitmap")
	procSelectObject     = gdi32Cap.NewProc("SelectObject")
	procBitBlt           = gdi32Cap.NewProc("BitBlt")
	procDeleteObject     = gdi32Cap.NewProc("DeleteObject")
	procDeleteDC         = gdi32Cap.NewProc("DeleteDC")
	procGetDIBits        = gdi32Cap.NewProc("GetDIBits")
)

const (
	srcCopy      = 0x00CC0020
	captureBlt   = 0x40000000
	biRGB        = 0
	dibRGBColors = 0
)

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// CaptureSnapshot captures the primary screen directly in memory via GDI BitBlt (~15ms, zero external tools)
// and returns a resized JPEG suitable for Computer Use vision models.
func CaptureSnapshot(maxWidth int) (SnapshotResult, error) {
	wPtr, _, _ := procGetSysMetricsCap.Call(0)
	hPtr, _, _ := procGetSysMetricsCap.Call(1)
	w, h := int(wPtr), int(hPtr)
	if w <= 0 || h <= 0 {
		return SnapshotResult{}, errors.New("invalid screen dimensions")
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return SnapshotResult{}, errors.New("GetDC failed")
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatDC.Call(hdcScreen)
	if hdcMem == 0 {
		return SnapshotResult{}, errors.New("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdcMem)

	hBitmap, _, _ := procCreateCompatBmp.Call(hdcScreen, uintptr(w), uintptr(h))
	if hBitmap == 0 {
		return SnapshotResult{}, errors.New("CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(hBitmap)

	oldObj, _, _ := procSelectObject.Call(hdcMem, hBitmap)
	defer procSelectObject.Call(hdcMem, oldObj)

	ok, _, _ := procBitBlt.Call(
		hdcMem, 0, 0, uintptr(w), uintptr(h),
		hdcScreen, 0, 0, srcCopy|captureBlt,
	)
	if ok == 0 {
		return SnapshotResult{}, errors.New("BitBlt failed")
	}

	var bih bitmapInfoHeader
	bih.BiSize = uint32(unsafe.Sizeof(bih))
	bih.BiWidth = int32(w)
	bih.BiHeight = -int32(h) // top-down DIB
	bih.BiPlanes = 1
	bih.BiBitCount = 32
	bih.BiCompression = biRGB

	raw := make([]byte, w*h*4)
	lines, _, _ := procGetDIBits.Call(
		hdcMem, hBitmap, 0, uintptr(h),
		uintptr(unsafe.Pointer(&raw[0])),
		uintptr(unsafe.Pointer(&bih)),
		dibRGBColors,
	)
	if lines == 0 {
		return SnapshotResult{}, errors.New("GetDIBits failed")
	}

	// Convert BGRA -> RGBA in place for Go's image.RGBA
	for i := 0; i < len(raw); i += 4 {
		raw[i], raw[i+2] = raw[i+2], raw[i]
		raw[i+3] = 255
	}
	img := &image.RGBA{
		Pix:    raw,
		Stride: w * 4,
		Rect:   image.Rect(0, 0, w, h),
	}

	jpegBytes, tw, th, err := ResizeAndEncodeJPEG(img, maxWidth, 82)
	if err != nil {
		return SnapshotResult{}, err
	}
	return SnapshotResult{
		JPEGBytes: jpegBytes,
		Width:     tw,
		Height:    th,
		ScreenW:   w,
		ScreenH:   h,
	}, nil
}
