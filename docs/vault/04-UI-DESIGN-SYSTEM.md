---
title: AnyDask — UI Design System (Apple Glass)
status: proposta-v1
updated: 2026-11-26
tags:
  - design-system
  - ui
  - glassmorphism
  - android
  - compose
---

# UI Design System — Estética Apple Premium (visionOS / iOS)

Voltar para [[00-INDEX]] | Ver arquitetura em [[01-ARCHITECTURE]] | Ver backlog em [[03-BACKLOG-PROGRESS]]

---

## 1. Princípios de Design

1. **Conteúdo em Primeiro Plano:** Durante o uso manual, `98%` da tela pertence ao vídeo remoto do Mac Mini ou Windows 11. Toda a chrome de interface é retrátil, flutuante e translúcida.
2. **Profundidade por Luz e Blur (Glassmorphism Real):** Não usar cinzas chapados opacos. Superfícies devem desfocar em tempo real o stream de vídeo por trás (`TextureView` + `Haze` / `RenderEffect` AGSL), com borda superior/interna especular de `1dp` simulando refração de vidro estilo visionOS.
3. **Fisicalidade Orgânica:** Todas as transições usam física de mola (`Spring.DampingRatioNoBouncy` / `MediumBouncy`, `StiffnessMediumLow`) sem movimentos lineares robóticos.
4. **Feedback Háptico Sutil:** Haptics diferenciados no Android (`HapticFeedbackType` / `VibrationEffect`) para clique de mouse, ativação do Orb, passo do agente e acionamento do Kill Switch.
5. **Zero Emojis (Estética Nativa Apple):** Nunca usar emojis na interface, nos logs ao vivo nem na documentação. Usar exclusivamente ícones vetoriais SVG limpos (estilo SF Symbols) e tipografia clara.

---

## 2. Design Tokens

### A. Superfícies de Vidro (Dark-First Adaptativo)
O app adota estética **Obsidian Glass** para manter alto contraste independentemente de a tela remota estar em um editor escuro (VS Code/Terminal) ou uma página branca (Navegador).

| Token | Valor / Regra | Uso |
| :--- | :--- | :--- |
| `glass.surface.thin` | `#1C1C1E` @ `52%` + Blur `20.dp` | Barra de atalhos Dev, pills discretas |
| `glass.surface.regular` | `#16161A` @ `68%` + Blur `28.dp` | Overlay compacto do Agente, Host Switcher |
| `glass.surface.thick` | `#121215` @ `82%` + Blur `36.dp` | Tela de Configurações (API Keys), modais de confirmação |
| `glass.border.specular` | Gradiente linear `White @ 28%` (topo-esq) -> `White @ 6%` (base-dir), `1.dp` | Borda iluminada de todos os containers de vidro |
| `glass.shadow.ambient` | `#000000` @ `45%`, elevação difusa `24.dp` | Separação visual sobre fundos claros |

### B. Paleta Semântica & Acentos (Siri / Dynamic Island)
| Token | Hex | Uso |
| :--- | :--- | :--- |
| `accent.primary` | `#0A84FF` (Apple System Blue) | Ações primárias, máquina selecionada |
| `accent.orb.a` | `#5E5CE6` (Indigo) | Gradiente vivo do Orb (Pensando/Ouvindo) |
| `accent.orb.b` | `#BF5AF2` (Purple) | Gradiente vivo do Orb |
| `accent.orb.c` | `#32D74B` / `#64D2FF` (Cyan/Mint) | Halo de ação / pulso de voz |
| `status.online` | `#30D158` | Host conectado / latência `< 50ms` |
| `status.warning` | `#FFD60A` | Latência moderada / ação requer confirmação |
| `status.danger` | `#FF453A` | **Botão PARAR / Kill Switch**, erro crítico |
| `text.primary` | `#FFFFFF` @ `95%` | Títulos e comandos principais |
| `text.secondary` | `#EBEBF5` @ `65%` | Metadados (FPS, RTT, modelo ativo) |
| `text.tertiary` | `#EBEBF5` @ `38%` | Placeholders e labels discretos |

### C. Tipografia & Raios
- **Fonte UI:** `Inter` / `Roboto Flex` com tracking ligeiramente ajustado (`-0.2sp` em títulos, tabular numerals `tnum` para contadores de ms/FPS).
- **Fonte Código / Atalhos:** `JetBrains Mono` (comandos shell, teclas da DevBar, logs do agente).
- **Raios de Borda (Squircle feel):**
  - `radius.pill`: `999.dp` (Orb, pills de status, botões de atalho)
  - `radius.sheet`: `28.dp` (Overlay do Agente, Host Switcher)
  - `radius.card`: `20.dp` (Cards de máquina e configurações)
  - `radius.key`: `10.dp` (Teclas da DevBar)

