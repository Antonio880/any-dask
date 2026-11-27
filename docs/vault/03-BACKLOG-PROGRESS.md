---
title: AnyDask — Backlog & Progresso Vivo
status: fase-1-planejamento
updated: 2026-11-26
tags:
  - backlog
  - roadmap
  - progress
---

# 📋 Backlog & Progresso (Roadmap 4 Semanas)

Voltar para [[00-INDEX]] | Ver arquitetura em [[01-ARCHITECTURE]] | Ver design em [[04-UI-DESIGN-SYSTEM]]

---

## 🔄 Status Atual

### ✅ Done
- [x] **Fase 1:** Criação do Obsidian Memory Bank em `/docs/vault/` (5 documentos base)
- [x] **Fase 1:** Definição da arquitetura de rede (Tailscale + WebRTC P2P), pipeline de vídeo/input e loop de Computer Use
- [x] **Fase 1:** Definição dos ADRs (ADR-001 a ADR-007) e Design System Apple Glass
- [x] **Aprovação do usuário:** Stack Go (`pion/webrtc`) + Tailscale/LAN + Estratégia Dual-Track PWA Apple Glass -> APK Nativo
- [x] **Toolchain:** Go 1.27.4 + FFmpeg 9.1 instalados via winget
- [x] **Protocolo:** `protocol/input-binary-spec.md` e `protocol/control-messages.schema.json` + tipos Go testados
- [x] **Host Agent (`host/`):**
  - [x] WebRTC `pion/webrtc/v4` (H.264 VideoTrack + DataChannels `input` binário unordered e `control` JSON)
  - [x] Captura H.264 HW auto-detectada (`ddagrab`/`gdigrab` + `nvenc/qsv/amf/mf/libx264` no Win11; `avfoundation` + `h264_videotoolbox` no macOS)
  - [x] Snapshot direto de tela em memória (GDI BitBlt Win32 / `screencapture` macOS) com redimensionamento canônico `1280px`
  - [x] `InputArbiter` + `CoordinateMapper` HiDPI + injeção nativa (`SendInput` Windows / `CGEventPost` macOS) + Kill Switch `< 10ms`
  - [x] Orquestrador de Computer Use + Guardrails + Adapters BYOK (**Anthropic Claude**, **Google Gemini** com áudio nativo, **OpenAI/OpenRouter**)
- [x] **Cliente Mobile PWA Apple Glass (`host/internal/webui/static/`):**
  - [x] Glassmorphism real sobre `<video>`, Trackpad de precisão + Toque Direto + Gestos 2 dedos
  - [x] Orb Flutuante arrastável com snap magnético + Push-to-Talk + Dynamic Island Pill com botão **PARAR**
  - [x] Halo visual de clique do agente + Modal Glass de confirmação de segurança
  - [x] Barra de atalhos Dev retrátil + Host Switcher (Windows 11 <-> Mac Mini) + Tela de Configurações BYOK
- [x] **Build & Testes:** `go test ./...` passando + `bin/anydask-host.exe` validado ao vivo (`13ms · 60fps` no celular via `ddagrab -> h264_amf`)
- [x] **UX Mobile v2:** Toque Direto como modo padrão (clique exato no local tocado), Pinch-to-Zoom `1x–4x` + Pan suave, Long-Press = clique direito, botões rápidos `1x`/`2.2x` e `↻` Rotação
- [x] **IA Gemini Default:** Endpoint validado com `gemini-3-flash-preview` e chave default carregada via `/api/ai-defaults` (`~/.anydask/ai-config.json`)
- [x] **App Android Nativo (`android/`):** Projeto Kotlin imersivo compilado com sucesso em `bin/anydask-android.apk` (`v1.0.2`, Tailscale + LAN fallback)
- [x] **Acesso Remoto Externo (4G/5G):** Tailscale + `ICE-Lite` (`SetLite(true)`) + filtro de interfaces virtuais WSL/Hyper-V validado ao vivo pelo usuário
- [x] **Bot Assistente Virtual Fluido:** Avatar animado com olhinhos expressivos (piscar natural + eye-tracking no arraste), física de mola 60fps com inclinação (`tilt`), animação de despertar ao tocar e Hero animado com onda sonora em `<canvas>`
- [x] **Suporte macOS (Mac Mini):** Auto-detecção de índice de tela (`Capture screen 0`) + script `start-anydask-mac.sh` prontos
- [x] **GitHub:** Publicado em `https://github.com/Antonio880/any-dask` (branch `main`, zero segredos expostos)
- [x] **Fix Vídeo macOS (Anti-Freeze):** `-fflags nobuffer -flags low_delay -flush_packets 1`, CFR `fps=%s`, downscale Retina `min(1920,iw)`, `prio_speed` + `nv12` no `h264_videotoolbox`, RTP duration real pelo relógio monotônico, RTCP reader no `rtpSender` e `playoutDelayHint = 0` no cliente
- [x] **iOS 26 Liquid Glass Dual-Side (Celular + Tela do PC/Mac):**
  - [x] Overlay nativo click-through no **Windows 11** (`UpdateLayeredWindow` per-pixel alpha: borda líquida Apple Intelligence + Dynamic Island com waveform viva + Liquid Glass Ripples em cada clique)
  - [x] Overlay nativo click-through no **macOS** (`NSWindow` + `NSVisualEffectView` blur real da Apple + borda líquida + Liquid Glass Ripples)
  - [x] Upgrade **iOS 26 Liquid Glass** no App Mobile (moldura perimetral sincronizada, reflexo especular diagonal, touch ripples refrativos e botão `✨` na DevBar para ligar/desligar o vidro no PC/Mac)
