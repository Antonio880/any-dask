package capture

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4/pkg/media"
)

// Config configures the low-latency H.264 video capture pipeline.
type Config struct {
	Width       int
	Height      int
	FPS         int
	BitrateKbps int
	MaxWidth    int // 1920 (HD) or 1280 (Turbo ultra-low latency)
	FFmpegPath  string
	Encoder     string // optional override; auto-detected if empty
}

// SnapshotResult holds a resized screenshot and its dimensions for accurate CoordinateMapper projection.
type SnapshotResult struct {
	JPEGBytes []byte
	Width     int
	Height    int
	ScreenW   int
	ScreenH   int
}

// Streamer broadcasts H.264 Annex-B samples to all active WebRTC video subscribers.
// When no subscribers are connected, capture stops automatically (0% CPU/GPU idle).
type Streamer struct {
	cfg          Config
	mu           sync.Mutex
	subs         map[uint64]func(media.Sample) error
	nextID       uint64
	cancel       context.CancelFunc
	procCancel   context.CancelFunc
	running      bool
	encoderName  string
	captureMode  string
	displayIndex int
	displayX     int
	displayY     int
	displayW     int
	displayH     int
	maxWidth     int
	bitrateKbps  int
	lastSPS      []byte
	lastPPS      []byte
}

func NewStreamer(cfg Config) *Streamer {
	if cfg.FPS <= 0 {
		cfg.FPS = 60
	}
	if cfg.BitrateKbps <= 0 {
		cfg.BitrateKbps = 6500
	}
	if cfg.MaxWidth <= 0 {
		cfg.MaxWidth = 1920
	}
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = findFFmpeg()
	}
	s := &Streamer{
		cfg:         cfg,
		subs:        make(map[uint64]func(media.Sample) error),
		displayW:    cfg.Width,
		displayH:    cfg.Height,
		maxWidth:    cfg.MaxWidth,
		bitrateKbps: cfg.BitrateKbps,
	}
	s.encoderName = s.detectEncoder()
	s.captureMode = s.detectCaptureMode()
	return s
}

func (s *Streamer) EncoderName() string { return s.encoderName }
func (s *Streamer) CaptureMode() string { return s.captureMode }

// SwitchDisplay switches capture live to another monitor without dropping the WebRTC session.
func (s *Streamer) SwitchDisplay(index, x, y, w, h int) {
	s.mu.Lock()
	s.displayIndex = index
	s.displayX = x
	s.displayY = y
	s.displayW = w
	s.displayH = h
	pc := s.procCancel
	s.mu.Unlock()
	if pc != nil {
		pc() // Triggers immediate FFmpeg restart on the newly selected monitor (<250ms)
	}
}

// SetQualityProfile switches live between "turbo" (720p ultra-responsive) and "hd" (1080p sharp).
func (s *Streamer) SetQualityProfile(profile string) {
	s.mu.Lock()
	if profile == "turbo" {
		s.maxWidth = 1280
		s.bitrateKbps = 4200
	} else {
		s.maxWidth = 1920
		s.bitrateKbps = 6500
	}
	pc := s.procCancel
	s.mu.Unlock()
	if pc != nil {
		pc()
	}
}

// Subscribe registers a WebRTC video track writer and starts capture on demand.
func (s *Streamer) Subscribe(writer func(media.Sample) error) func() {
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.subs[id] = writer
	needStart := !s.running
	if needStart {
		s.running = true
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		go s.runLoop(ctx)
	}
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		delete(s.subs, id)
		if len(s.subs) == 0 && s.running {
			if s.cancel != nil {
				s.cancel()
			}
			s.running = false
		}
		s.mu.Unlock()
	}
}

// RequestKeyframe restarts the encoder gently if needed or relies on short GOP (1s).
func (s *Streamer) RequestKeyframe() {
	// With GOP = FPS (1s) and SPS/PPS caching, new peers lock on in <1s without stalling existing streams.
}

