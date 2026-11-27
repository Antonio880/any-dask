//go:build windows

package overlay

import (
	"math"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	u32                  = syscall.NewLazyDLL("user32.dll")
	g32                  = syscall.NewLazyDLL("gdi32.dll")
	k32                  = syscall.NewLazyDLL("kernel32.dll")
	pRegisterClassExW    = u32.NewProc("RegisterClassExW")
	pCreateWindowExW     = u32.NewProc("CreateWindowExW")
	pDefWindowProcW      = u32.NewProc("DefWindowProcW")
	pShowWindow          = u32.NewProc("ShowWindow")
	pUpdateLayeredWindow = u32.NewProc("UpdateLayeredWindow")
	pPeekMessageW        = u32.NewProc("PeekMessageW")
	pTranslateMessage    = u32.NewProc("TranslateMessage")
	pDispatchMessageW    = u32.NewProc("DispatchMessageW")
	pDestroyWindow       = u32.NewProc("DestroyWindow")
	pGetSystemMetricsOv  = u32.NewProc("GetSystemMetrics")
	pGetDC               = u32.NewProc("GetDC")
	pReleaseDC           = u32.NewProc("ReleaseDC")
	pCreateCompatDC      = g32.NewProc("CreateCompatibleDC")
	pCreateDIBSection    = g32.NewProc("CreateDIBSection")
	pSelectObject        = g32.NewProc("SelectObject")
	pDeleteObject        = g32.NewProc("DeleteObject")
	pDeleteDC            = g32.NewProc("DeleteDC")
	pGetModuleHandleW    = k32.NewProc("GetModuleHandleW")
)

const (
	wsPopup         = 0x80000000
	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	wsExNoActivate  = 0x08000000

	swHide       = 0
	swShowNA     = 8
	pmRemove     = 1
	ulwAlpha     = 2
	acSrcOver    = 0
	acSrcAlpha   = 1
	biRGB        = 0
	dibRGBColors = 0
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ x, y int32 }
type size struct{ cx, cy int32 }

type msgStruct struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type ripple struct {
	x, y  int
	start time.Time
	isAI  bool
}

type winOverlay struct {
	mu      sync.Mutex
	enabled bool
	state   State
	label   string
	ripples []ripple
	stopCh  chan struct{}
	closed  bool
}

// RunMainLoop simply invokes serverFn on Windows (the Win32 overlay runs on its own locked UI goroutine).
func RunMainLoop(serverFn func()) {
	serverFn()
}

func New(enabled bool) Overlay {
	ov := &winOverlay{
		enabled: enabled,
		state:   StateHidden,
		stopCh:  make(chan struct{}),
	}
	go ov.uiThread()
	return ov
}

func (o *winOverlay) SetEnabled(enabled bool) {
	o.mu.Lock()
	o.enabled = enabled
	o.mu.Unlock()
}

func (o *winOverlay) IsEnabled() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.enabled
}

func (o *winOverlay) SetState(state State, label string) {
	o.mu.Lock()
	o.state = state
	o.label = label
	o.mu.Unlock()
}

func (o *winOverlay) TriggerRipple(x, y int, isAI bool) {
	o.mu.Lock()
	o.ripples = append(o.ripples, ripple{x: x, y: y, start: time.Now(), isAI: isAI})
	if len(o.ripples) > 10 {
		o.ripples = o.ripples[len(o.ripples)-10:]
	}
	o.mu.Unlock()
}

func (o *winOverlay) Close() {
	o.mu.Lock()
	if !o.closed {
		o.closed = true
		close(o.stopCh)
	}
	o.mu.Unlock()
}

