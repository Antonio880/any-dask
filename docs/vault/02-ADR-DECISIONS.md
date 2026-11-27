---
title: AnyDask — Registro de Decisões de Arquitetura (ADR)
status: ativo
updated: 2026-11-26
tags:
  - adr
  - decisions
  - architecture
---

# ⚖️ Registro de Decisões de Arquitetura (ADRs)

Voltar para [[00-INDEX]] | Ver arquitetura em [[01-ARCHITECTURE]]

---

## ADR-001: Transporte via WebRTC P2P (`pion/webrtc`) sobre Tailscale Mesh VPN

- **Data:** 2026-11-26
- **Status:** Proposto (Aguardando aprovação)

### Contexto
Precisamos conectar um smartphone Android a um Mac Mini (macOS) e um PC (Windows 11) em redes diferentes (Wi-Fi / 4G / 5G) com latência `< 50ms`, sem expor portas na internet pública e dentro de um prazo de 1 mês.

### Decisão
Utilizar **Tailscale (WireGuard)** como camada de túnel seguro privado entre Android, Mac Mini e Windows 11, executando **WebRTC direto (P2P)** com signaling WebSocket embarcado no próprio Host Agent.

### Alternativas Avaliadas e Rejeitadas
1. **Wrapper sobre Sunshine + Moonlight:** Excelente performance em Windows/NVIDIA, mas o suporte do Sunshine ao macOS (captura + injeção de input) é historicamente instável; além disso, o protocolo GameStream não oferece DataChannels flexíveis para sincronizar o Agente de IA, Kill Switch, áudio do Orb e overlays de coordenadas em uma única sessão coesa.
2. **Fork do RustDesk:** Código-fonte grande e acoplado a infraestrutura própria de relay/ID server; customizar UI Apple Glass + Computer Use profundo consumiria mais tempo entendendo o legado do que construindo um core enxuto focado no nosso caso de uso.
3. **VNC / RDP / WebSocket com JPEG:** Latência e banda inaceitáveis para uso fluído (não usa hardware H.264 stateful nem transporte UDP sem head-of-line blocking).
4. **WebRTC com servidor STUN/TURN próprio na nuvem:** Exige provisionar, pagar e proteger servidor público (coturn + auth server), aumentando complexidade de infraestrutura no MVP de 1 mês.

### Consequências
- **Positivas:** Zero portas abertas; criptografia dupla (WireGuard + DTLS-SRTP); conexão WebRTC fecha instantaneamente via `ICE host candidates`; funciona em CGNAT/4G/5G sem manter servidor TURN.
- **Trade-off:** Exige o app Tailscale ativo no Android, Mac Mini e Windows 11 (setup único de 2 minutos). No futuro (Fase 2), pode-se adicionar signaling/TURN opcional sem mudar o protocolo WebRTC.

---

## ADR-002: Host Agent Unificado em Go com Bridges Nativos (macOS & Windows)

- **Data:** 2026-11-26
- **Status:** Proposto (Aguardando aprovação)

### Contexto
O Host Agent precisa rodar com baixo uso de CPU/RAM em macOS e Windows 11, fazer streaming WebRTC H.264 acelerado por hardware, injetar eventos de mouse/teclado nativos e executar o Orquestrador de IA.

### Decisão
Construir o Host Agent em **Go**, utilizando **`pion/webrtc`**, com pacotes isolados por plataforma (`build tags` `darwin` e `windows`) para captura de tela GPU/H.264 e injeção de input nativa (`CGEventPost` no macOS e `SendInput` no Windows). Na Semana 1, usar uma abstração `ScreenCapturer` que permita subir vídeo H.264 imediatamente e plugar o backend nativo (`ScreenCaptureKit + VideoToolbox` / `DXGI + MediaFoundation`) sem alterar o restante do pipeline.

### Alternativas Avaliadas e Rejeitadas
1. **Rust puro:** Performance excelente, mas o ecossistema cross-platform de WebRTC + VideoToolbox + DXGI/NVENC exige muito código `unsafe`/FFI e tempo de compilação/debug que coloca em risco o prazo de 1 mês.
2. **Python:** Excelente para IA, mas péssimo para streaming de vídeo 60fps de baixa latência (GIL, overhead de memória, `aiortc` significativamente mais lento que `pion/webrtc` e difícil de empacotar como serviço leve).
3. **Node.js / Electron:** Alto consumo de RAM em segundo plano e falta de acesso nativo direto de baixa latência a DXGI/ScreenCaptureKit/CGEvent sem múltiplos módulos nativos C++ frágeis.