func (s *Streamer) runLoop(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	useFallbackCapture := false
	useFallbackEncoder := false
	useFastPixFmt := true

	for {
		if ctx.Err() != nil {
			return
		}
		procCtx, procCancel := context.WithCancel(ctx)
		s.mu.Lock()
		s.procCancel = procCancel
		s.mu.Unlock()

		err := s.streamOnce(procCtx, useFallbackCapture, useFallbackEncoder, useFastPixFmt)
		procCancel()
		if ctx.Err() != nil {
			return
		}
		// If canceled intentionally by SwitchDisplay / SetQualityProfile, restart immediately without fallback!
		if procCtx.Err() != nil {
			continue
		}
		log.Printf("[capture] stream exited: %v", err)
		if runtime.GOOS == "darwin" && useFastPixFmt {
			log.Printf("[capture] retrying avfoundation without explicit nv12 pixel_format")
			useFastPixFmt = false
		} else if !useFallbackEncoder && s.encoderName != "libx264" {
			log.Printf("[capture] falling back from %s to libx264 (keeping %s)", s.encoderName, s.captureMode)
			useFallbackEncoder = true
			s.encoderName = "libx264"
		} else if !useFallbackCapture && s.captureMode == "ddagrab" {
			log.Printf("[capture] falling back from ddagrab to gdigrab")
			useFallbackCapture = true
			s.captureMode = "gdigrab"
		} else {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}
}

func (s *Streamer) streamOnce(ctx context.Context, fallbackCapture, fallbackEncoder, fastPixFmt bool) error {
	args := s.buildFFmpegArgs(fallbackCapture, fallbackEncoder, fastPixFmt)
	log.Printf("[capture] starting ffmpeg (%s + %s): %s %s",
		s.captureMode, s.encoderName, s.cfg.FFmpegPath, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, s.cfg.FFmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}

	frameDur := time.Second / time.Duration(s.cfg.FPS)
	parseErr := s.parseAnnexB(ctx, stdout, frameDur)

	_ = cmd.Process.Kill()
	waitErr := cmd.Wait()
	if parseErr != nil && ctx.Err() == nil {
		return fmt.Errorf("parse: %v (stderr: %s)", parseErr, tailString(stderrBuf.String(), 300))
	}
	if waitErr != nil && ctx.Err() == nil {
		return fmt.Errorf("ffmpeg exit: %v (stderr: %s)", waitErr, tailString(stderrBuf.String(), 300))
	}
	return nil
}

func (s *Streamer) buildFFmpegArgs(fallbackCapture, fallbackEncoder, fastPixFmt bool) []string {
	s.mu.Lock()
	dispIdx := s.displayIndex
	dispX, dispY, dispW, dispH := s.displayX, s.displayY, s.displayW, s.displayH
	maxW := s.maxWidth
	kbps := s.bitrateKbps
	s.mu.Unlock()

	if maxW <= 0 {
		maxW = 1920
	}
	if kbps <= 0 {
		kbps = 6500
	}

	fpsStr := strconv.Itoa(s.cfg.FPS)
	gop := s.cfg.FPS / 2
	if gop < 15 {
		gop = 15
	}
	gopStr := strconv.Itoa(gop)
	bitrate := fmt.Sprintf("%dk", kbps)
	maxrate := fmt.Sprintf("%dk", int(float64(kbps)*1.25))
	bufsize := fmt.Sprintf("%dk", kbps/2)

	enc := s.encoderName
	if fallbackEncoder {
		enc = "libx264"
	}

	var args []string
	args = append(args,
		"-hide_banner", "-loglevel", "error",
		"-fflags", "nobuffer", "-flags", "low_delay",
		"-probesize", "32", "-analyzeduration", "0",
	)

	pixFmt := "yuv420p"
	if enc == "h264_amf" || enc == "h264_mf" || enc == "h264_qsv" || enc == "h264_videotoolbox" {
		pixFmt = "nv12"
	}
	scaleFilter := fmt.Sprintf("scale='min(%d,iw)':-2:flags=fast_bilinear", maxW)

	// 1. Input capture per OS (with native multi-monitor selection!)
	switch runtime.GOOS {
	case "windows":
		if !fallbackCapture && s.captureMode == "ddagrab" {
			args = append(args,
				"-init_hw_device", "d3d11va",
				"-filter_complex", fmt.Sprintf("ddagrab=output_idx=%d:framerate=%s:draw_mouse=1,hwdownload,format=bgra,%s,format=%s", dispIdx, fpsStr, scaleFilter, pixFmt),
			)
		} else {
			args = append(args,
				"-f", "gdigrab",
				"-framerate", fpsStr,
				"-draw_mouse", "1",
			)
			if dispW > 0 && dispH > 0 {
				args = append(args,
					"-offset_x", strconv.Itoa(dispX),
					"-offset_y", strconv.Itoa(dispY),
					"-video_size", fmt.Sprintf("%dx%d", dispW, dispH),
				)
			}
			args = append(args,
				"-i", "desktop",
				"-vf", fmt.Sprintf("%s,format=%s", scaleFilter, pixFmt),
			)
		}
	case "darwin":
		screenIdx := detectDarwinScreenIndex(s.cfg.FFmpegPath, dispIdx)
		args = append(args, "-f", "avfoundation", "-framerate", fpsStr, "-capture_cursor", "1")
		if fastPixFmt {
			args = append(args, "-pixel_format", "nv12")
		}
		args = append(args,
			"-i", screenIdx+":none",
			"-vf", fmt.Sprintf("fps=%s,%s,format=%s", fpsStr, scaleFilter, pixFmt),
		)
	default:
		args = append(args,
			"-f", "x11grab",
			"-framerate", fpsStr,
			"-i", ":0.0",
			"-vf", fmt.Sprintf("%s,format=%s", scaleFilter, pixFmt),
		)
	}

	// 2. Encoder (zero B-frames, low-latency, Annex-B SPS/PPS repeat)
	switch enc {
	case "h264_nvenc":
		args = append(args,
			"-c:v", "h264_nvenc",
			"-preset", "p1",
			"-tune", "ull",
			"-profile:v", "baseline",
			"-rc", "cbr",
			"-bf", "0",
			"-zerolatency", "1",
			"-g", gopStr,
			"-b:v", bitrate,
			"-maxrate", maxrate,
			"-bufsize", bufsize,
		)
	case "h264_qsv":
		args = append(args,
			"-c:v", "h264_qsv",
			"-preset", "veryfast",
			"-profile:v", "baseline",
			"-low_power", "1",
			"-bf", "0",
			"-g", gopStr,
			"-b:v", bitrate,
			"-maxrate", maxrate,
			"-bufsize", bufsize,
		)
	case "h264_amf":
		args = append(args,
			"-c:v", "h264_amf",
			"-usage", "ultralowlatency",
			"-quality", "speed",
			"-profile:v", "constrained_baseline",
			"-header_insertion_mode", "gop",
			"-g", gopStr,
			"-b:v", bitrate,
			"-maxrate", maxrate,
			"-bufsize", bufsize,
		)
	case "h264_mf":
		args = append(args,
			"-c:v", "h264_mf",
			"-rate_control", "cbr",
			"-scenario", "display_remoting",
			"-g", gopStr,
			"-b:v", bitrate,
		)
	case "h264_videotoolbox":
		args = append(args,
			"-c:v", "h264_videotoolbox",
			"-realtime", "true",
			"-prio_speed", "true",
			"-profile:v", "baseline",
			"-allow_sw", "1",
			"-bf", "0",
			"-g", gopStr,
			"-b:v", bitrate,
			"-maxrate", maxrate,
			"-bufsize", bufsize,
		)
	default:
		args = append(args,
			"-c:v", "libx264",
			"-preset", "ultrafast",
			"-tune", "zerolatency",
			"-profile:v", "baseline",
			"-bf", "0",
			"-g", gopStr,
			"-keyint_min", gopStr,
			"-sc_threshold", "0",
			"-b:v", bitrate,
			"-maxrate", maxrate,
			"-bufsize", bufsize,
		)
	}

	args = append(args,
		"-bsf:v", "dump_extra",
		"-flush_packets", "1",
		"-f", "h264",
		"pipe:1",
	)
	return args
}

// parseAnnexB reads directly from the OS pipe (zero bufio delay) and splits raw H.264 Annex-B into Access Units.
func (s *Streamer) parseAnnexB(ctx context.Context, r io.Reader, frameDur time.Duration) error {
	buf := make([]byte, 0, 512*1024)
	tmp := make([]byte, 64*1024)

	var au bytes.Buffer
	hasVCL := false
	lastSampleAt := time.Now()
	frameCount := 0

	flushAU := func() {
		if au.Len() == 0 {
			return
		}
		data := make([]byte, au.Len())
		copy(data, au.Bytes())
		au.Reset()
		hasVCL = false

		now := time.Now()
		dt := now.Sub(lastSampleAt)
		lastSampleAt = now
		if dt < 5*time.Millisecond {
			dt = frameDur
		} else if dt > 250*time.Millisecond {
			dt = 250 * time.Millisecond
		}

		frameCount++
		if frameCount == 1 || frameCount == 60 {
			log.Printf("[capture] streaming active (%s + %s): %d frames sent", s.captureMode, s.encoderName, frameCount)
		}

		sample := media.Sample{
			Data:     data,
			Duration: dt,
		}
		s.broadcast(sample)
	}

	processNAL := func(nal []byte) {
		if len(nal) == 0 {
			return
		}
		nalType := nal[0] & 0x1F
		switch nalType {
		case 7: // SPS
			if hasVCL {
				flushAU()
			}
			s.mu.Lock()
			s.lastSPS = append([]byte(nil), nal...)
			s.mu.Unlock()
			au.Write([]byte{0, 0, 0, 1})
			au.Write(nal)
		case 8: // PPS
			s.mu.Lock()
			s.lastPPS = append([]byte(nil), nal...)
			s.mu.Unlock()
			au.Write([]byte{0, 0, 0, 1})
			au.Write(nal)
		case 5: // IDR slice (keyframe)
			if hasVCL {
				flushAU()
			}
			// Guarantee SPS/PPS precede IDR for fast WebRTC decoder lock-on.
			if au.Len() == 0 {
				s.mu.Lock()
				sps, pps := s.lastSPS, s.lastPPS
				s.mu.Unlock()
				if len(sps) > 0 && len(pps) > 0 {
					au.Write([]byte{0, 0, 0, 1})
					au.Write(sps)
					au.Write([]byte{0, 0, 0, 1})
					au.Write(pps)
				}
			}
			au.Write([]byte{0, 0, 0, 1})
			au.Write(nal)
			hasVCL = true
		case 1: // Non-IDR slice (P-frame)
			if hasVCL {
				flushAU()
			}
			au.Write([]byte{0, 0, 0, 1})
			au.Write(nal)
			hasVCL = true
		case 9: // AUD
			if hasVCL {
				flushAU()
			}
		default:
			au.Write([]byte{0, 0, 0, 1})
			au.Write(nal)
		}
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			for {
				start, startLen := findStartCode(buf, 0)
				if start < 0 {
					if len(buf) > 4 {
						buf = buf[len(buf)-4:]
					}
					break
				}
				next, _ := findStartCode(buf, start+startLen)
				if next < 0 {
					if start > 0 {
						buf = append(buf[:0], buf[start:]...)
					}
					break
				}
				nal := buf[start+startLen : next]
				processNAL(nal)
				buf = append(buf[:0], buf[next:]...)
			}
		}
		if err != nil {
			if err == io.EOF {
				if hasVCL {
					flushAU()
				}
				return io.EOF
			}
			return err
		}
	}
}

