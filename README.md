# AnyDask — Remote Desktop Ultra-Low Latency + Agente de IA Computer Use

Sistema próprio de acesso remoto de baixíssima latência (`~10–15ms @ 60fps`) via **WebRTC H.264 acelerado por hardware (`ICE-Lite`)** sobre **Tailscale / LAN**, com estética **iOS 26 Liquid Glass** tanto no **App Android nativo** quanto na **tela do computador controlado**, **Multi-Monitor ao vivo**, e um **Bot Assistente de IA (Computer Use)** acionado sob demanda por voz (*Hold-to-Talk*) ou texto.

> Toda a arquitetura, ADRs, Design System e Backlog vivo estão no Obsidian Memory Bank em [`docs/vault/00-INDEX.md`](docs/vault/00-INDEX.md).

---

## Principais Recursos

- **Acesso Remoto `< 20ms` @ 60fps:** WebRTC P2P (`pion/webrtc/v4`) com hardware H.264 (`ddagrab -> h264_amf/nvenc/qsv` no Windows 11; `avfoundation -> h264_videotoolbox` no macOS) + alternância **`HD` / `Turbo` (720p)** em 1 toque.
- **Multi-Monitor Nativo:** troca instantânea entre Tela 1 / Tela 2+ (`Tela 1/2`) sem desconectar, sincronizando vídeo, toques exatos, screenshots da IA e o overlay Liquid Glass no monitor ativo.
- **Estética iOS 26 Liquid Glass (Dual-Side):**
  - **No celular:** app Android nativo em tela cheia imersiva, superfícies de vidro líquido, Toque Direto como padrão (clica no pixel exato), Pinch-to-Zoom (`1x–4x`) + Pan suave, rotação Paisagem em 1 toque e barra de atalhos Dev retrátil.
  - **Na tela do computador controlado (Windows 11 & macOS):** overlay nativo 100% *click-through* com borda líquida Apple Intelligence, Dynamic Island no topo (com `NSVisualEffectView` blur real no macOS) e *Liquid Glass Ripples* a cada clique — ative/desative em 1 toque no botão **Vidro** (ou inicie com `--no-overlay`).
- **Bot Assistente Virtual de Computer Use (Sob Demanda, Model-Agnostic):**
  - Fica 100% inativo durante uso manual (`0%` overhead/tokens).
  - **Segure o Bot flutuante** para gravar voz nativamente (*Hold-to-Talk*, funciona em HTTP/LAN/Tailscale) e **solte** para enviar na hora.
  - **Console ao Vivo Liquid Glass** logo abaixo da Dynamic Island mostrando cada etapa em tempo real + botão vermelho **`PARAR`** (Kill Switch `< 10ms`) + guardrails para comandos destrutivos.
  - Suporta **Google Gemini** (padrão rápido `gemini-3.1-flash-lite` com fallback automático e áudio multimodal nativo), **Anthropic Claude** e **OpenAI / OpenRouter** (BYOK).

---

## 1. Instalação e Execução no Windows 11 (PC)

### Pré-requisitos (1ª vez)
```powershell
winget install --id GoLang.Go --exact --accept-source-agreements --accept-package-agreements
winget install --id Gyan.FFmpeg --exact --accept-source-agreements --accept-package-agreements
winget install --id Tailscale.Tailscale --exact --accept-source-agreements --accept-package-agreements
```
*(Reinicie o terminal após instalar para atualizar o PATH.)*

### Compilar e Rodar
```powershell
git clone https://github.com/Antonio880/any-dask.git
cd any-dask\host
go build -o ..\bin\anydask-host.exe .\cmd\anydask-host
cd ..
.\start-anydask.bat
```
> **Importante no Windows:** execute pelo seu terminal interativo ou duplo clique em `start-anydask.bat` (necessário para captura DXGI da sua sessão desktop). Se o Firewall do Windows pedir permissão para `anydask-host.exe`, marque **Privada e Pública**.

---

## 2. Instalação e Execução no Mac (Mac Mini / MacBook — 100% Gratuito)

Não precisa de conta Apple Developer — tudo roda localmente com ferramentas open-source gratuitas.

