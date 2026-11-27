package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/anydask/host/internal/agent"
	"github.com/anydask/host/internal/agent/providers"
	"github.com/anydask/host/internal/capture"
	"github.com/anydask/host/internal/input"
	"github.com/anydask/host/internal/overlay"
	"github.com/anydask/host/internal/protocol"
	hostrtc "github.com/anydask/host/internal/webrtc"
	"github.com/anydask/host/internal/webui"
	"github.com/pion/webrtc/v4"
)

func main() {
	addr := flag.String("addr", ":8787", "HTTP/Signaling listen address")
	fps := flag.Int("fps", 60, "Target video FPS (30 or 60)")
	bitrate := flag.Int("bitrate", 6500, "Target video bitrate in kbps")
	encoder := flag.String("encoder", "", "Optional FFmpeg H.264 encoder override (e.g. h264_nvenc, h264_videotoolbox, libx264)")
	ffmpegPath := flag.String("ffmpeg", "", "Optional full path to ffmpeg binary")
	tokenFlag := flag.String("token", "", "Optional fixed auth token (auto-generated & persisted if omitted)")
	noOverlay := flag.Bool("no-overlay", false, "Disable Liquid Glass desktop visual overlay")
	flag.Parse()

	token, err := loadOrCreateToken(*tokenFlag)
	if err != nil {
		log.Fatalf("failed to initialize auth token: %v", err)
	}

	injector := input.NewInjector()
	bounds := injector.ScreenBounds()
	arbiter := input.NewArbiter(injector)
	desktopOverlay := overlay.New(!*noOverlay)
	defer desktopOverlay.Close()

	streamer := capture.NewStreamer(capture.Config{
		Width:       bounds.Width,
		Height:      bounds.Height,
		FPS:         *fps,
		BitrateKbps: *bitrate,
		Encoder:     *encoder,
		FFmpegPath:  *ffmpegPath,
	})

	providerFactory := func(req protocol.AIStartTaskPayload) (agent.LLMProvider, error) {
		switch strings.ToLower(strings.TrimSpace(req.Provider)) {
		case "anthropic", "claude":
			return providers.NewAnthropic(req.APIKey, req.Model, req.BaseURL), nil
		case "gemini", "google":
			return providers.NewGemini(req.APIKey, req.Model, req.BaseURL), nil
		case "openai", "openrouter", "openai-compatible":
			return providers.NewOpenAICompat(req.APIKey, req.Model, req.BaseURL), nil
		default:
			return nil, errors.New("provedor de IA não suportado: " + req.Provider)
		}
	}

	orchestrator := agent.NewOrchestrator(arbiter, providerFactory, desktopOverlay)
	sessions := hostrtc.NewSessionManager(streamer, arbiter, orchestrator, desktopOverlay, *fps)

	mux := http.NewServeMux()

	// Static PWA Apple Glass UI
	fileServer := http.FileServer(http.FS(webui.StaticFS()))
	mux.Handle("/", fileServer)

	// Health / Host Discovery endpoint (used by Android Host Switcher)
	mux.HandleFunc("/api/health", withCORS(func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		osName := runtime.GOOS
		if osName == "darwin" {
			osName = "macos"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"hostname":     host,
			"os":           osName,
			"width":        bounds.Width,
			"height":       bounds.Height,
			"fps":          *fps,
			"encoder":      streamer.EncoderName(),
			"capture_mode": streamer.CaptureMode(),
			"ts":           time.Now().UnixMilli(),
		})
	}))

	// Serve default AI config stored locally in ~/.anydask/ai-config.json (protected by pairing token)
	mux.HandleFunc("/api/ai-defaults", withCORS(func(w http.ResponseWriter, r *http.Request) {
		reqToken := r.Header.Get("X-AnyDask-Token")
		if reqToken == "" {
			reqToken = r.URL.Query().Get("token")
		}
		if reqToken != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		cfg := loadDefaultAIConfig()
		writeJSON(w, http.StatusOK, cfg)
	}))

	// One-tap Android APK download directly from the Host Agent
	mux.HandleFunc("/download/anydask.apk", func(w http.ResponseWriter, r *http.Request) {
		apkPath := findAPKPath()
		if apkPath == "" {
			http.Error(w, "APK not built yet", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.android.package-archive")
		w.Header().Set("Content-Disposition", `attachment; filename="AnyDask.apk"`)
		http.ServeFile(w, r, apkPath)
	})

	// 1-roundtrip WebRTC Signaling endpoint (protected by pairing token)
	mux.HandleFunc("/api/signal", withCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		reqToken := r.Header.Get("X-AnyDask-Token")
		if reqToken == "" {
			reqToken = r.URL.Query().Get("token")
		}
		if reqToken != token {
			http.Error(w, "unauthorized: invalid pairing token", http.StatusUnauthorized)
			return
		}

		var offer webrtc.SessionDescription
		if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
			http.Error(w, "invalid SDP offer: "+err.Error(), http.StatusBadRequest)
			return
		}

		_, answer, err := sessions.CreateAnswer(offer)
		if err != nil {
			log.Printf("[signal] error creating answer: %v", err)
			http.Error(w, "webrtc error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, answer)
	}))

	printBanner(*addr, token, bounds, *fps, streamer.CaptureMode(), streamer.EncoderName())

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	overlay.RunMainLoop(func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("server stopped: %v", err)
		}
	})
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-AnyDask-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func loadDefaultAIConfig() map[string]any {
	out := map[string]any{
		"provider":        "gemini",
		"model":           "gemini-3-flash-preview",
		"api_key":         "",
		"base_url":        "",
		"require_confirm": true,
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return out
	}
	b, err := os.ReadFile(filepath.Join(home, ".anydask", "ai-config.json"))
	if err != nil {
		return out
	}
	// Strip UTF-8 BOM if present (PowerShell Set-Content UTF8)
	b = []byte(strings.TrimPrefix(string(b), "\uFEFF"))
	var parsed map[string]any
	if json.Unmarshal(b, &parsed) == nil {
		for k, v := range parsed {
			out[k] = v
		}
	}
	return out
}

