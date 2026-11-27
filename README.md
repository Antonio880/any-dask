# AnyDask — Remote Desktop (< 50ms) + Agente de IA Computer Use Sob Demanda

Sistema próprio de acesso remoto de baixíssima latência via **WebRTC H.264 acelerado por hardware** sobre **Tailscale / LAN**, com cliente mobile **Apple Glass (PWA Fullscreen)** e **Agente de IA de Computer Use sob demanda** (Claude, Gemini e OpenAI/OpenRouter).

> Toda a arquitetura, ADRs, Design System e Backlog vivo estão no Obsidian Memory Bank em [`docs/vault/00-INDEX.md`](docs/vault/00-INDEX.md).

---

## Como Rodar Agora

### 1. Windows 11 (PC)
```powershell
cd host
go build -o ..\bin\anydask-host.exe .\cmd\anydask-host
..\bin\anydask-host.exe
```

### 2. Mac Mini (macOS)
Pré-requisitos no Mac Mini (`brew install go ffmpeg`) e conceder permissões de **Gravação de Tela** e **Acessibilidade** ao terminal em *Ajustes do Sistema -> Privacidade e Segurança*:
```bash
cd host
go build -o ../bin/anydask-host ./cmd/anydask-host
./../bin/anydask-host
```

### 3. Smartphone Android
1. Abra a URL exibida no terminal (`[TAILSCALE]` para qualquer rede 4G/5G ou `[LAN]` no Wi-Fi local) no **Chrome do Android**.
2. Toque no menu do Chrome -> **"Adicionar à tela inicial"** para abrir em modo fullscreen nativo sem barra de endereço.
3. No app, toque em **`⚙` (Configurações)** para inserir sua API Key (**Gemini**, **Claude** ou **OpenAI/OpenRouter**).
4. Toque em **`Hosts`** para cadastrar tanto o **Windows 11** quanto o **Mac Mini** e alternar entre eles em 1 toque.

---

## Atalhos & Uso do Agente de IA
- **Modo Manual (padrão):** IA 100% inativa. Use **Trackpad** (1 dedo move, tap = clique esquerdo, 2 dedos tap = clique direito, 2 dedos arrastar = scroll, double-tap & segurar = arrastar) ou alterne para **Toque Direto**.
- **Orb Flutuante:** Arraste para qualquer canto (faz snap magnético nas bordas).
  - **1 Toque:** abre o overlay translúcido (texto + voz + chips rápidos).
  - **Segurar (Long-Press):** Push-to-Talk direto!
- **Botão `PARAR` (Dynamic Island):** Interrompe o agente em `< 10ms`, cancela chamadas LLM e encerra processos shell, devolvendo o controle manual imediatamente.