### Passo 1 — Dependências (apenas 1ª vez)
```bash
xcode-select --install
brew install go ffmpeg
brew install --cask tailscale
```
*(Se ainda não tiver Homebrew: `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)*

### Passo 2 — Clonar e Iniciar
```bash
git clone https://github.com/Antonio880/any-dask.git
cd any-dask
chmod +x start-anydask-mac.sh
./start-anydask-mac.sh
```
O script compila o binário nativo (`arm64` ou `x86_64`) com aceleração `h264_videotoolbox` + `NSVisualEffectView` Liquid Glass e imprime no terminal as URLs `[LAN]` e `[TAILSCALE]` + o **Token**.

### Passo 3 — Permissões do macOS (apenas 1ª vez)
Vá em **Ajustes do Sistema -> Privacidade e Segurança** e marque o **Terminal** (ou iTerm) em:
1. **Gravação de Tela e Áudio do Sistema** *(transmissão 60fps)*
2. **Acessibilidade** *(controle de mouse, cliques e atalhos `Cmd`)*

Depois feche (`Ctrl+C`) e execute `./start-anydask-mac.sh` novamente.

---

## 3. Acesso Fora de Casa (4G/5G — 100% Gratuito via Tailscale)

1. Faça login com a **mesma conta** (Google/GitHub) no app **Tailscale** no **Windows 11**, no **Mac** e no **Android** (Google Play Store).
2. Deixe o Tailscale ligado nos aparelhos. O terminal do Host passará a exibir automaticamente:
   `[TAILSCALE] http://100.x.y.z:8787/?token=...`
3. **Como deixar acessível 24/7 sem cair:**
   - **Windows 11:** *Configurações -> Sistema -> Energia -> Tela e suspensão* -> **Suspender = Nunca** (não bloqueie com `Win+L`; se quiser apagar a tela, desligue o monitor físico).
   - **Mac Mini / MacBook:** *Ajustes do Sistema -> Energia* -> ative **"Impedir repouso automático quando a tela estiver desligada"** (ou rode `caffeinate -di &`). Se o Mac Mini ficar sem monitor físico (*headless*), use o app gratuito **BetterDisplay** ou um HDMI Dummy Plug.
   - **Tailscale permanente:** em `https://login.tailscale.com/admin/machines` -> clique em `...` em cada máquina -> **"Disable key expiry"** (remove a expiração padrão de 180 dias).

---

## 4. Aplicativo Android Nativo (`AnyDask.apk`)

### Instalar / Atualizar em 1 toque pelo Host
Se o APK já estiver compilado em `bin/anydask-android.apk`, basta abrir no Chrome do Android:
```text
http://<IP-TAILSCALE-OU-LAN>:8787/download/anydask.apk
```
Toque no arquivo baixado -> **Instalar / Atualizar** (conceda permissão de Microfone para o *Hold-to-Talk* nativo).

### Compilar o APK do zero (opcional, em Windows/macOS com JDK 17 + Android SDK 35)
```bash
cd android
gradle assembleDebug --no-daemon
# APK gerado em: android/app/build/outputs/apk/debug/app-debug.apk
```

### Alternar entre Windows 11 e Mac no mesmo app
Toque em **`Hosts`** na barra inferior -> **Adicionar máquina** -> informe o Nome, a URL Tailscale (`http://100.x.y.z:8787`) e o Token do outro computador. Depois basta 1 toque para alternar entre eles (e a barra troca automaticamente `Ctrl` / `Cmd`).

---

## 5. Como Usar & Controles

### Gestos na Tela (Modo Toque Direto — Padrão)
- **1 toque:** clique esquerdo exato no ponto tocado (funciona mesmo com zoom `2x–4x`).
- **Duplo toque:** duplo clique no ponto tocado.
- **Segurar 1 dedo (~400ms):** clique direito no ponto tocado.
- **Pinch com 2 dedos:** zoom suave (`1x` até `4x`).
- **Arrastar com 1 dedo (com zoom > 1x):** move a câmera livremente (Pan).
- **Toque em `Toque / Track`:** alterna para modo Trackpad relativo quando preferir.

### Barra Inferior (2 Linhas Liquid Glass)
- **Linha 1 (Controles):** `Tela 1/2` (trocar monitor) · `HD / Turbo` (1080p vs 720p ultra-rápido) · `Vidro` (overlay Liquid Glass no PC/Mac) · `1x` (zoom rápido `2.2x`) · `Girar` (Paisagem/Retrato) · `Toque` · `Teclado` · `Hosts` · `Config` (API Keys BYOK).
- **Linha 2 (Atalhos Dev):** `Ctrl / Cmd`, `Alt`, `Shift`, `Esc`, `Tab`, `^C`, `Enter`, setas, `/`, `|`, `` ` ``, `~`.
- Toque no handle superior (`11ms · 60fps`) para recolher/mostrar a barra inteira.

### Bot Assistente Virtual (Computer Use)
- **Segurar o Bot flutuante:** começa a gravar sua voz na hora (*Hold-to-Talk*) -> **soltar** envia direto para a IA agir.
- **1 toque no Bot:** abre o painel com campo de texto, chips rápidos e seletor de modelo.
- **Arrastar o Bot:** move livremente pela tela com física de mola e snap magnético nas bordas.
- **Botão vermelho `PARAR` (Dynamic Island):** interrompe a IA imediatamente (`< 10ms`) e devolve o controle manual.

### Configurar API Key Padrão no Host (Opcional)
Crie `~/.anydask/ai-config.json` (no Windows: `%USERPROFILE%\.anydask\ai-config.json`):
```json
{
  "provider": "gemini",
  "model": "gemini-3.1-flash-lite",
  "api_key": "SUA_API_KEY_AQUI",
  "base_url": "",
  "require_confirm": true
}
```
Ou configure diretamente no celular tocando em **Configurações** (suporta **Gemini**, **Claude** e **OpenAI/OpenRouter**).

---

## 6. Como Atualizar Depois de Novas Melhorias

- **No Mac:**
  ```bash
  cd any-dask
  git pull
  ./start-anydask-mac.sh
  ```
- **No Windows 11:**
  ```powershell
  cd any-dask
  git pull
  cd host
  go build -o ..\bin\anydask-host.exe .\cmd\anydask-host
  cd ..
  .\start-anydask.bat
  ```

---

## Flags Opcionais do Host Agent
```text
anydask-host --addr :8787 --fps 60 --bitrate 6500 --encoder "" --token "" --no-overlay
```