---

## 3. O Orb Flutuante (Botão de IA Sob Demanda)

O Orb é a assinatura visual do app. Inspirado na **Siri (Apple Intelligence) + Dynamic Island**.

### Comportamento Espacial
- **Tamanho Inativo:** `44.dp x 44.dp`, semi-translúcido (`opacity: 0.65`) quando sem interação por > 3 segundos, para nunca atrapalhar a visão da tela remota.
- **Arrastável Livremente:** O usuário pode arrastar o Orb para qualquer ponto da tela; ao soltar, ele faz **snap magnético com mola** para a borda esquerda ou direita mais próxima (mantendo margem segura de `12.dp` das bordas e da DevBar).
- **Interações:**
  - **1 Toque:** Expande suavemente (morphing estilo Dynamic Island) para o **Overlay Compacto** (voz + texto).
  - **Segurar (Long-Press):** Ativa modo **Push-to-Talk direto** — enquanto segura, grava a voz; ao soltar, já dispara a tarefa para o Agente!

### Máquina de Estados Visual do Orb
| Estado | Visual & Animação | Ação ao Tocar |
| :--- | :--- | :--- |
| **`IDLE` (Inativo)** | Círculo de vidro fosco discreto com núcleo luminoso mínimo (quase estático). | Abre Overlay Compacto |
| **`LISTENING` (Ouvindo)** | Expande levemente (`52.dp`), shader AGSL colorido em ondas reativas à amplitude do microfone. | Finaliza gravação e envia |
| **`THINKING` (Pensando)** | Anel líquido rotativo suave (Indigo -> Purple -> Cyan) + pill lateral compacta *"Analisando tela..."*. | Expande detalhes / mostra botão Parar |
| **`ACTING` (Agindo no PC)** | Transforma-se em **Pill Dynamic Island** compacta no topo/canto com indicador pulsante + texto da ação atual (ex: *"Clicando em Terminal..."*) + **botão vermelho `PARAR` em 1 toque**. | Toque em `PARAR` dispara Kill Switch `< 10ms` |
| **`CONFIRM` (Aguardando)** | Pulso âmbar suave + mini-card glass com comando proposto e botões `Recusar` / `Permitir`. | Aprova ou cancela ação |

---

## 4. Overlay Compacto de Instrução

Ao tocar no Orb, um cartão translúcido (`glass.surface.regular`, `radius.sheet`) flutua próximo à parte inferior (acima do teclado se aberto) **sem escurecer totalmente a tela remota**, permitindo que o usuário continue vendo o contexto do computador enquanto dita ou digita.

### Elementos do Overlay
1. **Cabeçalho Minimalista:** Chip do host ativo (`Mac Mini · macOS` ou `Workstation · Win11`) + seletor rápido de perfil/modelo (ex: `Claude Sonnet`, `Gemini Pro`, `OpenRouter`).
2. **Área de Input Híbrida:**
   - Campo de texto limpo (`"Peça para eu agir na tela..."`).
   - Botão de Microfone proeminente (Push-to-Talk ou toggle) com visualizador de forma de onda.
3. **Chips de Sugestão Contextual (Opcionais/Rápidos):** Ex: *"Explicar erro na tela"*, *"Abrir terminal aqui"*, *"Corrigir e rodar testes"*.
4. **Modo Acompanhamento (quando `ACTING`):** O overlay retrai automaticamente para a **Pill Dynamic Island** deixando `92%` da tela livre para observar o cursor da IA trabalhando ao vivo, com um **halo de alvo** sutil renderizado sobre o stream indicando onde a IA está clicando.

---

## 5. Barra de Atalhos Rápidos Dev (Retrátil)

Posicinada na borda inferior (ou lateral em modo paisagem), em `glass.surface.thin`, colapsável em 1 gesto de swipe:
- **Grupo 1 (Modificadores Sticky — toque fixa até a próxima tecla):**
  - `⌘ Cmd` (no macOS) / `Ctrl` (no Windows) — adapta o rótulo automaticamente ao SO conectado!
  - `⌥ Opt / Alt`, `⇧ Shift`, `⌃ Ctrl`
- **Grupo 2 (Teclas Dev Essenciais):**
  - `Esc`, `Tab`, `Ctrl+C` (interrupção rápida de terminal), `↑ ↓ ← →` (cluster compacto ou swipe), `/`, `|`, `` ` ``
- **Grupo 3 (Controles de Sessão):**
  - Ícone de **Teclado Android**
  - **Host Switcher Pill** (1 toque para alternar entre `Mac Mini` e `Windows 11`)
  - Botão de **Modo Cursor** (Trackpad relativo vs Toque direto absoluto)
  - Badge discreto de saúde da conexão (`18ms · 60fps`)