func (s *Streamer) broadcast(sample media.Sample) {
	s.mu.Lock()
	writers := make([]func(media.Sample) error, 0, len(s.subs))
	for _, w := range s.subs {
		writers = append(writers, w)
	}
	s.mu.Unlock()

	for _, w := range writers {
		_ = w(sample)
	}
}

func findStartCode(b []byte, from int) (int, int) {
	for i := from; i+3 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 {
			if b[i+2] == 1 {
				return i, 3
			}
			if b[i+2] == 0 && b[i+3] == 1 {
				return i, 4
			}
		}
	}
	return -1, 0
}

func (s *Streamer) detectEncoder() string {
	if s.cfg.Encoder != "" {
		return s.cfg.Encoder
	}
	out, err := exec.Command(s.cfg.FFmpegPath, "-hide_banner", "-encoders").CombinedOutput()
	if err != nil {
		return "libx264"
	}
	text := string(out)
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		// Test hardware encoders in priority order; libx264 ultrafast zerolatency is reliable fallback.
		candidates = []string{"h264_nvenc", "h264_qsv", "h264_amf", "h264_mf", "libx264"}
	case "darwin":
		candidates = []string{"h264_videotoolbox", "libx264"}
	default:
		candidates = []string{"h264_nvenc", "libx264"}
	}
	for _, c := range candidates {
		if !strings.Contains(text, c) {
			continue
		}
		if c == "libx264" {
			return c
		}
		// Quick 1-frame probe to ensure the actual GPU driver supports this hardware encoder.
		if probeEncoder(s.cfg.FFmpegPath, c) {
			return c
		}
	}
	return "libx264"
}

