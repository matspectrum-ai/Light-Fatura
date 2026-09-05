#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "Execute como root." >&2
  exit 1
fi

BASE=/opt/light-fatura
CURRENT="$BASE/current"
PREVIOUS="$BASE/previous"

if [[ ! -L "$PREVIOUS" ]]; then
  echo "Nenhuma release anterior registrada." >&2
  exit 1
fi

target="$(readlink -f "$PREVIOUS")"
if [[ ! -x "$target/light-fatura" || ! -f "$target/Light/index.html" ]]; then
  echo "Release anterior inválida: $target" >&2
  exit 1
fi

tmp="${CURRENT}.tmp.$$"
rm -f "$tmp"
ln -s "$target" "$tmp"
mv -Tf "$tmp" "$CURRENT"
systemctl restart light-fatura.service

for _ in $(seq 1 20); do
  if curl --fail --silent --max-time 2 http://127.0.0.1:8080/healthz >/dev/null; then
    echo "Rollback concluído: $target"
    exit 0
  fi
  sleep 1
done

echo "Rollback aplicado, mas o health check falhou." >&2
exit 1
