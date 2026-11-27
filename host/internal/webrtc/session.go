package webrtc

import (
	"encoding/json"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/anydask/host/internal/agent"
	"github.com/anydask/host/internal/capture"
	"github.com/anydask/host/internal/input"
	"github.com/anydask/host/internal/osctx"
	"github.com/anydask/host/internal/overlay"
	"github.com/anydask/host/internal/protocol"
	"github.com/pion/webrtc/v4"
)

type SessionManager struct {
	streamer     *capture.Streamer
	arbiter      *input.Arbiter
	orchestrator *agent.Orchestrator
	overlay      overlay.Overlay
	fps          int

	mu          sync.Mutex
	activePeers int
}

func NewSessionManager(streamer *capture.Streamer, arbiter *input.Arbiter, orch *agent.Orchestrator, ov overlay.Overlay, fps int) *SessionManager {
	return &SessionManager{
		streamer:     streamer,
		arbiter:      arbiter,
		orchestrator: orch,
		overlay:      ov,
		fps:          fps,
	}
}

type PeerSession struct {
	PC          *webrtc.PeerConnection
	controlDC   *webrtc.DataChannel
	controlMu   sync.Mutex
	unsubVideo  func()
	closeOnce   sync.Once
	countedPeer bool
	mgr         *SessionManager
}

// CreateAnswer creates a WebRTC PeerConnection with an H.264 video track and handles
// the client-created "input" (unordered binary) and "control" (ordered JSON) DataChannels.
func (m *SessionManager) CreateAnswer(offer webrtc.SessionDescription) (*PeerSession, webrtc.SessionDescription, error) {
	me := &webrtc.MediaEngine{}
	if err := me.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, webrtc.SessionDescription{}, err
	}

	se := webrtc.SettingEngine{}
	// ICE-Lite makes WebRTC over Tailscale/VPN/4G connect instantly: the Host responds
	// immediately to STUN checks arriving from Android's tun0 interface without needing
	// Android to expose its VPN IP in the SDP offer.
	se.SetLite(true)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetInterfaceFilter(func(ifaceName string) bool {
		lower := strings.ToLower(ifaceName)
		// Skip virtual Hyper-V / WSL / Docker bridges that are unreachable from the phone
		if strings.Contains(lower, "vethernet") || strings.Contains(lower, "wsl") || strings.Contains(lower, "docker") || strings.Contains(lower, "loopback") {
			return false
		}
		return true
	})

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(me),
		webrtc.WithSettingEngine(se),
	)
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	})
	if err != nil {
		return nil, webrtc.SessionDescription{}, err
	}

	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"screen",
		"anydask-screen",
	)
	if err != nil {
		_ = pc.Close()
		return nil, webrtc.SessionDescription{}, err
	}

	rtpSender, err := pc.AddTrack(videoTrack)
	if err != nil {
		_ = pc.Close()
		return nil, webrtc.SessionDescription{}, err
	}

	// Read incoming RTCP packets (PLI/NACK/REMB) so pion/webrtc interceptors never stall video delivery
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	sess := &PeerSession{
		PC:  pc,
		mgr: m,
	}

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[webrtc] ice state -> %s", state.String())
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("[webrtc] peer state -> %s", state.String())
		switch state {
		case webrtc.PeerConnectionStateConnected:
			sess.startVideo(videoTrack)
			sess.markConnected()
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			// Fail-closed safety: if the phone disconnects while AI is acting, abort AI immediately!
			m.orchestrator.Abort(nil)
			sess.Close()
		}
	})

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("[webrtc] datachannel opened: %s", dc.Label())
		switch dc.Label() {
		case "input":
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				ev, err := protocol.DecodeBinaryInput(msg.Data)
				if err != nil {
					return
				}
				if m.overlay != nil && ev.Type == protocol.BinMouseButton &&
					(ev.Action == protocol.MouseActionClick || ev.Action == protocol.MouseActionDblClick || ev.Action == protocol.MouseActionDown) &&
					ev.XNorm != 0xFFFF && ev.YNorm != 0xFFFF {
					rx, ry := m.arbiter.MapNormalized(ev.XNorm, ev.YNorm)
					m.overlay.TriggerRipple(rx, ry, false)
				}
				_ = m.arbiter.HandleManualBinary(ev)
			})
		case "control":
			sess.controlMu.Lock()
			sess.controlDC = dc
			sess.controlMu.Unlock()

			dc.OnOpen(func() {
				sess.sendHostInfo()
			})
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				sess.handleControlMessage(msg.Data)
			})
		}
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return nil, webrtc.SessionDescription{}, err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		return nil, webrtc.SessionDescription{}, err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		return nil, webrtc.SessionDescription{}, err
	}

	select {
	case <-gatherComplete:
	case <-time.After(2 * time.Second):
	}

	return sess, *pc.LocalDescription(), nil
}

func (s *PeerSession) startVideo(track *webrtc.TrackLocalStaticSample) {
	s.controlMu.Lock()
	already := s.unsubVideo != nil
	if !already {
		s.unsubVideo = s.mgr.streamer.Subscribe(track.WriteSample)
	}
	s.controlMu.Unlock()
}

