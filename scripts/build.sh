#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${1:-$ROOT/dist/light-fatura}"

rm -rf "$OUT"
install -d -m 0755 "$OUT"

cd "$ROOT"
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$OUT/light-fatura" ./cmd/server
cp -a "$ROOT/Light" "$OUT/Light"

# Fake/local customer index must never ship in the production bundle.
rm -rf "$OUT/Light/static/index"

echo "Bundle criado em: $OUT"
echo "Conteúdo: light-fatura + Light/"