### Consequências
- **Positivas:** Binário único e leve; `pion/webrtc` é maduro e battle-tested; goroutines simplificam drasticamente o pipeline concorrente (Video + InputArbiter + AI Orchestrator + Kill Switch com `context.WithCancel`).
- **Trade-off:** Requer pequenas bridges em Objective-C/Swift (CGo) no macOS para `ScreenCaptureKit`/`VideoToolbox` e syscalls Win32/COM no Windows para `DXGI`.

---

## ADR-003: Cliente Android Nativo em Kotlin + Jetpack Compose

- **Data:** 2026-11-26
- **Status:** Proposto (Aguardando aprovação)

### Contexto
O app mobile será usado exclusivamente em smartphone Android, exigindo decodificação de vídeo H.264 por hardware com mínima latência, captura de gestos de alta frequência (trackpad/toque) e interface inspirada na Apple com **Glassmorphism e blur em tempo real** sobre o stream de vídeo.

### Decisão
Desenvolver nativamente em **Kotlin + Jetpack Compose**, usando `libwebrtc` (`io.getstream:stream-webrtc-android`), renderização em `TextureView` (para permitir blur real do conteúdo do stream por trás dos painéis de vidro) e **Haze / RenderEffect (AGSL)** para Glassmorphism fluido a 60/120fps.

### Alternativas Avaliadas e Rejeitadas
1. **Flutter:** `SurfaceView`/PlatformViews de WebRTC têm restrições sérias com `BackdropFilter` (blur em tempo real sobre vídeo nativo frequentemente falha ou causa quedas severas de FPS no Android).
2. **React Native:** Bridge adiciona jitter em gestos de trackpad de 120Hz e o suporte a `RenderEffect`/AGSL shaders customizados (efeito líquido do Orb estilo Siri) é muito inferior ao Compose nativo.

### Consequências
- **Positivas:** Estética Apple Glass de altíssimo nível sobre vídeo ao vivo; acesso direto ao `MediaCodec`, `AudioRecord` e `Android Keystore`; zero overhead de abstração multiplataforma desnecessária.
- **Trade-off:** Código mobile exclusivo para Android (se futuramente houver cliente iOS, será construído em SwiftUI reaproveitando 100% do Host Agent e do protocolo).

---

## ADR-004: Orquestrador de Computer Use Embarcado no Host Agent

- **Data:** 2026-11-26
- **Status:** Proposto (Aguardando aprovação)

### Contexto
O Agente de IA precisa capturar frames da tela, inspecionar a janela ativa/acessibilidade, chamar APIs de LLM (Claude/Gemini/OpenAI) e executar cliques, digitação e comandos shell em loop rápido.

### Decisão
Executar o **Orquestrador de IA diretamente no Host Agent** (macOS / Windows), recebendo o comando inicial e as credenciais de sessão pelo DataChannel `control` vindo do Android.

### Alternativas Avaliadas e Rejeitadas
1. **Orquestrador rodando no Android:** Obrigado a capturar frames já comprimidos pelo vídeo (menor nitidez de texto pequeno para OCR/visão), enviar via 4G/5G para a LLM e enviar cada clique de volta pela rede — dobrando a latência de cada passo e consumindo muito pacote de dados móvel.
2. **Servidor Cloud Intermediário:** Adiciona custo mensal, ponto único de falha, preocupação de privacidade (screenshots da tela pessoal passando por um backend extra) e mais código para manter no prazo de 1 mês.

### Consequências
- **Positivas:** Screenshots nítidos retirados direto do framebuffer local antes da compressão agressiva de vídeo; execução direta de shell + árvore de acessibilidade; Kill Switch determinístico no mesmo processo do `InputArbiter`.
- **Trade-off:** O Host precisa de acesso à internet para chamar as APIs das LLMs (o que já é garantido pois está conectado ao Android).