func findAPKPath() string {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(exeDir, "anydask-android.apk"),
		filepath.Join(exeDir, "AnyDask.apk"),
		filepath.Join(".", "bin", "anydask-android.apk"),
		filepath.Join("..", "bin", "anydask-android.apk"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func loadOrCreateToken(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return randomToken(), nil
	}
	dir := filepath.Join(home, ".anydask")
	_ = os.MkdirAll(dir, 0o700)
	tokPath := filepath.Join(dir, "token")
	if b, err := os.ReadFile(tokPath); err == nil {
		t := strings.TrimSpace(string(b))
		if len(t) >= 12 {
			return t, nil
		}
	}
	t := randomToken()
	_ = os.WriteFile(tokPath, []byte(t+"\n"), 0o600)
	return t, nil
}

func randomToken() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func printBanner(addr, token string, bounds input.ScreenBounds, fps int, capMode, encoder string) {
	port := strings.TrimPrefix(addr, ":")
	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		port = parts[len(parts)-1]
	}
	host, _ := os.Hostname()
	fmt.Println("================================================================")
	fmt.Printf("  AnyDask Host Agent (%s | %s/%s)\n", host, runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Display : %dx%d @ %d FPS\n", bounds.Width, bounds.Height, fps)
	fmt.Printf("  Video   : %s -> %s (H.264 Ultra-Low Latency)\n", capMode, encoder)
	fmt.Printf("  Token   : %s\n", token)
	fmt.Println("----------------------------------------------------------------")
	fmt.Println("  Abra no Chrome do Android (e toque em 'Adicionar a Tela Inicial'):")

	ips := listLocalIPs()
	if len(ips) == 0 {
		fmt.Printf("  -> http://localhost:%s/?token=%s\n", port, token)
	}
	for _, item := range ips {
		fmt.Printf("  [%s] http://%s:%s/?token=%s\n", item.label, item.ip, port, token)
	}
	fmt.Println("================================================================")
}

type ipEntry struct {
	label string
	ip    string
}

func listLocalIPs() []ipEntry {
	var out []ipEntry
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil {
				continue
			}
			s := ip4.String()
			label := "LAN"
			// Tailscale CGNAT range: 100.64.0.0/10
			if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
				label = "TAILSCALE"
			}
			out = append(out, ipEntry{label: label, ip: s})
		}
	}
	return out
}