func (s *PeerSession) markConnected() {
	s.mgr.mu.Lock()
	if !s.countedPeer {
		s.countedPeer = true
		s.mgr.activePeers++
		if s.mgr.activePeers == 1 && s.mgr.overlay != nil {
			s.mgr.overlay.SetState(overlay.StateConnected, "AnyDask · Conectado")
		}
	}
	s.mgr.mu.Unlock()
}

func (s *PeerSession) Close() {
	s.closeOnce.Do(func() {
		s.controlMu.Lock()
		unsub := s.unsubVideo
		s.unsubVideo = nil
		s.controlMu.Unlock()
		if unsub != nil {
			unsub()
		}
		s.mgr.mu.Lock()
		if s.countedPeer {
			s.countedPeer = false
			if s.mgr.activePeers > 0 {
				s.mgr.activePeers--
			}
			if s.mgr.activePeers == 0 && s.mgr.overlay != nil {
				s.mgr.overlay.SetState(overlay.StateHidden, "")
			}
		}
		s.mgr.mu.Unlock()
		_ = s.PC.Close()
	})
}

func (s *PeerSession) SendControl(msgType string, payload any) {
	b, err := protocol.MarshalControl(msgType, time.Now().UnixMilli(), payload)
	if err != nil {
		return
	}
	s.controlMu.Lock()
	dc := s.controlDC
	s.controlMu.Unlock()
	if dc != nil && dc.ReadyState() == webrtc.DataChannelStateOpen {
		_ = dc.SendText(string(b))
	}
}

func (s *PeerSession) sendHostInfo() {
	host, _ := os.Hostname()
	b := s.mgr.arbiter.ScreenBounds()
	displays := s.mgr.arbiter.Displays()
	activeIdx := s.mgr.arbiter.ActiveDisplay()
	osName := runtime.GOOS
	primary := "Ctrl"
	if osName == "darwin" {
		osName = "macos"
		primary = "Cmd"
	}
	s.SendControl("host.info", protocol.HostInfo{
		Hostname:      host,
		OS:            osName,
		Arch:          runtime.GOARCH,
		Width:         b.Width,
		Height:        b.Height,
		FPS:           s.mgr.fps,
		Encoder:       s.mgr.streamer.EncoderName(),
		CaptureMode:   s.mgr.streamer.CaptureMode(),
		PrimaryMod:    primary,
		ActiveDisplay: activeIdx,
		Displays:      displays,
	})
}

func (s *PeerSession) handleControlMessage(raw []byte) {
	var msg protocol.ControlMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "ping":
		s.SendControl("pong", map[string]any{"client_ts": msg.TS})

	case "video.request_keyframe":
		s.mgr.streamer.RequestKeyframe()

	case "key.event":
		var p protocol.KeyEventPayload
		if json.Unmarshal(msg.Payload, &p) == nil {
			_ = s.mgr.arbiter.HandleManualKey(p)
		}

	case "key.text":
		var p protocol.KeyTextPayload
		if json.Unmarshal(msg.Payload, &p) == nil {
			_ = s.mgr.arbiter.HandleManualText(p.Text)
		}

	case "clipboard.set":
		var p protocol.ClipboardPayload
		if json.Unmarshal(msg.Payload, &p) == nil {
			_ = osctx.SetClipboard(p.Text)
		}

	case "ai.start_task":
		var p protocol.AIStartTaskPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			s.SendControl("ai.status", protocol.AIStatusPayload{State: "ERROR", Message: "Payload inválido"})
			return
		}
		if err := s.mgr.orchestrator.StartTask(p, s.SendControl); err != nil {
			s.SendControl("ai.status", protocol.AIStatusPayload{State: "ERROR", Message: err.Error()})
		}

	case "ai.abort":
		// Highest priority Kill Switch!
		s.mgr.orchestrator.Abort(s.SendControl)

	case "ai.confirm_response":
		var p protocol.AIConfirmResponsePayload
		if json.Unmarshal(msg.Payload, &p) == nil {
			s.mgr.orchestrator.HandleConfirmResponse(p)
		}

	case "overlay.toggle":
		if s.mgr.overlay != nil {
			next := !s.mgr.overlay.IsEnabled()
			s.mgr.overlay.SetEnabled(next)
			s.SendControl("overlay.state", map[string]any{"enabled": next})
		}

	case "display.switch":
		var p protocol.DisplaySwitchPayload
		if json.Unmarshal(msg.Payload, &p) == nil {
			disp := s.mgr.arbiter.SetActiveDisplay(p.Index)
			s.mgr.streamer.SwitchDisplay(disp.Index, disp.X, disp.Y, disp.Width, disp.Height)
			log.Printf("[webrtc] switched active display -> #%d (%s)", disp.Index, disp.Name)
			s.sendHostInfo()
		}

	case "video.quality":
		var p struct {
			Profile string `json:"profile"` // "turbo" | "hd"
		}
		if json.Unmarshal(msg.Payload, &p) == nil {
			s.mgr.streamer.SetQualityProfile(p.Profile)
			log.Printf("[webrtc] switched video quality profile -> %s", p.Profile)
			s.SendControl("video.quality_state", map[string]any{"profile": p.Profile})
		}
	}
}