func (o *winOverlay) uiThread() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	wPtr, _, _ := pGetSystemMetricsOv.Call(0)
	hPtr, _, _ := pGetSystemMetricsOv.Call(1)
	w, h := int(wPtr), int(hPtr)
	if w <= 0 || h <= 0 {
		return
	}

	hInst, _, _ := pGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("AnyDaskLiquidGlassOverlay")
	winTitle, _ := syscall.UTF16PtrFromString("AnyDask Liquid Glass")

	wndProc := syscall.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
		ret, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	})

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   wndProc,
		hInstance:     hInst,
		lpszClassName: className,
	}
	_, _, _ = pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	exStyle := uintptr(wsExLayered | wsExTransparent | wsExTopmost | wsExToolWindow | wsExNoActivate)
	hwnd, _, _ := pCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(winTitle)),
		wsPopup,
		0, 0, uintptr(w), uintptr(h),
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return
	}
	defer pDestroyWindow.Call(hwnd)

	hdcScreen, _, _ := pGetDC.Call(0)
	defer pReleaseDC.Call(0, hdcScreen)
	hdcMem, _, _ := pCreateCompatDC.Call(hdcScreen)
	defer pDeleteDC.Call(hdcMem)

	var bmi bitmapInfoHeader
	bmi.biSize = uint32(unsafe.Sizeof(bmi))
	bmi.biWidth = int32(w)
	bmi.biHeight = -int32(h) // top-down
	bmi.biPlanes = 1
	bmi.biBitCount = 32
	bmi.biCompression = biRGB

	var bits unsafe.Pointer
	hBmp, _, _ := pCreateDIBSection.Call(
		hdcScreen,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if hBmp == 0 || bits == nil {
		return
	}
	defer pDeleteObject.Call(hBmp)
	oldBmp, _, _ := pSelectObject.Call(hdcMem, hBmp)
	defer pSelectObject.Call(hdcMem, oldBmp)

	buf := unsafe.Slice((*byte)(bits), w*h*4)

	visible := false
	phase := 0.0
	ticker := time.NewTicker(33 * time.Millisecond) // smooth ~30fps liquid motion
	defer ticker.Stop()

	var m msgStruct
	for {
		for {
			r, _, _ := pPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), hwnd, 0, 0, pmRemove)
			if r == 0 {
				break
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}

		select {
		case <-o.stopCh:
			return
		case <-ticker.C:
		}

		o.mu.Lock()
		active := o.enabled && o.state != StateHidden
		st := o.state
		now := time.Now()
		liveRipples := o.ripples[:0]
		for _, rp := range o.ripples {
			if now.Sub(rp.start) < 680*time.Millisecond {
				liveRipples = append(liveRipples, rp)
			}
		}
		o.ripples = liveRipples
		ripplesCopy := append([]ripple(nil), liveRipples...)
		o.mu.Unlock()

		if !active {
			if visible {
				pShowWindow.Call(hwnd, swHide)
				visible = false
			}
			continue
		}

		if !visible {
			pShowWindow.Call(hwnd, swShowNA)
			visible = true
		}

		speed := 0.055
		intensity := 0.58
		borderDepth := 32
		if st == StateAIThinking || st == StateAIActing {
			speed = 0.12
			intensity = 0.96
			borderDepth = 46
		}
		phase += speed

		renderLiquidGlass(buf, w, h, phase, intensity, borderDepth, st, ripplesCopy, now)

		ptSrc := point{0, 0}
		ptDst := point{0, 0}
		sz := size{int32(w), int32(h)}
		bf := blendFunction{
			BlendOp:             acSrcOver,
			BlendFlags:          0,
			SourceConstantAlpha: 255,
			AlphaFormat:         acSrcAlpha,
		}
		pUpdateLayeredWindow.Call(
			hwnd, hdcScreen,
			uintptr(unsafe.Pointer(&ptDst)),
			uintptr(unsafe.Pointer(&sz)),
			hdcMem,
			uintptr(unsafe.Pointer(&ptSrc)),
			0,
			uintptr(unsafe.Pointer(&bf)),
			ulwAlpha,
		)
	}
}