func probeEncoder(ffmpegPath, enc string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:s=256x256:d=0.1",
		"-frames:v", "1",
		"-c:v", enc,
		"-f", "null", "-",
	)
	return cmd.Run() == nil
}

func (s *Streamer) detectCaptureMode() string {
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command(s.cfg.FFmpegPath, "-hide_banner", "-filters").CombinedOutput()
		if err == nil && strings.Contains(string(out), "ddagrab") {
			return "ddagrab"
		}
		return "gdigrab"
	case "darwin":
		return "avfoundation"
	default:
		return "x11grab"
	}
}

func detectDarwinScreenIndex(ffmpegPath string, displayIndex int) string {
	target := fmt.Sprintf("Capture screen %d", displayIndex)
	out, _ := exec.Command(ffmpegPath, "-hide_banner", "-f", "avfoundation", "-list_devices", "true", "-i", "").CombinedOutput()
	var firstScreen string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "Capture screen ") {
			continue
		}
		lastOpen := strings.LastIndex(line, "[")
		lastClose := strings.LastIndex(line, "]")
		if lastOpen >= 0 && lastClose > lastOpen {
			idx := strings.TrimSpace(line[lastOpen+1 : lastClose])
			if _, err := strconv.Atoi(idx); err == nil {
				if firstScreen == "" {
					firstScreen = idx
				}
				if strings.Contains(line, target) {
					return idx
				}
			}
		}
	}
	if firstScreen != "" {
		return firstScreen
	}
	return "1"
}

