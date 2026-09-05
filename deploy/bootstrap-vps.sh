#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "Execute como root." >&2
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOMAIN="${1:-}"
if [[ -z "$DOMAIN" ]]; then
  echo "Uso: sudo bash deploy/bootstrap-vps.sh <dominio>" >&2
  exit 1
fi
if [[ ! "$DOMAIN" =~ ^[A-Za-z0-9.-]+$ ]] || [[ "$DOMAIN" == .* ]] || [[ "$DOMAIN" == *..* ]]; then
  echo "Domínio inválido: $DOMAIN" >&2
  exit 1
fi
for cmd in useradd install sed nginx systemctl; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Comando obrigatório ausente: $cmd" >&2; exit 1; }
done

if ! id light-fatura >/dev/null 2>&1; then
  useradd --system --home /nonexistent --shell /usr/sbin/nologin light-fatura
fi

install -d -m 0755 /opt/light-fatura/releases
install -d -m 0750 -o root -g light-fatura /etc/light-fatura

if [[ ! -f /etc/light-fatura/light-fatura.env ]]; then
  install -m 0640 -o root -g light-fatura "$ROOT/deploy/light-fatura.env.example" /etc/light-fatura/light-fatura.env
  echo "Criado /etc/light-fatura/light-fatura.env. Preencha as credenciais antes do primeiro deploy."
fi

install -m 0644 "$ROOT/deploy/systemd/light-fatura.service" /etc/systemd/system/light-fatura.service

if [[ -d /etc/nginx/sites-available ]]; then
  target=/etc/nginx/sites-available/light-fatura.conf
  sed "s/__DOMAIN__/$DOMAIN/g" "$ROOT/deploy/nginx/light-fatura.conf" > "$target"
  ln -sfn "$target" /etc/nginx/sites-enabled/light-fatura.conf
  rm -f /etc/nginx/sites-enabled/default
elif [[ -d /etc/nginx/conf.d ]]; then
  target=/etc/nginx/conf.d/light-fatura.conf
  sed "s/__DOMAIN__/$DOMAIN/g" "$ROOT/deploy/nginx/light-fatura.conf" > "$target"
else
  echo "Diretório de configuração do Nginx não encontrado." >&2
  exit 1
fi

nginx -t
systemctl daemon-reload
systemctl enable light-fatura.service
systemctl reload nginx

echo "Bootstrap concluído. Edite /etc/light-fatura/light-fatura.env e publique um bundle com deploy/release.sh."