// renderLiquidGlass paints Apple Intelligence liquid perimeter glow, top glass Dynamic Island pill,
// and refractive touch/AI click ripples using premultiplied BGRA pixels.
func renderLiquidGlass(buf []byte, w, h int, phase, intensity float64, borderDepth int, st State, ripples []ripple, now time.Time) {
	// Clear buffer quickly
	clear(buf)

	// 1. Liquid Glass / Apple Intelligence Perimeter Glow (only touches perimeter bands -> ultra fast!)
	drawBand := func(x0, y0, x1, y1 int) {
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		if x1 > w {
			x1 = w
		}
		if y1 > h {
			y1 = h
		}
		cx := float64(w) * 0.5
		cy := float64(h) * 0.5
		bd := float64(borderDepth)

		for y := y0; y < y1; y++ {
			dyEdge := y
			if h-1-y < dyEdge {
				dyEdge = h - 1 - y
			}
			rowOff := y * w * 4
			for x := x0; x < x1; x++ {
				dxEdge := x
				if w-1-x < dxEdge {
					dxEdge = w - 1 - x
				}
				d := dxEdge
				if dyEdge < d {
					d = dyEdge
				}
				if d >= borderDepth {
					continue
				}

				// Smooth liquid wave modulating border thickness
				ang := math.Atan2(float64(y)-cy, float64(x)-cx)
				wave := 0.84 + 0.16*math.Sin(ang*3.0+phase*1.6) + 0.10*math.Cos(ang*5.0-phase*1.1)
				norm := float64(d) / (bd * wave)
				if norm >= 1.0 {
					continue
				}

				// Quadratic glass falloff + crisp specular inner rim right at edge (d <= 2)
				falloff := (1.0 - norm) * (1.0 - norm)
				alpha := falloff * intensity * 0.62

				// Apple Intelligence liquid spectrum: Cyan -> Indigo -> Purple -> Soft Rose
				t := 0.5 + 0.5*math.Sin(ang*2.0-phase)
				t2 := 0.5 + 0.5*math.Cos(ang*3.0+phase*0.8)
				r := 90.0 + 145.0*t2
				g := 125.0 + 100.0*(1.0-t)
				b := 255.0

				// Specular liquid glass edge highlight (white crystal refraction)
				if d <= 2 {
					spec := (1.0 - float64(d)/3.0) * 0.55
					r = r*(1-spec) + 255*spec
					g = g*(1-spec) + 255*spec
					b = b*(1-spec) + 255*spec
					alpha = math.Min(0.88, alpha+spec*0.45)
				}

				blendPixel(buf, rowOff+x*4, r, g, b, alpha)
			}
		}
	}

	// Top, Bottom, Left, Right perimeter bands
	drawBand(0, 0, w, borderDepth)
	drawBand(0, h-borderDepth, w, h)
	drawBand(0, borderDepth, borderDepth, h-borderDepth)
	drawBand(w-borderDepth, borderDepth, w, h-borderDepth)

	// 2. Top-Center Liquid Glass Dynamic Island Pill
	pillW := 188
	if st == StateAIThinking || st == StateAIActing {
		pillW = 244
	}
	pillH := 34
	pillX := (w - pillW) / 2
	pillY := 14
	rad := float64(pillH) * 0.5
	drawGlassPill(buf, w, h, pillX, pillY, pillW, pillH, rad, phase, st == StateAIThinking || st == StateAIActing)

	// 3. Refractive Liquid Glass Ripples on every remote touch or AI click!
	for _, rp := range ripples {
		prog := float64(now.Sub(rp.start).Milliseconds()) / 680.0
		if prog < 0 || prog >= 1 {
			continue
		}
		// Smooth ease-out cubic expansion
		ease := 1.0 - math.Pow(1.0-prog, 3.0)
		fade := 1.0 - prog
		outerR := 12.0 + ease*68.0
		innerR := 6.0 + ease*44.0

		minX := rp.x - int(outerR) - 6
		maxX := rp.x + int(outerR) + 6
		minY := rp.y - int(outerR) - 6
		maxY := rp.y + int(outerR) + 6
		if minX < 0 {
			minX = 0
		}
		if minY < 0 {
			minY = 0
		}
		if maxX > w {
			maxX = w
		}
		if maxY > h {
			maxY = h
		}

		for y := minY; y < maxY; y++ {
			dy := float64(y - rp.y)
			rowOff := y * w * 4
			for x := minX; x < maxX; x++ {
				dx := float64(x - rp.x)
				dist := math.Hypot(dx, dy)
				if dist > outerR+5 {
					continue
				}

				// Outer liquid glass ring
				dOuter := math.Abs(dist - outerR)
				dInner := math.Abs(dist - innerR)
				var a float64
				var r, g, b float64
				if dOuter < 4.5 {
					ring := (1.0 - dOuter/4.5)
					a = ring * fade * 0.85
					if rp.isAI {
						r, g, b = 191, 90, 242 // AI Violet-Purple
					} else {
						r, g, b = 110, 224, 255 // Liquid Glass Cyan
					}
				}
				if dInner < 3.0 {
					ring2 := (1.0 - dInner/3.0) * fade * 0.72
					if ring2 > a {
						a = ring2
						r, g, b = 255, 255, 255 // Specular crystal highlight
					}
				}
				// Subtle glass lens sheen inside the ripple
				if dist < outerR && a < 0.14*fade {
					sheen := (1.0 - dist/outerR) * fade * 0.16
					if sheen > a {
						a = sheen
						r, g, b = 140, 215, 255
					}
				}
				if a > 0.01 {
					blendPixel(buf, rowOff+x*4, r, g, b, a)
				}
			}
		}
	}
}