---

## ADR-005: Arquitetura Model-Agnostic (Strategy/Adapter) + BYOK

- **Data:** 2026-11-26
- **Status:** Proposto (Aguardando aprovação)

### Contexto
Modelos de Computer Use evoluem rapidamente e variam em custo/velocidade (ex: Gemini Flash para tarefas rápidas + áudio nativo, Claude Sonnet para raciocínio visual complexo de código/UI, OpenRouter para flexibilidade).

### Decisão
Isolar todos os provedores atrás de uma interface `LLMProvider` com esquema de ações canônico (`mouse_move`, `click`, `double_click`, `drag`, `type`, `hotkey`, `scroll`, `shell`, `wait`, `done`) e `CoordinateMapper` agnóstico a resolução. Suportar nativamente **Anthropic**, **Google Gemini** e **OpenAI/OpenRouter** com troca em tempo real na tela de Configurações do Android.

### Consequências
- **Positivas:** Sem vendor lock-in; permite usar perfis diferentes (ex: *"Rápido/Barato"* vs *"Precisão Máxima"*) direto no overlay do Orb.
- **Trade-off:** Cada adapter precisa traduzir pequenas diferenças de formato de coordenadas (pixels escalados no Claude vs escala `0..1000` no Gemini).

---

## ADR-006: Segurança Operacional, `InputArbiter` e Guardrails do Agente

- **Data:** 2026-11-26
- **Status:** Proposto (Aguardando aprovação)

### Contexto
Um agente com controle real de mouse, teclado e terminal pode causar danos se interpretar mal uma instrução ou alucinar um clique.

### Decisão
1. **Kill Switch de Prioridade Máxima:** Botão sempre visível no Android enquanto o agente age; cancela o `context.Context`, bloqueia injeção de input no `InputArbiter` em `< 10ms` e mata processos shell iniciados pela tarefa.
2. **Acesso Fail-Closed:** Perda de conexão WebRTC (> 3s) aborta o agente automaticamente.
3. **Guardrails Configuráveis:** Comandos shell potencialmente destrutivos (`rm -rf`, `sudo`, `format`, `git push --force`, etc.) ou áreas sensíveis exigem confirmação em 1 toque no Android antes da execução (com opção de desativar por sessão se o usuário desejar autonomia total).
4. **Pairing Autenticado:** Host só aceita conexões assinadas com token/chave pareada via QR Code no primeiro setup.

### Consequências
- **Positivas:** Uso seguro em máquinas pessoais de produção (Mac Mini e PC Windows 11).
- **Trade-off:** Pequena etapa de pairing inicial (feita uma única vez por máquina).

---

## ADR-007: Estratégia Dual-Track Mobile — PWA Fullscreen Apple Glass Embarcado Antes do APK Nativo

- **Data:** 2026-11-26
- **Status:** Aprovado

### Contexto
O ambiente atual de desenvolvimento no Windows 11 não possui Android Studio/JDK/Android SDK instalados ainda, e o usuário deseja ter o sistema completo funcionando no smartphone Android o mais rápido possível, sem abrir mão de `< 50ms` de latência nem da estética Apple Glass.

### Decisão
Entregar primeiro um **Cliente Mobile PWA Fullscreen Apple Glass** servido diretamente pelo próprio Host Agent (`//go:embed`), utilizando o engine WebRTC nativo do Chrome Android (`libwebrtc` + decode H.264 por hardware `MediaCodec`) e `backdrop-filter: blur(28px)` acelerado por GPU. Mantemos o mesmo protocolo binário/JSON de WebRTC para plugar o cliente **Kotlin + Jetpack Compose** (APK nativo, ADR-003) na sequência sem reescrever o backend.

### Consequências
- **Positivas:** Uso real no Android no Dia 1 via QR Code / "Adicionar à Tela Inicial" (fullscreen sem barra de URL); zero fricção de build APK inicial; Glassmorphism real sobre `<video>` com 60/120fps; suporta também abertura instantânea em qualquer dispositivo (Android, iPad, outro notebook).
- **Trade-off:** Armazenamento de API Keys fica em `localStorage`/IndexedDB criptografado na sessão do navegador até a chegada do APK nativo com Android Keystore.