func findFFmpeg() string {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		if localApp := strings.TrimSpace(getenv("LOCALAPPDATA")); localApp != "" {
			candidates = append(candidates, localApp+`\Microsoft\WinGet\Links\ffmpeg.exe`)
		}
		candidates = append(candidates, `C:\ProgramData\chocolatey\bin\ffmpeg.exe`)
	case "darwin":
		candidates = append(candidates, "/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg")
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return "ffmpeg"
}

func getenv(k string) string {
	return os.Getenv(k)
}

func tailString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[len(s)-maxLen:]
}

// ResizeAndEncodeJPEG resizes an RGBA image to fit within maxWidth while preserving aspect ratio
// and returns high-clarity JPEG bytes for Vision / Computer Use LLMs.
func ResizeAndEncodeJPEG(src image.Image, maxWidth int, quality int) ([]byte, int, int, error) {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	tw, th := sw, sh
	if maxWidth > 0 && sw > maxWidth {
		tw = maxWidth
		th = int(float64(sh) * float64(maxWidth) / float64(sw))
		if th < 1 {
			th = 1
		}
	}

	var target image.Image = src
	if tw != sw || th != sh {
		dst := image.NewRGBA(image.Rect(0, 0, tw, th))
		xRatio := float64(sw) / float64(tw)
		yRatio := float64(sh) / float64(th)
		for y := 0; y < th; y++ {
			sy := sb.Min.Y + int(float64(y)*yRatio)
			for x := 0; x < tw; x++ {
				sx := sb.Min.X + int(float64(x)*xRatio)
				dst.Set(x, y, src.At(sx, sy))
			}
		}
		target = dst
	}

	var out bytes.Buffer
	if quality <= 0 {
		quality = 82
	}
	if err := jpeg.Encode(&out, target, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, err
	}
	return out.Bytes(), tw, th, nil
}