func drawGlassPill(buf []byte, w, h, px, py, pw, ph int, rad, phase float64, aiActive bool) {
	// Bounding box with small aura margin
	margin := 10
	x0, y0 := px-margin, py-margin
	x1, y1 := px+pw+margin, py+ph+margin
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > w {
		x1 = w
	}
	if y1 > h {
		y1 = h
	}

	leftCX := float64(px) + rad
	rightCX := float64(px+pw) - rad
	midY := float64(py) + rad
	orbCX := float64(px) + 19.0
	orbCY := midY

	for y := y0; y < y1; y++ {
		rowOff := y * w * 4
		fy := float64(y)
		for x := x0; x < x1; x++ {
			fx := float64(x)
			// Signed distance to rounded capsule
			cx := fx
			if cx < leftCX {
				cx = leftCX
			} else if cx > rightCX {
				cx = rightCX
			}
			sdf := math.Hypot(fx-cx, fy-midY) - rad

			if sdf > float64(margin) {
				continue
			}

			// Ambient liquid aura outside the pill when AI is active
			if sdf > 0 {
				glowStr := 0.20
				if aiActive {
					glowStr = 0.45
				}
				glow := (1.0 - sdf/float64(margin))
				glow = glow * glow * glowStr
				ang := math.Atan2(fy-midY, fx-float64(px+pw/2))
				t := 0.5 + 0.5*math.Sin(ang*2.0+phase*1.5)
				blendPixel(buf, rowOff+x*4, 95+100*t, 120+90*(1-t), 255, glow)
				continue
			}

			// Inside the frosted obsidian glass capsule
			topGloss := math.Max(0, 1.0-(fy-float64(py))/12.0) * 0.16
			r, g, b, a := 16.0+255*topGloss, 18.0+255*topGloss, 28.0+255*topGloss, 0.74

			// Specular glass rim (1.5px border)
			if sdf > -1.8 {
				rim := 0.34
				if aiActive {
					rim = 0.58
				}
				r = r*(1-rim) + 210*rim
				g = g*(1-rim) + 235*rim
				b = b*(1-rim) + 255*rim
				a = 0.88
			}

			// Animated AI Orb / status jewel on the left side of the capsule
			dOrb := math.Hypot(fx-orbCX, fy-orbCY)
			if dOrb < 8.5 {
				pulse := 0.82 + 0.18*math.Sin(phase*2.2)
				if dOrb < 6.0 {
					if aiActive {
						t := 0.5 + 0.5*math.Sin(phase*2.5+dOrb*0.5)
						r, g, b, a = 100+110*t, 190-80*t, 255, 0.96
					} else {
						r, g, b, a = 48*pulse, 215*pulse, 120*pulse, 0.95
					}
				} else {
					glowO := (1.0 - (dOrb-6.0)/2.5) * 0.55
					r, g, b, a = 100, 210, 255, math.Max(a, glowO)
				}
			}

			// Subtle liquid waveform bars inside the capsule (alive indicator!)
			barX := int(fx) - (px + 38)
			if barX >= 0 && barX < pw-52 && barX%7 < 3 {
				barIdx := float64(barX / 7)
				amp := 2.5
				if aiActive {
					amp = 7.5
				}
				halfH := 2.0 + math.Abs(math.Sin(phase*1.9+barIdx*0.55))*amp
				if math.Abs(fy-midY) <= halfH {
					r, g, b, a = 165, 225, 255, 0.82
				}
			}

			blendPixel(buf, rowOff+x*4, r, g, b, a)
		}
	}
}

func blendPixel(buf []byte, off int, r, g, b, a float64) {
	if a <= 0 {
		return
	}
	if a > 1 {
		a = 1
	}
	// Source-over blending in premultiplied BGRA
	invA := 1.0 - a
	dstB := float64(buf[off])
	dstG := float64(buf[off+1])
	dstR := float64(buf[off+2])
	dstA := float64(buf[off+3])

	outB := b*a + dstB*invA
	outG := g*a + dstG*invA
	outR := r*a + dstR*invA
	outA := a*255.0 + dstA*invA

	buf[off] = clampByte(outB)
	buf[off+1] = clampByte(outG)
	buf[off+2] = clampByte(outR)
	buf[off+3] = clampByte(outA)
}

func clampByte(v float64) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}