- [x] **Multi-Monitor Nativo (macOS + Windows 11) + Modo Turbo Ultra-Latência:**
  - [x] Descoberta de todos os monitores (`CGGetActiveDisplayList` no macOS / `EnumDisplayMonitors` no Windows) + troca ao vivo sem desconectar (`🖥 1/2` na DevBar) + mapeamento exato de coordenadas de toque e Computer Use no monitor selecionado
  - [x] Redução agressiva de latência: leitura direta do pipe sem `bufio`, `-probesize 32 -analyzeduration 0`, `-pixel_format nv12` nativo no `avfoundation` do Mac, clique reduzido de `12ms` → `4ms`, clear parcial de memória no overlay Windows, e botão **`HD` / `⚡Turbo` (720p@60fps)** em 1 toque
- [x] **Portal Liquid Glass de Conexão & Fix WebView Android (`v1.0.3`):**
  - [x] Tela de conexão Apple Liquid Glass com Bot animado + anéis orbitais + stepper (`Sinal -> Túnel P2P -> 60 FPS`) e vídeo oculto até o 1º frame real decodificado
  - [x] `MainActivity.kt` sobrescrevendo `getDefaultVideoPoster()` (elimina o ícone cinza ▶ esticado do Android WebView) + `LOAD_NO_CACHE` + `clearCache(true)` (resolve cache preso de UI antiga)
- [x] **Computer Use Ultra-Rápido (~1.5s/passo) + Hold-to-Talk Nativo (`v1.0.4`):**
  - [x] Benchmark ao vivo detectou timeout no `gemini-3-flash-preview` (>25s) e validou **`gemini-3.1-flash-lite` em 1.4s–1.7s** -> novo default + cadeia de fallback automática + timeout 16s + screenshot 1024px (~50KB)
  - [x] **Console ao Vivo Liquid Glass (`#aiLiveLog`)** abaixo da Dynamic Island + cronômetro ao vivo (`0.0s`) exibindo cada etapa, ação, saída e erro em tempo real
  - [x] **Hold-to-Talk nativo no Bot Flutuante** (`MainActivity.kt` `MediaRecorder` AAC `audio/mp4` sem depender de HTTPS + `#voiceHud` ao vivo): segura o Bot -> grava -> solta -> envia na hora para o Gemini agir!

### 🚧 In Progress
- [ ] Validação ao vivo do Hold-to-Talk e do agente ultra-rápido (`~1.5s/passo`) no Android (`v1.0.4`) + Windows 11 + Mac

### 📌 Próximo Passo Imediato
1. Rodar `bin\anydask-host.exe` neste Windows 11 para o usuário testar já no Android!
2. Compilar no Mac Mini e cadastrar os dois hosts no **Host Switcher**.
3. Opcional (Fase 1.5): Empacotar cliente nativo Android (Kotlin + Jetpack Compose) e auto-start como serviço Windows/LaunchAgent macOS.

---

## 🗓️ Roadmap Detalhado (Meta: 1 Mês)

### Semana 1 — Core Remote MVP (Vídeo + Input Funcional)
> **Objetivo:** Abrir o app Android, conectar no Windows 11 ou Mac Mini via Tailscale, ver a tela fluída e controlar mouse/teclado.

- [ ] Scaffold do monorepo (`host/` em Go, `android/` em Kotlin Compose, `protocol/`)
- [ ] Contrato de protocolo: `input-binary-spec.md` e `control-messages.schema.json`
- [ ] Host Agent: servidor WSS de signaling + autenticação por token/QR pairing
- [ ] Host Agent: pipeline `pion/webrtc` (1 VideoTrack H.264 + 2 DataChannels `input` e `control`)
- [ ] Host Agent: captura de tela + H.264 HW no **Windows 11** (`DXGI Desktop Duplication` + MediaFoundation/NVENC)
- [ ] Host Agent: captura de tela + H.264 HW no **macOS** (`ScreenCaptureKit` + `VideoToolbox`)
- [ ] Host Agent: `InputArbiter` (modo `MANUAL`) + injeção nativa (`SendInput` Win / `CGEventPost` macOS)
- [ ] Android App: cliente `libwebrtc` + `TextureView` com decode H.264 por hardware
- [ ] Android App: gestos básicos de Trackpad (cursor relativo/absoluto, tap=clique, 2 dedos=clique direito/scroll) + teclado virtual

