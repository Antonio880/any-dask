(() => {
  "use strict";

  // Binary input opcodes (match protocol/input-binary-spec.md)
  const BIN_MOUSE_MOVE_REL = 0x01;
  const BIN_MOUSE_MOVE_ABS = 0x02;
  const BIN_MOUSE_BUTTON = 0x03;
  const BIN_MOUSE_SCROLL = 0x04;

  const BTN_LEFT = 1, BTN_RIGHT = 2, BTN_MIDDLE = 3;
  const ACT_UP = 0, ACT_DOWN = 1, ACT_CLICK = 2, ACT_DBLCLICK = 3;

  const DEFAULT_MODELS = {
    gemini: "gemini-3-flash-preview",
    anthropic: "claude-sonnet-4-5-20250929",
    openai: "gpt-4o",
  };

  // DOM refs
  const stage = document.getElementById("stage");
  const video = document.getElementById("remoteVideo");
  const clickHalo = document.getElementById("clickHalo");
  const touchRipple = document.getElementById("touchRipple");
  const hiddenKeyboard = document.getElementById("hiddenKeyboard");

  const connectOverlay = document.getElementById("connectOverlay");
  const connectStatusText = document.getElementById("connectStatusText");
  const connectForm = document.getElementById("connectForm");
  const connectUrlInput = document.getElementById("connectUrlInput");
  const connectTokenInput = document.getElementById("connectTokenInput");

  const dynamicIsland = document.getElementById("dynamicIsland");
  const islandTitle = document.getElementById("islandTitle");
  const islandSub = document.getElementById("islandSub");
  const btnKillSwitch = document.getElementById("btnKillSwitch");

  const aiOrb = document.getElementById("aiOrb");
  const botEyesMini = document.getElementById("botEyesMini");
  const botHero = document.getElementById("botHero");
  const botHeroTitle = document.getElementById("botHeroTitle");
  const botHeroSub = document.getElementById("botHeroSub");
  const botWaveCanvas = document.getElementById("botWaveCanvas");
  const aiOverlay = document.getElementById("aiOverlay");
  const aiHostLabel = document.getElementById("aiHostLabel");
  const btnQuickModel = document.getElementById("btnQuickModel");
  const aiFeed = document.getElementById("aiFeed");
  const aiPromptInput = document.getElementById("aiPromptInput");
  const btnVoiceToggle = document.getElementById("btnVoiceToggle");

  const confirmModal = document.getElementById("confirmModal");
  const confirmReason = document.getElementById("confirmReason");
  const confirmCommand = document.getElementById("confirmCommand");

  const devBar = document.getElementById("devBar");
  const hudMiniStats = document.getElementById("hudMiniStats");
  const btnModPrimary = document.getElementById("btnModPrimary");
  const btnInputMode = document.getElementById("btnInputMode");
  const btnZoomToggle = document.getElementById("btnZoomToggle");
  const btnRotate = document.getElementById("btnRotate");
  const toastEl = document.getElementById("toast");

  // App state
  let pc = null;
  let inputDC = null;
  let controlDC = null;
  let pingTimer = null;
  let hostInfo = { hostname: "Host", os: "windows", primary_mod: "Ctrl", width: 1920, height: 1080 };
  let currentHost = null;
  // Direct Touch is now the default: tapping clicks exactly where your finger touches!
  let inputMode = localStorage.getItem("anydask_input_mode_v2") || "touch"; // "touch" | "trackpad"
  let stickyMods = new Set();
  let pendingConfirm = null;
  let aiState = "IDLE";
  let haloTimer = null;
  let toastTimer = null;

  // Viewport Zoom & Pan state (1.0x .. 4.0x)
  let viewZoom = 1.0;
  let panX = 0;
  let panY = 0;

  function applyViewportTransform() {
    if (viewZoom <= 1.01) {
      viewZoom = 1.0;
      panX = 0;
      panY = 0;
    } else {
      const maxX = (window.innerWidth * (viewZoom - 1)) / 2 + 120;
      const maxY = (window.innerHeight * (viewZoom - 1)) / 2 + 120;
      panX = Math.max(-maxX, Math.min(maxX, panX));
      panY = Math.max(-maxY, Math.min(maxY, panY));
    }
    video.style.transform = `translate3d(${panX}px, ${panY}px, 0) scale(${viewZoom})`;
    if (btnZoomToggle) {
      btnZoomToggle.textContent = viewZoom > 1.05 ? `${viewZoom.toFixed(1)}x` : "1x";
      btnZoomToggle.classList.toggle("active", viewZoom > 1.05);
    }
  }

  // Settings (BYOK)
  const settings = loadSettings();
  let savedHosts = loadHosts();

  function loadSettings() {
    try {
      const s = JSON.parse(localStorage.getItem("anydask_settings") || "{}");
      return {
        provider: s.provider || "gemini",
        model: s.model || DEFAULT_MODELS[s.provider || "gemini"],
        apiKey: s.apiKey || "",
        baseUrl: s.baseUrl || "",
        requireConfirm: s.requireConfirm !== false,
        sensitivity: Number(s.sensitivity || 1.8),
      };
    } catch {
      return { provider: "gemini", model: DEFAULT_MODELS.gemini, apiKey: "", baseUrl: "", requireConfirm: true, sensitivity: 1.8 };
    }
  }

  function saveSettings() {
    localStorage.setItem("anydask_settings", JSON.stringify(settings));
    updateQuickModelBadge();
  }

  function loadHosts() {
    try {
      return JSON.parse(localStorage.getItem("anydask_hosts") || "[]");
    } catch {
      return [];
    }
  }

  function saveHosts() {
    localStorage.setItem("anydask_hosts", JSON.stringify(savedHosts));
  }

  function showToast(msg, duration = 2600) {
    toastEl.textContent = msg;
    toastEl.classList.remove("hidden");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toastEl.classList.add("hidden"), duration);
  }

  function haptic(ms = 12) {
    if (window.AnyDaskNative && window.AnyDaskNative.haptic) {
      try { window.AnyDaskNative.haptic(ms); return; } catch {}
    }
    if (navigator.vibrate) navigator.vibrate(ms);
  }

  async function syncDefaultAIConfig(baseUrl, token) {
    if (settings.apiKey) return;
    try {
      const r = await fetch(`${baseUrl}/api/ai-defaults`, {
        headers: { "X-AnyDask-Token": token },
      });
      if (!r.ok) return;
      const d = await r.json();
      if (d && d.api_key) {
        settings.provider = d.provider || settings.provider;
        settings.model = d.model || settings.model;
        settings.apiKey = d.api_key;
        if (d.base_url) settings.baseUrl = d.base_url;
        saveSettings();
      }
    } catch {}
  }

  // ==================== WEBRTC CONNECTION ====================
  async function connectToHost(baseUrl, token, label) {
    disconnect();
    baseUrl = baseUrl.replace(/\/+$/, "");
    currentHost = { url: baseUrl, token, name: label || baseUrl };

    connectOverlay.classList.remove("hidden");
    connectStatusText.textContent = `Conectando em ${currentHost.name}...`;
    connectForm.classList.add("hidden");

    try {
      pc = new RTCPeerConnection({ iceServers: [] });

      // Receive-only H.264 video transceiver
      pc.addTransceiver("video", { direction: "recvonly" });

      pc.ontrack = (ev) => {
        if (ev.streams && ev.streams[0]) {
          video.srcObject = ev.streams[0];
        } else {
          const ms = new MediaStream([ev.track]);
          video.srcObject = ms;
        }
        video.play().catch(() => {});
        connectOverlay.classList.add("hidden");
      };

      pc.oniceconnectionstatechange = () => {
        if (!pc) return;
        const ist = pc.iceConnectionState;
        if (ist === "checking") {
          connectStatusText.textContent = `Fechando túnel WebRTC com ${currentHost.name}...`;
        } else if (ist === "failed") {
          connectStatusText.textContent = `Falha no túnel WebRTC (${currentHost.name}). Verifique se o Tailscale está ativo.`;
          connectUrlInput.value = baseUrl;
          connectTokenInput.value = token;
          connectForm.classList.remove("hidden");
        }
      };

      pc.onconnectionstatechange = () => {
        if (!pc) return;
        const st = pc.connectionState;
        if (st === "connected") {
          connectStatusText.textContent = `Conectado! Aguardando primeiro frame de vídeo...`;
          showToast(`Conectado a ${currentHost.name}`);
        } else if (st === "failed" || st === "disconnected") {
          hudMiniStats.textContent = "offline";
          showToast("Conexão interrompida — toque em Hosts para reconectar", 3500);
        }
      };

      // DataChannel 1: Unordered low-latency binary input
      inputDC = pc.createDataChannel("input", { ordered: false, maxRetransmits: 0 });
      inputDC.binaryType = "arraybuffer";

      // DataChannel 2: Ordered JSON control & AI agent orchestration
      controlDC = pc.createDataChannel("control", { ordered: true });
      controlDC.onopen = () => {
        startPingLoop();
      };
      controlDC.onmessage = (ev) => handleControlMessage(ev.data);

      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);

      // Wait briefly for local ICE host candidates
      await new Promise((resolve) => {
        if (pc.iceGatheringState === "complete") return resolve();
        const done = () => {
          if (!pc || pc.iceGatheringState === "complete") resolve();
        };
        pc.addEventListener("icegatheringstatechange", done);
        setTimeout(resolve, 900);
      });

      const res = await fetch(`${baseUrl}/api/signal`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-AnyDask-Token": token,
        },
        body: JSON.stringify(pc.localDescription),
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `HTTP ${res.status}`);
      }

      const answer = await res.json();
      await pc.setRemoteDescription(answer);

      // Remember host automatically & sync default AI key if configured on host
      upsertHost({ name: currentHost.name, url: baseUrl, token });
      localStorage.setItem("anydask_last_host", JSON.stringify({ url: baseUrl, token, name: currentHost.name }));
      syncDefaultAIConfig(baseUrl, token);
    } catch (err) {
      connectStatusText.textContent = `Erro ao conectar: ${err.message}`;
      connectUrlInput.value = baseUrl;
      connectTokenInput.value = token;
      connectForm.classList.remove("hidden");
    }
  }

  function disconnect() {
    clearInterval(pingTimer);
    if (inputDC) { try { inputDC.close(); } catch {} inputDC = null; }
    if (controlDC) { try { controlDC.close(); } catch {} controlDC = null; }
    if (pc) { try { pc.close(); } catch {} pc = null; }
  }

  function upsertHost(h) {
    const idx = savedHosts.findIndex((x) => x.url === h.url);
    if (idx >= 0) {
      savedHosts[idx] = { ...savedHosts[idx], ...h };
    } else {
      savedHosts.push(h);
    }
    saveHosts();
  }

  function startPingLoop() {
    clearInterval(pingTimer);
    pingTimer = setInterval(() => {
      sendControl("ping", null);
    }, 2000);
  }

  function sendControl(type, payload) {
    if (!controlDC || controlDC.readyState !== "open") return false;
    controlDC.send(JSON.stringify({ type, ts: Date.now(), payload }));
    return true;
  }

  function sendBinary(buf) {
    if (!inputDC || inputDC.readyState !== "open") return;
    inputDC.send(buf);
  }

  // ==================== BINARY INPUT ENCODERS ====================
  function sendMouseMoveRel(dx, dy) {
    const buf = new ArrayBuffer(6);
    const dv = new DataView(buf);
    dv.setUint8(0, BIN_MOUSE_MOVE_REL);
    dv.setInt16(1, clampInt16(dx), false);
    dv.setInt16(3, clampInt16(dy), false);
    dv.setUint8(5, 0);
    sendBinary(buf);
  }

  function sendMouseMoveAbs(rx, ry) {
    const buf = new ArrayBuffer(6);
    const dv = new DataView(buf);
    dv.setUint8(0, BIN_MOUSE_MOVE_ABS);
    dv.setUint16(1, Math.round(clamp01(rx) * 65535), false);
    dv.setUint16(3, Math.round(clamp01(ry) * 65535), false);
    dv.setUint8(5, 0);
    sendBinary(buf);
  }

  function sendMouseButton(button, action, rx = null, ry = null) {
    const buf = new ArrayBuffer(7);
    const dv = new DataView(buf);
    dv.setUint8(0, BIN_MOUSE_BUTTON);
    dv.setUint8(1, button);
    dv.setUint8(2, action);
    if (rx !== null && ry !== null) {
      dv.setUint16(3, Math.round(clamp01(rx) * 65535), false);
      dv.setUint16(5, Math.round(clamp01(ry) * 65535), false);
    } else {
      dv.setUint16(3, 0xFFFF, false);
      dv.setUint16(5, 0xFFFF, false);
    }
    sendBinary(buf);
  }

  function sendMouseScroll(dx, dy) {
    const buf = new ArrayBuffer(6);
    const dv = new DataView(buf);
    dv.setUint8(0, BIN_MOUSE_SCROLL);
    dv.setInt16(1, clampInt16(dx), false);
    dv.setInt16(3, clampInt16(dy), false);
    dv.setUint8(5, 0);
    sendBinary(buf);
  }

  function clamp01(v) { return Math.max(0, Math.min(1, v)); }
  function clampInt16(v) { return Math.max(-32768, Math.min(32767, Math.round(v))); }

  // Compute exact rendered video rectangle inside stage (handles letterboxing)
  function getVideoRect() {
    const vw = video.videoWidth || hostInfo.width || 1920;
    const vh = video.videoHeight || hostInfo.height || 1080;
    const rect = video.getBoundingClientRect();
    const videoRatio = vw / vh;
    const boxRatio = rect.width / rect.height;
    let renderW, renderH, offsetX, offsetY;
    if (boxRatio > videoRatio) {
      renderH = rect.height;
      renderW = renderH * videoRatio;
      offsetX = rect.left + (rect.width - renderW) / 2;
      offsetY = rect.top;
    } else {
      renderW = rect.width;
      renderH = renderW / videoRatio;
      offsetX = rect.left;
      offsetY = rect.top + (rect.height - renderH) / 2;
    }
    return { left: offsetX, top: offsetY, width: renderW, height: renderH };
  }

  function clientPointToNormalized(clientX, clientY) {
    const r = getVideoRect();
    return {
      rx: clamp01((clientX - r.left) / r.width),
      ry: clamp01((clientY - r.top) / r.height),
    };
  }

  function showRipple(x, y) {
    touchRipple.style.left = `${x}px`;
    touchRipple.style.top = `${y}px`;
    touchRipple.classList.remove("hidden");
    touchRipple.style.animation = "none";
    void touchRipple.offsetWidth;
    touchRipple.style.animation = "";
  }

  // ==================== TOUCH / ZOOM / PAN / TRACKPAD ENGINE ====================
  let touchState = {
    activeCount: 0,
    startX: 0, startY: 0,
    lastX: 0, lastY: 0,
    startTime: 0,
    moved: false,
    twoFingerMoved: false,
    pinchStartDist: 0,
    pinchStartZoom: 1,
    lastTapTime: 0,
    lastTapX: 0,
    lastTapY: 0,
    dragging: false,
    longPressTimer: null,
    longPressed: false,
  };

  function clearTouchLongPress() {
    if (touchState.longPressTimer) {
      clearTimeout(touchState.longPressTimer);
      touchState.longPressTimer = null;
    }
  }

  stage.addEventListener("touchstart", (e) => {
    if (aiState === "ACTING" || aiState === "THINKING") return;
    e.preventDefault();
    const t = e.touches;
    touchState.activeCount = t.length;
    clearTouchLongPress();

    if (t.length === 1) {
      const p = t[0];
      touchState.startX = touchState.lastX = p.clientX;
      touchState.startY = touchState.lastY = p.clientY;
      touchState.startTime = performance.now();
      touchState.moved = false;
      touchState.twoFingerMoved = false;
      touchState.longPressed = false;

      // Double-tap & hold -> drag selection/window
      const isDoubleHold =
        touchState.startTime - touchState.lastTapTime < 280 &&
        Math.hypot(p.clientX - touchState.lastTapX, p.clientY - touchState.lastTapY) < 36;
      if (isDoubleHold) {
        touchState.dragging = true;
        if (inputMode === "touch") {
          const { rx, ry } = clientPointToNormalized(p.clientX, p.clientY);
          sendMouseButton(BTN_LEFT, ACT_DOWN, rx, ry);
        } else {
          sendMouseButton(BTN_LEFT, ACT_DOWN);
        }
        haptic(15);
      } else if (inputMode === "touch") {
        // Long-press in Direct Touch mode -> Right-click at exact finger position!
        const cx = p.clientX, cy = p.clientY;
        touchState.longPressTimer = setTimeout(() => {
          if (!touchState.moved && touchState.activeCount === 1) {
            touchState.longPressed = true;
            const { rx, ry } = clientPointToNormalized(cx, cy);
            sendMouseButton(BTN_RIGHT, ACT_CLICK, rx, ry);
            showRipple(cx, cy);
            haptic(28);
          }
        }, 420);
      }
    } else if (t.length === 2) {
      const p0 = t[0], p1 = t[1];
      touchState.lastX = (p0.clientX + p1.clientX) / 2;
      touchState.lastY = (p0.clientY + p1.clientY) / 2;
      touchState.pinchStartDist = Math.hypot(p1.clientX - p0.clientX, p1.clientY - p0.clientY);
      touchState.pinchStartZoom = viewZoom;
      touchState.startTime = performance.now();
      touchState.twoFingerMoved = false;
    }
  }, { passive: false });

  stage.addEventListener("touchmove", (e) => {
    if (aiState === "ACTING" || aiState === "THINKING") return;
    e.preventDefault();
    const t = e.touches;

    if (t.length === 1) {
      const p = t[0];
      const dx = p.clientX - touchState.lastX;
      const dy = p.clientY - touchState.lastY;
      if (Math.hypot(p.clientX - touchState.startX, p.clientY - touchState.startY) > 7) {
        touchState.moved = true;
        clearTouchLongPress();
      }
      if (touchState.longPressed) return;

      if (touchState.dragging) {
        if (inputMode === "trackpad") {
          sendMouseMoveRel(dx * settings.sensitivity, dy * settings.sensitivity);
        } else {
          const { rx, ry } = clientPointToNormalized(p.clientX, p.clientY);
          sendMouseMoveAbs(rx, ry);
        }
      } else if (inputMode === "trackpad") {
        const speed = Math.hypot(dx, dy);
        const accel = settings.sensitivity * (speed > 14 ? 1.55 : speed > 6 ? 1.2 : 0.9);
        sendMouseMoveRel(dx * accel, dy * accel);
      } else {
        // Direct Touch mode:
        // - If zoomed in (>1.05x), 1-finger drag pans the viewport smoothly so you can reach any area!
        // - If at 1.0x, 1-finger drag scrolls the desktop smoothly under your finger!
        if (viewZoom > 1.05) {
          panX += dx;
          panY += dy;
          applyViewportTransform();
        } else if (touchState.moved) {
          sendMouseScroll(Math.round(-dx * 0.25), Math.round(-dy * 0.35));
        }
      }
      touchState.lastX = p.clientX;
      touchState.lastY = p.clientY;
    } else if (t.length === 2) {
      const p0 = t[0], p1 = t[1];
      const mx = (p0.clientX + p1.clientX) / 2;
      const my = (p0.clientY + p1.clientY) / 2;
      const dist = Math.hypot(p1.clientX - p0.clientX, p1.clientY - p0.clientY);
      const dx = mx - touchState.lastX;
      const dy = my - touchState.lastY;

      const distDelta = Math.abs(dist - touchState.pinchStartDist);
      if (distDelta > 12 || Math.hypot(dx, dy) > 5) {
        touchState.twoFingerMoved = true;
      }

      if (touchState.pinchStartDist > 20 && distDelta > 10) {
        // Smooth Pinch-to-Zoom (1.0x .. 4.0x) + simultaneous 2-finger pan
        const newZoom = Math.max(1.0, Math.min(4.0, touchState.pinchStartZoom * (dist / touchState.pinchStartDist)));
        const ratio = newZoom / viewZoom;
        const cx = mx - window.innerWidth / 2;
        const cy = my - window.innerHeight / 2;
        panX = cx - (cx - panX) * ratio + dx;
        panY = cy - (cy - panY) * ratio + dy;
        viewZoom = newZoom;
        applyViewportTransform();
      } else if (touchState.twoFingerMoved) {
        if (viewZoom > 1.05) {
          panX += dx;
          panY += dy;
          applyViewportTransform();
        } else {
          sendMouseScroll(Math.round(dx * 0.25), Math.round(-dy * 0.35));
        }
      }
      touchState.lastX = mx;
      touchState.lastY = my;
    }
  }, { passive: false });

  stage.addEventListener("touchend", (e) => {
    if (aiState === "ACTING" || aiState === "THINKING") return;
    e.preventDefault();
    clearTouchLongPress();
    const dt = performance.now() - touchState.startTime;

    if (touchState.dragging && e.touches.length === 0) {
      touchState.dragging = false;
      sendMouseButton(BTN_LEFT, ACT_UP);
      return;
    }

    if (touchState.longPressed) {
      if (e.touches.length === 0) {
        touchState.longPressed = false;
        touchState.activeCount = 0;
      }
      return;
    }

    // Two-finger quick tap -> Right click
    if (touchState.activeCount === 2 && !touchState.twoFingerMoved && dt < 280 && e.touches.length === 0) {
      sendMouseButton(BTN_RIGHT, ACT_CLICK);
      haptic(18);
      touchState.activeCount = 0;
      return;
    }

    // Single-finger tap -> Click at EXACT finger location (Direct Touch) or cursor (Trackpad)
    if (touchState.activeCount === 1 && !touchState.moved && dt < 280 && e.touches.length === 0) {
      const changed = e.changedTouches[0];
      showRipple(changed.clientX, changed.clientY);
      haptic(10);
      const now = performance.now();
      const isDoubleTap =
        now - touchState.lastTapTime < 290 &&
        Math.hypot(changed.clientX - touchState.lastTapX, changed.clientY - touchState.lastTapY) < 36;

      if (inputMode === "touch") {
        const { rx, ry } = clientPointToNormalized(changed.clientX, changed.clientY);
        sendMouseButton(BTN_LEFT, isDoubleTap ? ACT_DBLCLICK : ACT_CLICK, rx, ry);
      } else {
        sendMouseButton(BTN_LEFT, isDoubleTap ? ACT_DBLCLICK : ACT_CLICK);
      }
      touchState.lastTapTime = isDoubleTap ? 0 : now;
      touchState.lastTapX = changed.clientX;
      touchState.lastTapY = changed.clientY;
    }
    if (e.touches.length === 0) touchState.activeCount = 0;
  }, { passive: false });

  // Desktop browser mouse support (handy for testing on laptop too)
  let mouseDown = false;
  stage.addEventListener("mousedown", (e) => {
    if (e.target !== stage && e.target !== video) return;
    mouseDown = true;
    const { rx, ry } = clientPointToNormalized(e.clientX, e.clientY);
    const btn = e.button === 2 ? BTN_RIGHT : e.button === 1 ? BTN_MIDDLE : BTN_LEFT;
    sendMouseButton(btn, ACT_DOWN, rx, ry);
  });
  stage.addEventListener("mousemove", (e) => {
    if (e.buttons === 0 && !mouseDown) return;
    const { rx, ry } = clientPointToNormalized(e.clientX, e.clientY);
    sendMouseMoveAbs(rx, ry);
  });
  window.addEventListener("mouseup", (e) => {
    if (!mouseDown) return;
    mouseDown = false;
    const { rx, ry } = clientPointToNormalized(e.clientX, e.clientY);
    const btn = e.button === 2 ? BTN_RIGHT : e.button === 1 ? BTN_MIDDLE : BTN_LEFT;
    sendMouseButton(btn, ACT_UP, rx, ry);
  });
  stage.addEventListener("contextmenu", (e) => e.preventDefault());
  stage.addEventListener("wheel", (e) => {
    e.preventDefault();
    sendMouseScroll(Math.round(e.deltaX / 30), Math.round(e.deltaY / 30));
  }, { passive: false });

  // ==================== CONTROL CHANNEL HANDLER ====================
  function handleControlMessage(raw) {
    let msg;
    try { msg = JSON.parse(raw); } catch { return; }

    switch (msg.type) {
      case "pong": {
        const clientTs = msg.payload?.client_ts;
        if (clientTs) {
          const rtt = Math.max(1, Date.now() - clientTs);
          hudMiniStats.textContent = `${rtt}ms · ${hostInfo.fps}fps`;
        }
        break;
      }
      case "host.info": {
        hostInfo = { ...hostInfo, ...msg.payload };
        btnModPrimary.textContent = hostInfo.primary_mod === "Cmd" ? "⌘ Cmd" : "Ctrl";
        aiHostLabel.textContent = `${hostInfo.hostname} · ${hostInfo.os === "macos" ? "macOS" : "Win11"}`;
        if (currentHost) {
          currentHost.name = hostInfo.hostname;
          upsertHost({ name: hostInfo.hostname, url: currentHost.url, token: currentHost.token, os: hostInfo.os });
        }
        break;
      }
      case "ai.status": {
        const p = msg.payload || {};
        setAIState(p.state || "IDLE", p.message || "", p.step, p.max_step);
        break;
      }
      case "ai.step_preview": {
        const p = msg.payload || {};
        appendFeed(`⚡ Passo ${p.step}: ${p.description}`);
        if (p.has_point) {
          showClickHalo(p.x_norm, p.y_norm);
        }
        break;
      }
      case "ai.step_done": {
        const p = msg.payload || {};
        if (!p.success && p.output) {
          appendFeed(`⚠️ ${p.output}`);
        }
        break;
      }
      case "ai.confirm_required": {
        const p = msg.payload || {};
        pendingConfirm = p;
        confirmReason.textContent = p.reason || "O agente solicitou permissão para executar este comando.";
        confirmCommand.textContent = p.command || p.action;
        confirmModal.classList.remove("hidden");
        haptic(40);
        break;
      }
    }
  }

  function showClickHalo(xNorm, yNorm) {
    const r = getVideoRect();
    const px = r.left + xNorm * r.width;
    const py = r.top + yNorm * r.height;
    clickHalo.style.left = `${px}px`;
    clickHalo.style.top = `${py}px`;
    clickHalo.classList.remove("hidden");
    clearTimeout(haloTimer);
    haloTimer = setTimeout(() => clickHalo.classList.add("hidden"), 1100);
  }

  function appendFeed(line) {
    aiFeed.classList.remove("hidden");
    const div = document.createElement("div");
    div.textContent = line;
    aiFeed.appendChild(div);
    aiFeed.scrollTop = aiFeed.scrollHeight;
  }

  function setAIState(state, message, step, maxStep) {
    aiState = state;
    aiOrb.classList.remove("state-idle", "state-listening", "state-thinking", "state-acting");
    if (botHero) {
      botHero.classList.remove("state-idle", "state-listening", "state-thinking", "state-acting");
    }

    if (state === "THINKING" || state === "ACTING" || state === "CONFIRM") {
      const cls = state === "THINKING" ? "state-thinking" : "state-acting";
      aiOrb.classList.add(cls);
      if (botHero) botHero.classList.add(cls);
      if (botHeroTitle) botHeroTitle.textContent = state === "THINKING" ? "Analisando tela..." : "Agindo no PC";
      if (botHeroSub) botHeroSub.textContent = message || "Executando sua instrução...";
      aiOverlay.classList.add("hidden"); // Free 92% of screen so user watches live!
      dynamicIsland.classList.remove("hidden");
      islandTitle.textContent = step ? `Agente · Passo ${step}/${maxStep || 18}` : "Agente Ativo";
      islandSub.textContent = message || "Operando computador...";
    } else {
      aiOrb.classList.add("state-idle");
      if (botHero) botHero.classList.add("state-idle");
      if (botHeroTitle) botHeroTitle.textContent = "AnyDask AI";
      if (botHeroSub) botHeroSub.textContent = "Estou vendo sua tela. O que fazemos?";
      dynamicIsland.classList.add("hidden");
      clickHalo.classList.add("hidden");
      if (state === "COMPLETED") {
        appendFeed(`✅ ${message}`);
        showToast(message || "Tarefa concluída!", 3800);
        haptic(25);
      } else if (state === "ABORTED") {
        appendFeed("🛑 Interrompido — você está no controle.");
        showToast("Controle manual retomado!", 2400);
        haptic(30);
      } else if (state === "ERROR") {
        appendFeed(`❌ ${message}`);
        showToast(`Erro IA: ${message}`, 4500);
      }
    }
  }

  // ==================== KILL SWITCH (1-TAP INSTANT STOP) ====================
  btnKillSwitch.addEventListener("click", () => {
    haptic(45);
    sendControl("ai.abort", {});
    setAIState("ABORTED", "Controle retomado");
  });

  document.getElementById("btnConfirmApprove").addEventListener("click", () => {
    if (pendingConfirm) {
      sendControl("ai.confirm_response", { task_id: pendingConfirm.task_id, confirm_id: pendingConfirm.confirm_id, approved: true });
      pendingConfirm = null;
    }
    confirmModal.classList.add("hidden");
  });
  document.getElementById("btnConfirmDeny").addEventListener("click", () => {
    if (pendingConfirm) {
      sendControl("ai.confirm_response", { task_id: pendingConfirm.task_id, confirm_id: pendingConfirm.confirm_id, approved: false });
      pendingConfirm = null;
    }
    confirmModal.classList.add("hidden");
  });

  // ==================== FLOATING AI BOT (SPRING PHYSICS + EYE TRACKING + AWAKE ANIMATION) ====================
  const botPhys = {
    x: window.innerWidth - 70,
    y: window.innerHeight - 150,
    tx: window.innerWidth - 70,
    ty: window.innerHeight - 150,
    vx: 0,
    vy: 0,
    dragging: false,
  };

  function startBotPhysicsLoop() {
    const tick = () => {
      if (!botPhys.dragging) {
        // Organic spring physics (stiffness + damping)
        const ax = (botPhys.tx - botPhys.x) * 0.18;
        const ay = (botPhys.ty - botPhys.y) * 0.18;
        botPhys.vx = (botPhys.vx + ax) * 0.74;
        botPhys.vy = (botPhys.vy + ay) * 0.74;
        botPhys.x += botPhys.vx;
        botPhys.y += botPhys.vy;
      }

      aiOrb.style.left = `${botPhys.x}px`;
      aiOrb.style.top = `${botPhys.y}px`;
      aiOrb.style.right = "auto";
      aiOrb.style.bottom = "auto";

      // Tilt body & track eyes subtly in the direction of velocity!
      const speed = Math.hypot(botPhys.vx, botPhys.vy);
      if (botPhys.dragging || speed > 0.3) {
        const tilt = Math.max(-14, Math.min(14, botPhys.vx * 0.85));
        aiOrb.style.transform = `rotate(${tilt.toFixed(1)}deg)${botPhys.dragging ? " scale(1.08)" : ""}`;
        if (botEyesMini) {
          const ex = Math.max(-4.5, Math.min(4.5, botPhys.vx * 0.38));
          const ey = Math.max(-3.5, Math.min(3.5, botPhys.vy * 0.32));
          botEyesMini.style.transform = `translate(${ex.toFixed(1)}px, ${ey.toFixed(1)}px)`;
        }
      } else {
        aiOrb.style.transform = "";
        if (botEyesMini) botEyesMini.style.transform = "";
      }

      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }
  startBotPhysicsLoop();

  // Ambient 60fps Siri/Plasma Wave Canvas inside the Bot Assistant Hero
  let wavePhase = 0;
  function startWaveLoop() {
    if (!botWaveCanvas) return;
    const ctx = botWaveCanvas.getContext("2d");
    const draw = () => {
      if (!aiOverlay.classList.contains("hidden")) {
        const w = botWaveCanvas.width;
        const h = botWaveCanvas.height;
        ctx.clearRect(0, 0, w, h);
        const energy = isRecording ? 1.85 : aiState === "THINKING" || aiState === "ACTING" ? 1.35 : 0.55;
        wavePhase += 0.045 * energy;

        const colors = [
          "rgba(100, 210, 255, 0.42)",
          "rgba(94, 92, 230, 0.36)",
          "rgba(191, 90, 242, 0.34)",
        ];
        colors.forEach((col, idx) => {
          ctx.beginPath();
          ctx.strokeStyle = col;
          ctx.lineWidth = 2.2;
          for (let x = 0; x <= w; x += 6) {
            const norm = x / w;
            const env = Math.sin(norm * Math.PI);
            const y = h / 2 + Math.sin(norm * (6 + idx * 2.5) + wavePhase + idx * 1.4) * (12 * energy) * env;
            if (x === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
          }
          ctx.stroke();
        });
      }
      requestAnimationFrame(draw);
    };
    requestAnimationFrame(draw);
  }
  startWaveLoop();

  function snapOrbToEdge() {
    const size = 56;
    const margin = 12;
    botPhys.tx = botPhys.x + size / 2 < window.innerWidth / 2
      ? margin
      : window.innerWidth - size - margin;
    botPhys.ty = Math.max(64, Math.min(window.innerHeight - 126, botPhys.y));
  }
  window.addEventListener("resize", () => snapOrbToEdge());

  let orbDrag = { active: false, moved: false, startX: 0, startY: 0, lastX: 0, lastY: 0, origL: 0, origT: 0, holdTimer: null, longPressed: false };

  aiOrb.addEventListener("pointerdown", (e) => {
    e.stopPropagation();
    aiOrb.setPointerCapture(e.pointerId);
    orbDrag = {
      active: true, moved: false, longPressed: false,
      startX: e.clientX, startY: e.clientY,
      lastX: e.clientX, lastY: e.clientY,
      origL: botPhys.x, origT: botPhys.y,
      holdTimer: setTimeout(() => {
        if (!orbDrag.moved) {
          orbDrag.longPressed = true;
          haptic(25);
          startVoiceCapture(true);
        }
      }, 360),
    };
    botPhys.dragging = true;
    aiOrb.classList.add("dragging");
  });

  aiOrb.addEventListener("pointermove", (e) => {
    if (!orbDrag.active || orbDrag.longPressed) return;
    const dx = e.clientX - orbDrag.startX;
    const dy = e.clientY - orbDrag.startY;
    if (Math.hypot(dx, dy) > 6) {
      orbDrag.moved = true;
      clearTimeout(orbDrag.holdTimer);
    }
    if (orbDrag.moved) {
      botPhys.vx = e.clientX - orbDrag.lastX;
      botPhys.vy = e.clientY - orbDrag.lastY;
      botPhys.x = orbDrag.origL + dx;
      botPhys.y = orbDrag.origT + dy;
      orbDrag.lastX = e.clientX;
      orbDrag.lastY = e.clientY;
    }
  });

  aiOrb.addEventListener("pointerup", () => {
    if (!orbDrag.active) return;
    clearTimeout(orbDrag.holdTimer);
    orbDrag.active = false;
    botPhys.dragging = false;
    aiOrb.classList.remove("dragging");

    if (orbDrag.longPressed) {
      stopVoiceCapture(true);
      return;
    }
    if (orbDrag.moved) {
      // Throw inertia + magnetic edge snap
      botPhys.x += botPhys.vx * 1.4;
      botPhys.y += botPhys.vy * 1.4;
      snapOrbToEdge();
      return;
    }

    // Single tap -> Trigger Bot Awake Animation & toggle Assistant Overlay
    haptic(14);
    aiOrb.classList.remove("awake-pop");
    void aiOrb.offsetWidth;
    aiOrb.classList.add("awake-pop");
    aiOverlay.classList.toggle("hidden");
    if (!aiOverlay.classList.contains("hidden")) {
      aiPromptInput.focus();
    }
  });

  // ==================== VOICE CAPTURE (NATIVE AUDIO + SPEECH RECOGNITION) ====================
  let mediaRecorder = null;
  let audioChunks = [];
  let speechRec = null;
  let recognizedText = "";
  let isRecording = false;

  async function startVoiceCapture(autoSendOnStop = false) {
    if (isRecording) return;
    isRecording = true;
    recognizedText = "";
    audioChunks = [];
    btnVoiceToggle.classList.add("recording");
    aiOrb.classList.remove("state-idle");
    aiOrb.classList.add("state-listening");
    if (botHero) {
      botHero.classList.remove("state-idle");
      botHero.classList.add("state-listening");
    }
    if (botHeroTitle) botHeroTitle.textContent = "Ouvindo você...";
    if (botHeroSub) botHeroSub.textContent = "Fale o que deseja que eu faça na tela";
    showToast("🎙️ Ouvindo... fale sua instrução", 2000);

    // 1. Web Speech API (instant PT-BR transcription for Claude / OpenAI / preview)
    const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (SR) {
      try {
        speechRec = new SR();
        speechRec.lang = "pt-BR";
        speechRec.interimResults = true;
        speechRec.continuous = true;
        speechRec.onresult = (ev) => {
          let txt = "";
          for (let i = 0; i < ev.results.length; i++) txt += ev.results[i][0].transcript;
          recognizedText = txt.trim();
          aiPromptInput.value = recognizedText;
        };
        speechRec.start();
      } catch {}
    }

    // 2. Raw Audio Recorder (for Gemini native multimodal audio!)
    if (navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        mediaRecorder = new MediaRecorder(stream);
        mediaRecorder.ondataavailable = (ev) => { if (ev.data.size > 0) audioChunks.push(ev.data); };
        mediaRecorder.onstop = async () => {
          stream.getTracks().forEach((tr) => tr.stop());
          if (autoSendOnStop) {
            const audioB64 = await blobsToBase64(audioChunks);
            dispatchAITask(recognizedText, settings.provider === "gemini" ? audioB64 : "", mediaRecorder.mimeType || "audio/webm");
          }
        };
        mediaRecorder.start();
      } catch {
        if (!speechRec) {
          showToast("Microfone indisponível (use HTTPS/localhost ou digite o comando)", 3200);
          stopVoiceCapture(false);
        }
      }
    }
  }

  function stopVoiceCapture(autoSend = false) {
    if (!isRecording) return;
    isRecording = false;
    btnVoiceToggle.classList.remove("recording");
    aiOrb.classList.remove("state-listening");
    aiOrb.classList.add("state-idle");
    if (botHero) {
      botHero.classList.remove("state-listening");
      botHero.classList.add("state-idle");
    }
    if (botHeroTitle) botHeroTitle.textContent = "AnyDask AI";
    if (botHeroSub) botHeroSub.textContent = "Estou vendo sua tela. O que fazemos?";

    if (speechRec) { try { speechRec.stop(); } catch {} speechRec = null; }
    if (mediaRecorder && mediaRecorder.state !== "inactive") {
      mediaRecorder.stop();
    } else if (autoSend && recognizedText) {
      dispatchAITask(recognizedText, "", "");
    }
  }

  function blobsToBase64(chunks) {
    return new Promise((resolve) => {
      if (!chunks || chunks.length === 0) return resolve("");
      const blob = new Blob(chunks, { type: chunks[0].type || "audio/webm" });
      const reader = new FileReader();
      reader.onloadend = () => {
        const res = String(reader.result || "");
        const idx = res.indexOf(",");
        resolve(idx >= 0 ? res.slice(idx + 1) : res);
      };
      reader.readAsDataURL(blob);
    });
  }

  btnVoiceToggle.addEventListener("click", () => {
    if (isRecording) stopVoiceCapture(true);
    else startVoiceCapture(false);
  });

  // ==================== AI TASK DISPATCH ====================
  function dispatchAITask(prompt, audioBase64 = "", audioMimeType = "") {
    prompt = (prompt || "").trim();
    if (!prompt && !audioBase64) return;
    if (!settings.apiKey) {
      showToast("Configure sua API Key primeiro nas Configurações ⚙", 3200);
      openSettingsModal();
      return;
    }

    aiFeed.innerHTML = "";
    appendFeed(`💬 "${prompt || "Instrução por voz"}"`);
    const ok = sendControl("ai.start_task", {
      prompt,
      audio_base64: audioBase64,
      audio_mime_type: audioMimeType,
      provider: settings.provider,
      model: settings.model,
      api_key: settings.apiKey,
      base_url: settings.baseUrl,
      max_steps: 18,
      require_confirm: settings.requireConfirm,
    });
    if (!ok) {
      showToast("Não conectado ao host!", 2500);
      return;
    }
    aiPromptInput.value = "";
    setAIState("THINKING", `Chamando ${settings.provider}...`);
  }

  document.getElementById("btnSendAiPrompt").addEventListener("click", () => {
    dispatchAITask(aiPromptInput.value);
  });
  aiPromptInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      dispatchAITask(aiPromptInput.value);
    }
  });
  document.getElementById("btnCloseAiOverlay").addEventListener("click", () => {
    aiOverlay.classList.add("hidden");
  });
  document.querySelectorAll(".suggestion-chip").forEach((chip) => {
    chip.addEventListener("click", () => {
      dispatchAITask(chip.dataset.prompt);
    });
  });

  // ==================== DEV SHORTCUT BAR & SOFT KEYBOARD ====================
  document.getElementById("btnToggleDevBar").addEventListener("click", () => {
    devBar.classList.toggle("collapsed");
  });

  function updateInputModeUI() {
    btnInputMode.textContent = inputMode === "trackpad" ? "Track" : "Toque";
  }
  updateInputModeUI();
  btnInputMode.addEventListener("click", () => {
    inputMode = inputMode === "trackpad" ? "touch" : "trackpad";
    localStorage.setItem("anydask_input_mode_v2", inputMode);
    updateInputModeUI();
    showToast(inputMode === "trackpad" ? "Modo Trackpad (cursor relativo)" : "Modo Toque Direto (clica onde você toca)");
  });

  if (btnZoomToggle) {
    btnZoomToggle.addEventListener("click", () => {
      haptic(10);
      if (viewZoom > 1.05) {
        viewZoom = 1.0;
        panX = 0;
        panY = 0;
        showToast("Zoom resetado (1x)");
      } else {
        viewZoom = 2.2;
        panX = 0;
        panY = 0;
        showToast("Zoom 2.2x — arraste com 1 dedo para mover a tela");
      }
      applyViewportTransform();
    });
  }

  if (btnRotate) {
    btnRotate.addEventListener("click", async () => {
      haptic(12);
      if (window.AnyDaskNative && window.AnyDaskNative.toggleOrientation) {
        window.AnyDaskNative.toggleOrientation();
        return;
      }
      try {
        if (!document.fullscreenElement && document.documentElement.requestFullscreen) {
          await document.documentElement.requestFullscreen();
        }
        if (screen.orientation && screen.orientation.lock) {
          const next = screen.orientation.type.startsWith("landscape") ? "portrait" : "landscape";
          await screen.orientation.lock(next);
        }
      } catch {
        showToast("Gire o celular ou use o App AnyDask para travar paisagem");
      }
    });
  }

  document.querySelectorAll(".dev-key").forEach((btn) => {
    btn.addEventListener("click", () => {
      haptic(8);
      if (btn.dataset.mod) {
        const m = btn.dataset.mod;
        if (stickyMods.has(m)) {
          stickyMods.delete(m);
          btn.classList.remove("active");
        } else {
          stickyMods.add(m);
          btn.classList.add("active");
        }
        return;
      }
      if (btn.dataset.combo) {
        const parts = btn.dataset.combo.split("+");
        const key = parts.pop();
        sendControl("key.event", { key, action: "press", modifiers: parts });
        return;
      }
      if (btn.dataset.text) {
        sendControl("key.text", { text: btn.dataset.text });
        return;
      }
      if (btn.dataset.key) {
        sendControl("key.event", { key: btn.dataset.key, action: "press", modifiers: Array.from(stickyMods) });
        clearStickyMods();
      }
    });
  });

  function clearStickyMods() {
    stickyMods.clear();
    document.querySelectorAll(".mod-key.active").forEach((el) => el.classList.remove("active"));
  }

  // Android soft keyboard bridge
  document.getElementById("btnKeyboard").addEventListener("click", () => {
    hiddenKeyboard.value = "";
    hiddenKeyboard.focus();
  });

  hiddenKeyboard.addEventListener("input", () => {
    const val = hiddenKeyboard.value;
    if (val) {
      if (stickyMods.size > 0 && val.length === 1) {
        sendControl("key.event", { key: val, action: "press", modifiers: Array.from(stickyMods) });
        clearStickyMods();
      } else {
        sendControl("key.text", { text: val });
      }
      hiddenKeyboard.value = "";
    }
  });

  hiddenKeyboard.addEventListener("keydown", (e) => {
    if (["Backspace", "Enter", "Tab", "Escape", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(e.key)) {
      e.preventDefault();
      sendControl("key.event", { key: e.key, action: "press", modifiers: Array.from(stickyMods) });
      clearStickyMods();
    }
  });

  // Physical keyboard support (when opened on desktop/tablet with keyboard)
  window.addEventListener("keydown", (e) => {
    if (document.activeElement && ["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement.tagName) && document.activeElement !== hiddenKeyboard) {
      return;
    }
    if (document.activeElement === hiddenKeyboard) return;
    e.preventDefault();
    const mods = [];
    if (e.ctrlKey) mods.push("CTRL");
    if (e.metaKey) mods.push("CMD");
    if (e.altKey) mods.push("ALT");
    if (e.shiftKey) mods.push("SHIFT");
    sendControl("key.event", { key: e.key, code: e.code, action: "press", modifiers: mods });
  });

  // ==================== HOST SWITCHER & SETTINGS MODALS ====================
  function renderHostsList() {
    const list = document.getElementById("hostsList");
    list.innerHTML = "";
    if (savedHosts.length === 0) {
      list.innerHTML = `<p class="muted">Nenhuma máquina salva ainda.</p>`;
      return;
    }
    savedHosts.forEach((h, idx) => {
      const row = document.createElement("div");
      row.className = "host-item" + (currentHost && currentHost.url === h.url ? " active" : "");
      row.innerHTML = `
        <div class="host-meta">
          <div class="host-name">${escapeHtml(h.name || "Host")}</div>
          <div class="host-url">${escapeHtml(h.url)}</div>
        </div>
        <div style="display:flex;gap:6px;">
          <button class="btn-primary" style="padding:7px 12px;font-size:12px;">Conectar</button>
          <button class="icon-btn-sm" title="Remover">✕</button>
        </div>
      `;
      row.querySelector(".btn-primary").addEventListener("click", () => {
        document.getElementById("hostsModal").classList.add("hidden");
        connectToHost(h.url, h.token, h.name);
      });
      row.querySelector(".icon-btn-sm").addEventListener("click", () => {
        savedHosts.splice(idx, 1);
        saveHosts();
        renderHostsList();
      });
      list.appendChild(row);
    });
  }

  document.getElementById("btnHosts").addEventListener("click", () => {
    renderHostsList();
    document.getElementById("hostsModal").classList.remove("hidden");
  });

  document.getElementById("btnAddHost").addEventListener("click", () => {
    const name = document.getElementById("newHostName").value.trim() || "Máquina";
    const url = document.getElementById("newHostUrl").value.trim();
    const token = document.getElementById("newHostToken").value.trim();
    if (!url || !token) return showToast("Informe URL e Token");
    upsertHost({ name, url: url.replace(/\/+$/, ""), token });
    document.getElementById("newHostName").value = "";
    document.getElementById("newHostUrl").value = "";
    document.getElementById("newHostToken").value = "";
    renderHostsList();
    showToast("Máquina salva!");
  });

  function updateQuickModelBadge() {
    const names = { gemini: "Gemini", anthropic: "Claude", openai: "OpenAI" };
    btnQuickModel.textContent = `${names[settings.provider] || settings.provider} · ${settings.model.split("/").pop().slice(0, 16)}`;
  }
  updateQuickModelBadge();

  function openSettingsModal() {
    document.getElementById("cfgProvider").value = settings.provider;
    document.getElementById("cfgModel").value = settings.model;
    document.getElementById("cfgApiKey").value = settings.apiKey;
    document.getElementById("cfgBaseUrl").value = settings.baseUrl;
    document.getElementById("cfgRequireConfirm").checked = settings.requireConfirm;
    document.getElementById("cfgSensitivity").value = settings.sensitivity;
    document.getElementById("sensVal").textContent = settings.sensitivity.toFixed(1);
    document.getElementById("settingsModal").classList.remove("hidden");
  }

  document.getElementById("btnSettings").addEventListener("click", openSettingsModal);
  btnQuickModel.addEventListener("click", openSettingsModal);

  document.getElementById("cfgProvider").addEventListener("change", (e) => {
    const prov = e.target.value;
    document.getElementById("cfgModel").value = DEFAULT_MODELS[prov] || "";
  });
  document.getElementById("cfgSensitivity").addEventListener("input", (e) => {
    document.getElementById("sensVal").textContent = Number(e.target.value).toFixed(1);
  });

  document.getElementById("btnSaveSettings").addEventListener("click", () => {
    settings.provider = document.getElementById("cfgProvider").value;
    settings.model = document.getElementById("cfgModel").value.trim() || DEFAULT_MODELS[settings.provider];
    settings.apiKey = document.getElementById("cfgApiKey").value.trim();
    settings.baseUrl = document.getElementById("cfgBaseUrl").value.trim();
    settings.requireConfirm = document.getElementById("cfgRequireConfirm").checked;
    settings.sensitivity = Number(document.getElementById("cfgSensitivity").value || 1.8);
    saveSettings();
    document.getElementById("settingsModal").classList.add("hidden");
    showToast("Configurações salvas!");
  });

  document.querySelectorAll(".close-modal").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.getElementById(btn.dataset.close).classList.add("hidden");
    });
  });

  document.getElementById("btnManualConnect").addEventListener("click", () => {
    const url = connectUrlInput.value.trim();
    const token = connectTokenInput.value.trim();
    if (!url || !token) return showToast("Preencha URL e Token");
    connectToHost(url, token, url);
  });

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  }

  // ==================== BOOTSTRAP ====================
  function bootstrap() {
    const params = new URLSearchParams(window.location.search);
    const urlToken = params.get("token");
    if (urlToken) {
      // Clean token out of address bar for neatness
      const cleanUrl = window.location.origin;
      window.history.replaceState({}, "", "/");
      connectToHost(cleanUrl, urlToken, window.location.hostname);
      return;
    }

    try {
      const last = JSON.parse(localStorage.getItem("anydask_last_host") || "null");
      if (last && last.url && last.token) {
        connectToHost(last.url, last.token, last.name);
        return;
      }
    } catch {}

    connectStatusText.textContent = "Informe o endereço e o token do seu Mac Mini ou Windows 11";
    connectUrlInput.value = window.location.origin;
    connectForm.classList.remove("hidden");
  }

  bootstrap();
})();
