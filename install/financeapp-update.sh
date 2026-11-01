#!/usr/bin/env bash
# Finance App - installa o aggiorna l'app e i pacchetti di sistema del container.
# Gira DENTRO il container, di solito tramite il comando `update`.
#
#   update           aggiorna l'app e poi i pacchetti Debian
#   update --app     aggiorna solo l'app
#
# Aggiornamento dell'app: scarica l'ultima release GitHub -> verifica sha256 -> ferma il servizio ->
# backup del DB -> sostituisce il binario -> riavvia -> se non risponde, ripristina la versione precedente.
set -Eeuo pipefail

FA_REPO="${FA_REPO:-Bagareal/Finance-app}"
AUTO_UPDATES="${AUTO_UPDATES:-yes}"
APP_DIR="/opt/financeapp"
BIN="$APP_DIR/financeapp"
DATA="$APP_DIR/data"
SERVICE="financeapp"
STATUS_URL="http://127.0.0.1:8080/api/status"
TMP=""
cleanup() { if [[ -n "$TMP" ]]; then rm -rf "$TMP"; fi; }
trap cleanup EXIT

GN=$'\e[32m'; RD=$'\e[31m'; YW=$'\e[33m'; CL=$'\e[0m'
info() { echo -e "${YW}»${CL} $*"; }
ok() { echo -e "${GN}✔${CL} $*"; }
die() { echo -e "${RD}✘ $*${CL}" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "Esegui come root."

APP_ONLY=0
for arg in "$@"; do
  [[ "$arg" == "--app" ]] && APP_ONLY=1
done

# ripara le installazioni vecchie: `update` deve essere nel PATH minimo di Proxmox (pct enter / pct exec)
if [[ -x /usr/local/bin/update && ! -e /usr/bin/update ]]; then ln -sf /usr/local/bin/update /usr/bin/update; fi

case "$(dpkg --print-architecture)" in
  amd64) ARCH=amd64 ;;
  arm64) ARCH=arm64 ;;
  *) die "Architettura non supportata: $(dpkg --print-architecture)" ;;
esac

update_app() {
  info "Cerco l'ultima release di $FA_REPO..."
  local tag
  tag="$(curl -fsSL "https://api.github.com/repos/$FA_REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1 || true)"
  [[ -n "$tag" ]] || die "Nessuna release trovata su https://github.com/$FA_REPO/releases"

  local current
  current="$(cat "$APP_DIR/version" 2>/dev/null || echo none)"
  if [[ "$current" == "$tag" && -x "$BIN" && -z "${FORCE:-}" ]]; then
    ok "Finance App è già aggiornata ($tag)."
    return 0
  fi
  info "Versione installata: $current -> disponibile: $tag"

  local base asset expected actual
  TMP="$(mktemp -d)"
  base="https://github.com/$FA_REPO/releases/download/$tag"
  asset="financeapp-linux-$ARCH"

  info "Scarico $asset..."
  curl -fsSL -o "$TMP/financeapp" "$base/$asset" || die "Download non riuscito: $base/$asset"
  curl -fsSL -o "$TMP/checksums.txt" "$base/checksums.txt" || die "Download di checksums.txt non riuscito"
  expected="$(awk -v f="$asset" '$2==f {print $1}' "$TMP/checksums.txt")"
  [[ -n "$expected" ]] || die "Checksum per $asset non presente in checksums.txt"
  actual="$(sha256sum "$TMP/financeapp" | awk '{print $1}')"
  [[ "$expected" == "$actual" ]] || die "Checksum non valido: il file scaricato è corrotto."
  ok "Checksum verificato"

  local had_bin=0
  if [[ -x "$BIN" ]]; then
    had_bin=1
    info "Fermo il servizio e salvo i dati..."
    systemctl stop "$SERVICE" 2>/dev/null || true
    if [[ -f "$DATA/finance.db" ]]; then
      mkdir -p "$DATA/backups"
      cp "$DATA/finance.db" "$DATA/backups/finance-$current-$(date +%F-%H%M%S).db"
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

  local healthy=1 i
  for i in 1 2 3 4 5 6 7 8; do
    sleep 1
    if curl -fsS -o /dev/null "$STATUS_URL" 2>/dev/null; then healthy=0; break; fi
  done

  if [[ $healthy -eq 0 ]]; then
    echo "$tag" >"$APP_DIR/version"
    rm -f "$BIN.old"
    ok "Finance App $tag attiva su http://$(hostname -I | awk '{print $1}'):8080"
  else
    echo -e "${RD}Il servizio non risponde dopo l'aggiornamento.${CL}" >&2
    journalctl -u "$SERVICE" -n 15 --no-pager >&2 || true
    if [[ $had_bin -eq 1 && -f "$BIN.old" ]]; then
      info "Ripristino la versione precedente ($current)..."
      mv -f "$BIN.old" "$BIN"
      systemctl restart "$SERVICE" || true
      die "Aggiornamento annullato: ripristinata la versione $current."
    fi
    die "Installazione non riuscita."
  fi
}

ensure_auto_updates() {
  [[ "$AUTO_UPDATES" == "yes" ]] || return 0
  if ! dpkg -s unattended-upgrades >/dev/null 2>&1; then
    apt-get install -y -qq unattended-upgrades >/dev/null
  fi
  if [[ ! -f /etc/apt/apt.conf.d/20auto-upgrades ]]; then
    cat >/etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF
    ok "Aggiornamenti di sicurezza automatici attivi"
  fi
}

update_system() {
  info "Aggiorno i pacchetti di sistema..."
  export DEBIAN_FRONTEND=noninteractive
  if apt-get update -qq && apt-get -y -qq -o Dpkg::Options::=--force-confold upgrade &&
    apt-get -y -qq autoremove >/dev/null && apt-get -qq clean && ensure_auto_updates; then
    ok "Sistema aggiornato"
  else
    echo -e "${YW}Aggiornamento del sistema non completato (l'app non è stata toccata). Riprova più tardi.${CL}" >&2
  fi
  if [[ -f /var/run/reboot-required ]]; then
    info "Alcuni aggiornamenti richiedono un riavvio del container (sull'host: pct reboot <CTID>)."
  fi
}

update_app
if [[ $APP_ONLY -eq 0 ]]; then
  update_system
fi
