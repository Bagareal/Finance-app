#!/usr/bin/env bash
# Finance App - installa o aggiorna il binario dall'ultima release GitHub.
# Gira DENTRO il container. Si lancia con il comando `update` (installato da financeapp-install.sh).
#
# Passi: scarica la release -> verifica sha256 -> ferma il servizio -> backup del DB ->
#        sostituisce il binario -> riavvia -> se non risponde, ripristina la versione precedente.
set -Eeuo pipefail

FA_REPO="${FA_REPO:-Bagareal/Finance-app}"
APP_DIR="/opt/financeapp"
BIN="$APP_DIR/financeapp"
DATA="$APP_DIR/data"
SERVICE="financeapp"
PORT_URL="http://127.0.0.1:8080/api/status"

GN=$'\e[32m'; RD=$'\e[31m'; YW=$'\e[33m'; CL=$'\e[0m'
info() { echo -e "${YW}»${CL} $*"; }
ok() { echo -e "${GN}✔${CL} $*"; }
die() { echo -e "${RD}✘ $*${CL}" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "Esegui come root."

# ripara le installazioni vecchie: il comando `update` deve essere raggiungibile dal PATH minimo di Proxmox
[[ -x /usr/local/bin/update && ! -e /usr/bin/update ]] && ln -sf /usr/local/bin/update /usr/bin/update

case "$(dpkg --print-architecture)" in
  amd64) ARCH=amd64 ;;
  arm64) ARCH=arm64 ;;
  *) die "Architettura non supportata: $(dpkg --print-architecture)" ;;
esac

info "Cerco l'ultima release di $FA_REPO..."
TAG="$(curl -fsSL "https://api.github.com/repos/$FA_REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1 || true)"
[[ -n "$TAG" ]] || die "Nessuna release trovata su https://github.com/$FA_REPO/releases"

CURRENT="$(cat "$APP_DIR/version" 2>/dev/null || echo none)"
if [[ "$CURRENT" == "$TAG" && -x "$BIN" && -z "${FORCE:-}" ]]; then
  ok "Finance App è già aggiornata ($TAG)."
  exit 0
fi
info "Versione installata: $CURRENT -> disponibile: $TAG"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
BASE="https://github.com/$FA_REPO/releases/download/$TAG"
ASSET="financeapp-linux-$ARCH"

info "Scarico $ASSET..."
curl -fsSL -o "$TMP/financeapp" "$BASE/$ASSET" || die "Download non riuscito: $BASE/$ASSET"
curl -fsSL -o "$TMP/checksums.txt" "$BASE/checksums.txt" || die "Download di checksums.txt non riuscito"
EXPECTED="$(awk -v f="$ASSET" '$2==f {print $1}' "$TMP/checksums.txt")"
[[ -n "$EXPECTED" ]] || die "Checksum per $ASSET non presente in checksums.txt"
ACTUAL="$(sha256sum "$TMP/financeapp" | awk '{print $1}')"
[[ "$EXPECTED" == "$ACTUAL" ]] || die "Checksum non valido: il file scaricato è corrotto."
ok "Checksum verificato"

HAD_BIN=0
if [[ -x "$BIN" ]]; then
  HAD_BIN=1
  info "Fermo il servizio e salvo i dati..."
  systemctl stop "$SERVICE" 2>/dev/null || true
  if [[ -f "$DATA/finance.db" ]]; then
    mkdir -p "$DATA/backups"
    cp "$DATA/finance.db" "$DATA/backups/finance-$CURRENT-$(date +%F-%H%M%S).db"
    # tiene solo gli ultimi 5 backup
    ls -1t "$DATA"/backups/finance-*.db 2>/dev/null | tail -n +6 | xargs -r rm -f
    ok "Backup in $DATA/backups"
  fi
  cp "$BIN" "$BIN.old"
fi

install -m 0755 "$TMP/financeapp" "$BIN"
chown -R financeapp:financeapp "$DATA"
systemctl daemon-reload
systemctl enable "$SERVICE" >/dev/null 2>&1
systemctl restart "$SERVICE"

healthy() {
  for _ in 1 2 3 4 5 6 7 8; do
    sleep 1
    if curl -fsS -o /dev/null "$PORT_URL" 2>/dev/null; then return 0; fi
  done
  return 1
}

if healthy; then
  echo "$TAG" >"$APP_DIR/version"
  rm -f "$BIN.old"
  ok "Finance App $TAG attiva su http://$(hostname -I | awk '{print $1}'):8080"
else
  echo -e "${RD}Il servizio non risponde dopo l'aggiornamento.${CL}" >&2
  journalctl -u "$SERVICE" -n 15 --no-pager >&2 || true
  if [[ $HAD_BIN -eq 1 && -f "$BIN.old" ]]; then
    info "Ripristino la versione precedente ($CURRENT)..."
    mv -f "$BIN.old" "$BIN"
    systemctl restart "$SERVICE" || true
    die "Aggiornamento annullato: ripristinata la versione $CURRENT."
  fi
  die "Installazione non riuscita."
fi
