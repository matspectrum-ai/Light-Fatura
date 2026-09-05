#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "Execute como root." >&2
  exit 1
fi

BUNDLE="${1:-}"
RELEASE_ID="${2:-$(date -u +%Y%m%dT%H%M%SZ)}"
if [[ -z "$BUNDLE" || ! -d "$BUNDLE" || ! -f "$BUNDLE/light-fatura" || ! -f "$BUNDLE/Light/index.html" ]]; then
  echo "Uso: sudo bash deploy/release.sh <diretorio-bundle> [release-id]" >&2
  echo "O bundle deve conter light-fatura e Light/index.html." >&2
  exit 1
fi
if [[ ! "$RELEASE_ID" =~ ^[A-Za-z0-9._-]+$ ]] || [[ "$RELEASE_ID" == .* ]] || [[ "$RELEASE_ID" == *..* ]]; then
  echo "Release ID inválido: $RELEASE_ID" >&2
  exit 1
fi
if [[ ! -f /etc/light-fatura/light-fatura.env ]]; then
  echo "Arquivo /etc/light-fatura/light-fatura.env ausente." >&2
  exit 1
fi
for cmd in install readlink ln mv systemctl curl seq sed cp; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Comando obrigatório ausente: $cmd" >&2; exit 1; }
done

BASE=/opt/light-fatura
RELEASE="$BASE/releases/$RELEASE_ID"
CURRENT="$BASE/current"
PREVIOUS="$BASE/previous"

atomic_link() {
  local target="$1" link="$2" tmp="${2}.tmp.$$"
  rm -f "$tmp"
  ln -s "$target" "$tmp"
  mv -Tf "$tmp" "$link"
}

if [[ -e "$RELEASE" ]]; then
  echo "Release já existe: $RELEASE" >&2
  exit 1
fi

install -d -m 0755 "$RELEASE"
install -m 0755 "$BUNDLE/light-fatura" "$RELEASE/light-fatura"
cp -a "$BUNDLE/Light" "$RELEASE/Light"
chown -R root:root "$RELEASE"
find "$RELEASE/Light" -type d -exec chmod 0755 {} +
find "$RELEASE/Light" -type f -exec chmod 0644 {} +

old=""
if [[ -L "$CURRENT" ]]; then
  old="$(readlink -f "$CURRENT")"
  atomic_link "$old" "$PREVIOUS"
fi

atomic_link "$RELEASE" "$CURRENT"
systemctl daemon-reload
systemctl restart light-fatura.service

healthy=0
for _ in $(seq 1 30); do
  if curl --fail --silent --max-time 2 http://127.0.0.1:8080/healthz >/dev/null; then
    healthy=1
    break
  fi
  sleep 1
done

if [[ "$healthy" -ne 1 ]]; then
  echo "Health check falhou para $RELEASE_ID." >&2
  if [[ -n "$old" ]]; then
    echo "Restaurando release anterior: $old" >&2
    atomic_link "$old" "$CURRENT"
    systemctl restart light-fatura.service
  else
    systemctl stop light-fatura.service || true
  fi
  exit 1
fi

# O health check valida o processo; estes arquivos validam que o bundle visual também foi publicado.
test -s "$CURRENT/Light/index.html"
test -s "$CURRENT/Light/assets/logolight.svg"
test -s "$CURRENT/Light/assets/hero-char.png"

systemctl --no-pager --full status light-fatura.service | sed -n '1,12p' || true
echo "Release ativa: $RELEASE"
