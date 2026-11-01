#!/usr/bin/env bash
# Finance App - installazione DENTRO il container Debian.
# Lanciato da ct/financeapp.sh tramite `pct exec`.
# Variabili opzionali: FA_REPO, FA_BRANCH, TZ_NAME, AUTO_UPDATES (yes|no).
set -Eeuo pipefail

FA_REPO="${FA_REPO:-Bagareal/Finance-app}"
FA_BRANCH="${FA_BRANCH:-main}"
AUTO_UPDATES="${AUTO_UPDATES:-yes}"
RAW="https://raw.githubusercontent.com/$FA_REPO/$FA_BRANCH"
APP_DIR="/opt/financeapp"

GN=$'\e[32m'; RD=$'\e[31m'; YW=$'\e[33m'; CL=$'\e[0m'
info() { echo -e "${YW}»${CL} $*"; }
ok() { echo -e "${GN}✔${CL} $*"; }
trap 'echo -e "${RD}✘ Installazione fallita alla riga $LINENO${CL}" >&2' ERR

export DEBIAN_FRONTEND=noninteractive

info "Installo le dipendenze..."
apt-get update -qq
apt-get install -y -qq curl ca-certificates tzdata >/dev/null
ok "Dipendenze installate"

if [[ -n "${TZ_NAME:-}" && -f "/usr/share/zoneinfo/$TZ_NAME" ]]; then
  ln -sf "/usr/share/zoneinfo/$TZ_NAME" /etc/localtime
  echo "$TZ_NAME" >/etc/timezone
  ok "Fuso orario: $TZ_NAME"
fi

info "Creo utente e cartelle..."
id financeapp >/dev/null 2>&1 || useradd --system --home-dir "$APP_DIR" --shell /usr/sbin/nologin financeapp
mkdir -p "$APP_DIR/data"
chown -R financeapp:financeapp "$APP_DIR"

# configurazione modificabile: porta e indirizzo di ascolto
if [[ ! -f "$APP_DIR/financeapp.env" ]]; then
  cat >"$APP_DIR/financeapp.env" <<'EOF'
# Indirizzo di ascolto (es. :8080 oppure 127.0.0.1:8080 dietro a un reverse proxy)
FA_ADDR=:8080
EOF
fi

cat >/etc/financeapp.conf <<EOF
FA_REPO=$FA_REPO
FA_BRANCH=$FA_BRANCH
AUTO_UPDATES=$AUTO_UPDATES
EOF

cat >/etc/systemd/system/financeapp.service <<'EOF'
[Unit]
Description=Finance App
After=network.target

[Service]
Type=simple
User=financeapp
Group=financeapp
WorkingDirectory=/opt/financeapp
Environment=FA_DATA=/opt/financeapp/data
EnvironmentFile=-/opt/financeapp/financeapp.env
ExecStart=/opt/financeapp/financeapp
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/opt/financeapp/data

[Install]
WantedBy=multi-user.target
EOF

# comando `update`: scarica ogni volta la logica di aggiornamento più recente
cat >/usr/local/bin/update <<'EOF'
#!/usr/bin/env bash
[[ -f /etc/financeapp.conf ]] && source /etc/financeapp.conf
export FA_REPO="${FA_REPO:-Bagareal/Finance-app}"
export AUTO_UPDATES="${AUTO_UPDATES:-yes}"
FA_BRANCH="${FA_BRANCH:-main}"
script="$(curl -fsSL "https://raw.githubusercontent.com/$FA_REPO/$FA_BRANCH/install/financeapp-update.sh")" || { echo "Impossibile scaricare lo script di aggiornamento" >&2; exit 1; }
exec bash -c "$script" update "$@"
EOF
chmod +x /usr/local/bin/update
# la console di Proxmox (pct enter / pct exec) ha un PATH senza /usr/local/bin
ln -sf /usr/local/bin/update /usr/bin/update
ok "Servizio e comando 'update' configurati"

info "Scarico e avvio Finance App..."
script="$(curl -fsSL "$RAW/install/financeapp-update.sh")"
FA_REPO="$FA_REPO" AUTO_UPDATES="$AUTO_UPDATES" bash -c "$script" install

ok "Installazione completata"
