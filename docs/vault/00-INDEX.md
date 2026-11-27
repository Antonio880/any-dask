---
title: AnyDask — Índice do Projeto & Memory Bank
status: fase-2-mvp-em-andamento
updated: 2026-11-26
tags:
  - index
  - moc
  - anydask
---

# 🧭 AnyDask — Mapa Geral do Projeto

> **Visão:** Sistema próprio de acesso remoto de baixíssima latência (< 50ms) para controlar um **Mac Mini (macOS)** e um **PC (Windows 11)** a partir de um **smartphone Android** com estética Apple Premium (Glassmorphism/visionOS) e um **Agente de IA de Computer Use sob demanda**, acionado por um Orb flutuante discreto e 100% agnóstico a modelo (Claude, Gemini, OpenAI/OpenRouter).

---

## 📚 Mapa do Vault (Obsidian MOC)

| Documento | Propósito | Status |
| :--- | :--- | :--- |
| [[00-INDEX]] | Mapa geral, protocolo de contexto e dashboard de status | 🟢 Ativo |
| [[01-ARCHITECTURE]] | Arquitetura de sistema, diagramas Mermaid, rede, WebRTC e pipeline de Computer Use | 🟢 Proposta v1 |
| [[02-ADR-DECISIONS]] | Registro de Decisões de Arquitetura (ADR-001 a ADR-006) e trade-offs | 🟢 Proposta v1 |
| [[03-BACKLOG-PROGRESS]] | Checklist vivo (To Do / In Progress / Done) e roadmap de 4 semanas | 🟢 Proposta v1 |
| [[04-UI-DESIGN-SYSTEM]] | Tokens visuais, regras de Glassmorphism, Orb flutuante e barra de atalhos Dev | 🟢 Proposta v1 |

---

## 🎯 Estado Atual do Projeto

- **Fase Atual:** `Fase 2 — Sistema Completo em Produção Pessoal (Windows 11 + macOS + Android APK v1.0.4)`
- **Performance Validada ao Vivo:** `10–13ms · 60fps` reais em Windows 11 (`ddagrab → h264_amf`) e macOS (`avfoundation → h264_videotoolbox`) sobre Tailscale (`ICE-Lite`), com modo **`⚡Turbo`** (720p) e **Multi-Monitor** (`🖥 1/2`).
- **Estética iOS 26 Liquid Glass Dual-Side:** Portal de conexão Liquid Glass, Bot Assistente com física de mola + Hold-to-Talk nativo (`v1.0.4`), Console ao Vivo da IA e Overlay nativo click-through na tela do PC/Mac acompanhando o monitor ativo.
- **IA Computer Use:** `gemini-3.1-flash-lite` validado em `~1.4s/passo` com fallback automático multi-modelo + adapters Anthropic Claude e OpenAI/OpenRouter.
- **Repositório:** `https://github.com/Antonio880/any-dask` (`branch main`, `README.md` 100% atualizado).

### Metas Não-Negociáveis (SLO / Qualidade)
1. **Latência ponta-a-ponta (Glass-to-Glass):** `< 50ms` em rede local/fibra e `< 80ms` em 4G/5G estável.
2. **Inatividade da IA por padrão:** `0%` de overhead de CPU/tokens/UI invasiva durante uso manual.
3. **Kill Switch do Agente:** `< 10ms` no Host para interromper mouse/teclado/shell e devolver controle imediato ao usuário.
4. **Multi-SO Nativo:** Mapeamento transparente de atalhos e captura GPU tanto no **macOS** quanto no **Windows 11**.
5. **Model-Agnostic (BYOK):** Troca imediata de provedor/modelo/API key nas configurações do Android (Anthropic Claude, Google Gemini, OpenAI / OpenRouter).

---

## 🧠 Protocolo Obrigatório de Gestão de Contexto

Para manter continuidade total entre sessões de desenvolvimento:
1. **Antes de qualquer tarefa:** Ler [[00-INDEX]] e [[03-BACKLOG-PROGRESS]] para alinhar estado e prioridades.
2. **Antes de decisões técnicas relevantes:** Consultar [[02-ADR-DECISIONS]] e [[01-ARCHITECTURE]].
3. **Antes de construir ou ajustar telas mobile:** Consultar [[04-UI-DESIGN-SYSTEM]].
4. **Ao concluir qualquer módulo ou mudança relevante:** Atualizar automaticamente os status em [[03-BACKLOG-PROGRESS]], registrar novas decisões em [[02-ADR-DECISIONS]], refletir mudanças estruturais em [[01-ARCHITECTURE]] e [[00-INDEX]], e **manter o `README.md` na raiz sempre sincronizado com o passo a passo atualizado**.

---

## 🗺️ Resumo da Stack Proposta

- **Rede & Segurança:** Tailscale (WireGuard Mesh) + WebRTC P2P direto (`ICE host candidates`, sem abrir portas no roteador).
- **Host Agent (macOS + Windows 11):** **Go** (`pion/webrtc`) + capturadores/encoders nativos via GPU (`ScreenCaptureKit + VideoToolbox` no macOS; `DXGI Desktop Duplication + Media Foundation/NVENC` no Windows) + injeção de input nativa (`CGEvent` / `SendInput`).
- **Orquestrador de Computer Use:** Módulo Go embarcado no Host Agent com padrão **Strategy/Adapter** para LLMs, `InputArbiter` determinístico e ferramentas de GUI + Acessibilidade + Shell.
- **Cliente Mobile (Android):** **Kotlin + Jetpack Compose** nativo, `libwebrtc` (decodificação H.264 por hardware via `MediaCodec`), Glassmorphism real com **Haze / AGSL RenderEffect** e animação fluida a 60/120fps.

> Detalhes completos em [[01-ARCHITECTURE]] e justificativas em [[02-ADR-DECISIONS]].