### Semana 2 — Baixa Latência (<50ms), Multi-Host & UI Apple Glass
> **Objetivo:** Experiência diária impecável, alternância rápida Mac <-> PC e visual premium.

- [ ] Otimização de latência: tuning de bitrate adaptativo, zero B-frames, PLI instantâneo e telemetria HUD opcional (RTT, FPS, Decode ms)
- [ ] Mapeamento inteligente de atalhos cross-OS (`PRIMARY` -> `Cmd` no macOS / `Ctrl` no Windows) + suporte multi-monitor
- [ ] Sincronização bidirecional de Clipboard (copiar no Android <-> colar no PC/Mac)
- [ ] Android UI: implementação dos tokens de [[04-UI-DESIGN-SYSTEM]] (Glassmorphism real com Haze/AGSL sobre vídeo)
- [ ] Android UI: **Host Switcher** em folha translúcida (troca rápida entre Mac Mini e Windows 11 com status online/latência)
- [ ] Android UI: **Barra de Atalhos Dev retrátil** (Tab, Esc, Ctrl+C, Cmd/Ctrl, Alt, Setas, Slash/Pipe)
- [ ] Android UI: **Orb Flutuante** arrastável com física de snap nas bordas e estados visuais (sem lógica de IA ainda)

### Semana 3 — Agente de IA (Computer Use Engine + Multi-Provider)
> **Objetivo:** Clicar no Orb, enviar um comando em texto e assistir o agente operar o Mac/Windows em tempo real.

- [ ] Host Agent: `CoordinateMapper` HiDPI/Retina (macOS 2x / Windows DPI scaling)
- [ ] Host Agent: `ContextCollector` (frame nítido do RingBuffer, janela ativa, SO, árvore de acessibilidade opcional)
- [ ] Host Agent: Executor de ações do Agente (mouse, clique, duplo clique, drag, digitação Unicode, hotkeys, scroll, shell)
- [ ] Host Agent: `Orchestrator` (loop Vision -> Plan -> Act -> Verify com limite de passos e `context.WithCancel`)
- [ ] Provider Adapter #1: **Anthropic Claude** (Computer Use nativo + Bash tool)
- [ ] Provider Adapter #2: **Google Gemini** (Vision + Grounding `0..1000` + Function Calling)
- [ ] Provider Adapter #3: **OpenAI / OpenRouter** (endpoint compatível customizável)
- [ ] Android UI: Tela de Configurações segura (**Android Keystore**) para gerenciar API Keys, provedor ativo e modelo
- [ ] Android UI: Overlay compacto do Orb (input de texto + feed minimalista do que o agente está fazendo)

### Semana 4 — Voz em Tempo Real, Kill Switch, Segurança & Hardening
> **Objetivo:** Polimento de produção pessoal, controle por voz fluido, segurança e inicialização automática.

- [ ] Android Audio: Push-to-Talk / gravação no Orb com onda pulsante reativa ao volume (envio nativo para Gemini ou transcrição via Whisper/STT)
- [ ] Kill Switch end-to-end: botão **PARAR / ASSUMIR CONTROLE** em destaque durante `AI_ACTING` (parada `< 10ms` no Host + fail-safe de desconexão)
- [ ] Overlay visual no stream: halo animado indicando o ponto `(x, y)` onde o agente vai clicar antes/durante a ação
- [ ] Guardrails: diálogo Glass de confirmação em 1 toque para comandos destrutivos (`sudo`, `rm -rf`, etc.)
- [ ] Empacotamento e Auto-Start: `LaunchAgent` no macOS (Mac Mini) + Serviço/Task Scheduler no Windows 11 + ícone discreto na tray/menubar
- [ ] Bateria de testes reais: Wi-Fi local, 4G/5G externo, troca rápida Mac Mini <-> Windows 11, sessões longas de Claude Code via Orb

---

## 📝 Notas de Escopo (Fora do MVP de 1 Mês)
Itens deliberadamente adiados para Fase 2 para proteger o prazo:
- Áudio desktop bidirecional do PC para o celular (foco inicial é controle de trabalho/dev e voz do usuário -> IA; áudio do PC entra na Fase 2 via segunda MediaTrack Opus).
- Cliente iOS / Web.
- Servidor de relay/signaling próprio na nuvem sem Tailscale.
- Transferência de arquivos por drag-and-drop.
