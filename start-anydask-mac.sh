#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if ! command -v go >/dev/null 2>&1; then
  echo "[ERRO] Go nao encontrado. Instale com: brew install go"
  exit 1
fi

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "[ERRO] FFmpeg nao encontrado. Instale com: brew install ffmpeg"
  exit 1
fi

mkdir -p bin

echo "Compilando AnyDask Host Agent para macOS ($(uname -m))..."
(cd host && go build -o ../bin/anydask-host ./cmd/anydask-host)

echo ""
echo "DICA (1a vez no macOS): conceda permissoes ao Terminal em:"
echo "  Ajustes do Sistema -> Privacidade e Seguranca -> Gravacao de Tela"
echo "  Ajustes do Sistema -> Privacidade e Seguranca -> Acessibilidade"
echo ""

exec ./bin/anydask-host "$@"
