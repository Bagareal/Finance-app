#!/usr/bin/env bash
# Finance App - script per Proxmox VE (da lanciare sull'HOST, come root).
# Crea un container LXC Debian 13 (ripiego su 12) e installa l'app.
#
#   Installa:   bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Finance-app/main/ct/financeapp.sh)"
#   Aggiorna:   bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Finance-app/main/ct/financeapp.sh)" -- update [CTID]
#   (oppure, dentro il container:  update)
#
# Non usa build.func di community-scripts: quello scarica lo script di installazione da un URL fisso
# del repo community-scripts, quindi non può installare app esterne. Questo script fa lo stesso lavoro
# con i comandi nativi di Proxmox (pveam / pct).
set -Eeuo pipefail

FA_REPO="${FA_REPO:-Bagareal/Finance-app}"
FA_BRANCH="${FA_BRANCH:-main}"
RAW="https://raw.githubusercontent.com/$FA_REPO/$FA_BRANCH"
APP="Finance App"

# valori predefiniti (sovrascrivibili con variabili d'ambiente: var_cpu=2 var_ram=512 ...)
var_ctid="${var_ctid:-}"
var_hostname="${var_hostname:-financeapp}"
var_cpu="${var_cpu:-1}"
var_ram="${var_ram:-256}"
var_disk="${var_disk:-2}"
var_bridge="${var_bridge:-vmbr0}"
var_ip="${var_ip:-dhcp}"
var_gateway="${var_gateway:-}"
var_storage="${var_storage:-}"
var_template_storage="${var_template_storage:-}"
var_dns="${var_dns:-}"                 # vuoto = usa quello dell'host
var_searchdomain="${var_searchdomain:-}"
var_password="${var_password:-}"       # vuoto = nessuna password di root (si entra con pct enter)
var_ssh="${var_ssh:-no}"               # yes = installa sshd e permette il login root con password
var_vlan="${var_vlan:-}"
var_mtu="${var_mtu:-}"
var_auto_updates="${var_auto_updates:-yes}"   # automatic Debian security updates (unattended-upgrades)

GN=$'\e[32m'; RD=$'\e[31m'; YW=$'\e[33m'; BL=$'\e[36m'; CL=$'\e[0m'
msg_info() { echo -e "${YW}»${CL} $*"; }
msg_ok() { echo -e "${GN}✔${CL} $*"; }
msg_error() { echo -e "${RD}✘ $*${CL}" >&2; }
die() { msg_error "$*"; exit 1; }
trap 'msg_error "Errore alla riga $LINENO. Controlla l'\''output qui sopra."' ERR

header() {
  clear 2>/dev/null || true
  echo -e "${BL}"
  cat <<'EOF'
  _____ _                                  _
 |  ___(_)_ __   __ _ _ __   ___ ___      / \   _ __  _ __
 | |_  | | '_ \ / _` | '_ \ / __/ _ \    / _ \ | '_ \| '_ \
 |  _| | | | | | (_| | | | | (_|  __/   / ___ \| |_) | |_) |
 |_|   |_|_| |_|\__,_|_| |_|\___\___|  /_/   \_\ .__/| .__/
                                               |_|   |_|
EOF
  echo -e "${CL}"
}

check_env() {
  [[ $EUID -eq 0 ]] || die "Lancia lo script come root sull'host Proxmox."
  command -v pct >/dev/null 2>&1 && command -v pveam >/dev/null 2>&1 || die "Questo script va eseguito su un host Proxmox VE (pct/pveam non trovati)."
  command -v curl >/dev/null 2>&1 || die "curl non trovato."
}

# ---------------------------------------------------------------- update

do_update() {
  local ctid="${1:-}"
  if [[ -z "$ctid" ]]; then
    local found=()
    while read -r id status _; do
      [[ "$status" == "running" ]] || continue
      if pct exec "$id" -- test -x /opt/financeapp/financeapp 2>/dev/null; then found+=("$id"); fi
    done < <(pct list | awk 'NR>1')
    if [[ ${#found[@]} -eq 0 ]]; then
      die "Nessun container Finance App in esecuzione. Indica il CTID: ... -- update 105"
    elif [[ ${#found[@]} -eq 1 ]]; then
      ctid="${found[0]}"
    else
      echo "Container Finance App trovati: ${found[*]}"
      read -r -p "CTID da aggiornare: " ctid
    fi
  fi
  pct status "$ctid" 2>/dev/null | grep -q running || die "Il container $ctid non esiste o non è avviato."
  pct exec "$ctid" -- test -x /usr/local/bin/update || die "Il container $ctid non sembra un'installazione di $APP."
  msg_info "Aggiorno $APP e il sistema nel container $ctid..."
  pct exec "$ctid" -- /usr/local/bin/update "${@:2}"
}

# ---------------------------------------------------------------- install

ask() { # ask "Domanda" "predefinito" -> stampa la risposta
  local ans
  if command -v whiptail >/dev/null 2>&1 && [[ -t 0 ]]; then
    ans="$(whiptail --title "$APP" --inputbox "$1" 9 62 "$2" 3>&1 1>&2 2>&3)" || exit 1
  else
    read -r -p "$1 [$2]: " ans
  fi
  echo "${ans:-$2}"
}

ask_secret() {
  local ans
  if command -v whiptail >/dev/null 2>&1 && [[ -t 0 ]]; then
    ans="$(whiptail --title "$APP" --passwordbox "$1" 9 62 3>&1 1>&2 2>&3)" || exit 1
  else
    read -r -s -p "$1: " ans
    echo >&2
  fi
  echo "$ans"
}

ask_yesno() { # ask_yesno "Domanda" [yes|no (predefinito, default: no)] -> yes|no
  local def="${2:-no}"
  if command -v whiptail >/dev/null 2>&1 && [[ -t 0 ]]; then
    local flag=()
    [[ "$def" == "no" ]] && flag=(--defaultno)
    if whiptail --title "$APP" --yesno "$1" 9 62 "${flag[@]}"; then echo yes; else echo no; fi
  else
    local a
    if [[ "$def" == "yes" ]]; then read -r -p "$1 [S/n]: " a; [[ ! "$a" =~ ^[nN] ]] && echo yes || echo no
    else read -r -p "$1 [s/N]: " a; [[ "$a" =~ ^[sSyY] ]] && echo yes || echo no; fi
  fi
}

choose_storage() { # choose_storage <content> <variabile preferita> -> stampa lo storage scelto
  local content="$1" pref="$2" list
  list="$(pvesm status -content "$content" 2>/dev/null | awk 'NR>1 && $3=="active" {print $1}')"
  if [[ -n "$pref" || $(wc -l <<<"$list") -le 1 || ! -t 0 ]] || ! command -v whiptail >/dev/null 2>&1; then
    echo "$pref"
    return
  fi
  local items=() st
  while read -r st; do items+=("$st" ""); done <<<"$list"
  whiptail --title "$APP" --menu "Storage per '$content'" 16 62 6 "${items[@]}" 3>&1 1>&2 2>&3 || exit 1
}

choose_settings() {
  local mode="default"
  if [[ -t 0 && -z "${var_yes:-}" ]]; then
    if command -v whiptail >/dev/null 2>&1; then
      mode="$(whiptail --title "$APP" --menu "Come vuoi installare?" 12 62 2 \
        default "Impostazioni predefinite (consigliato)" \
        advanced "Impostazioni avanzate" 3>&1 1>&2 2>&3)" || exit 1
    else
      read -r -p "Impostazioni avanzate? [s/N]: " a
      [[ "$a" =~ ^[sSyY] ]] && mode=advanced
    fi
  fi
  if [[ "$mode" == "advanced" ]]; then
    var_ctid="$(ask "ID del container (vuoto = automatico)" "${var_ctid:-}")"
    var_hostname="$(ask "Hostname" "$var_hostname")"
    var_cpu="$(ask "Core CPU" "$var_cpu")"
    var_ram="$(ask "RAM in MB" "$var_ram")"
    var_disk="$(ask "Disco in GB" "$var_disk")"
    var_bridge="$(ask "Bridge di rete" "$var_bridge")"
    var_ip="$(ask "IP (dhcp oppure 192.168.1.50/24)" "$var_ip")"
    if [[ "$var_ip" != "dhcp" ]]; then
      var_gateway="$(ask "Gateway" "${var_gateway:-}")"
    fi
    var_dns="$(ask "Server DNS (vuoto = quello dell'host)" "${var_dns:-}")"
    var_searchdomain="$(ask "Dominio di ricerca DNS (vuoto = quello dell'host)" "${var_searchdomain:-}")"
    var_vlan="$(ask "VLAN tag (vuoto = nessuna)" "${var_vlan:-}")"
    var_mtu="$(ask "MTU (vuoto = predefinito)" "${var_mtu:-}")"
    var_storage="$(choose_storage rootdir "$var_storage")"
    var_auto_updates="$(ask_yesno "Installare automaticamente gli aggiornamenti di sicurezza di Debian?" yes)"
    var_password="$(ask_secret "Password di root del container (vuoto = nessuna, si entra con pct enter)")"
    if [[ -n "$var_password" ]]; then
      var_ssh="$(ask_yesno "Abilitare SSH con login root tramite password?")"
    fi
  fi
  [[ "$var_cpu" =~ ^[0-9]+$ && "$var_ram" =~ ^[0-9]+$ && "$var_disk" =~ ^[0-9]+$ ]] || die "CPU, RAM e disco devono essere numeri."
  [[ "$var_hostname" =~ ^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$ ]] || die "Hostname non valido: $var_hostname"
  if [[ "$var_ip" != "dhcp" && -z "$var_gateway" ]]; then die "Con IP statico serve anche il gateway."; fi
  [[ -z "$var_vlan" || "$var_vlan" =~ ^[0-9]+$ ]] || die "La VLAN deve essere un numero."
  [[ -z "$var_mtu" || "$var_mtu" =~ ^[0-9]+$ ]] || die "L'MTU deve essere un numero."
  [[ -z "$var_password" || ${#var_password} -ge 5 ]] || die "La password di root deve avere almeno 5 caratteri."
}

pick_storage() { # $1 = content (rootdir|vztmpl), $2 = preferenza
  local pref="$2" list
  list="$(pvesm status -content "$1" 2>/dev/null | awk 'NR>1 && $3=="active" {print $1}')"
  [[ -n "$list" ]] || die "Nessuno storage con contenuto '$1' disponibile."
  if [[ -n "$pref" ]]; then
    grep -qx "$pref" <<<"$list" || die "Storage '$pref' non valido per '$1'. Disponibili: $(echo "$list" | tr '\n' ' ')"
    echo "$pref"
  elif grep -qx "local-lvm" <<<"$list" && [[ "$1" == "rootdir" ]]; then
    echo "local-lvm"
  elif grep -qx "local" <<<"$list" && [[ "$1" == "vztmpl" ]]; then
    echo "local"
  else
    head -n1 <<<"$list"
  fi
}

do_install() {
  choose_settings

  local ctid="${var_ctid:-$(pvesh get /cluster/nextid)}"
  pct status "$ctid" >/dev/null 2>&1 && die "Il container $ctid esiste già."

  local storage tstorage
  storage="$(pick_storage rootdir "$var_storage")"
  tstorage="$(pick_storage vztmpl "$var_template_storage")"

  msg_info "Cerco il template Debian 13..."
  pveam update >/dev/null 2>&1 || true
  local template available
  available="$(pveam available --section system | awk '{print $2}')"
  template="$(grep -E '^debian-13-standard' <<<"$available" | sort -V | tail -n1 || true)"
  if [[ -z "$template" ]]; then
    msg_info "Debian 13 non disponibile su questo Proxmox (serve Proxmox VE 9): uso Debian 12."
    template="$(grep -E '^debian-12-standard' <<<"$available" | sort -V | tail -n1 || true)"
  fi
  [[ -n "$template" ]] || die "Nessun template Debian trovato (pveam available)."
  local have
  have="$(pveam list "$tstorage" | awk '{print $1}')"
  if ! grep -q "/$template\$" <<<"$have"; then
    msg_info "Scarico $template su $tstorage..."
    pveam download "$tstorage" "$template" >/dev/null
  fi
  msg_ok "Template pronto: $template"

  local net="name=eth0,bridge=$var_bridge,ip=$var_ip"
  [[ "$var_ip" != "dhcp" ]] && net+=",gw=$var_gateway"
  [[ -n "$var_vlan" ]] && net+=",tag=$var_vlan"
  [[ -n "$var_mtu" ]] && net+=",mtu=$var_mtu"
  local extra=()
  [[ -n "$var_dns" ]] && extra+=(--nameserver "$var_dns")
  [[ -n "$var_searchdomain" ]] && extra+=(--searchdomain "$var_searchdomain")
  [[ -n "$var_password" ]] && extra+=(--password "$var_password")

  msg_info "Creo il container $ctid ($var_hostname: ${var_cpu} CPU, ${var_ram} MB RAM, ${var_disk} GB su $storage)..."
  pct create "$ctid" "$tstorage:vztmpl/$template" \
    --hostname "$var_hostname" --cores "$var_cpu" --memory "$var_ram" --swap 256 \
    --rootfs "$storage:$var_disk" --net0 "$net" \
    --unprivileged 1 --features nesting=1 --onboot 1 --ostype debian --tags finance "${extra[@]}" >/dev/null
  msg_ok "Container creato"

  msg_info "Avvio il container..."
  pct start "$ctid"
  local i
  for i in $(seq 1 30); do
    pct exec "$ctid" -- getent hosts github.com >/dev/null 2>&1 && break
    sleep 2
    [[ $i -eq 30 ]] && die "Il container non ha accesso a internet (controlla bridge, IP e gateway)."
  done
  msg_ok "Rete attiva"

  local tz=""
  tz="$(cat /etc/timezone 2>/dev/null || timedatectl show -p Timezone --value 2>/dev/null || true)"

  if [[ "$var_ssh" == "yes" && -n "$var_password" ]]; then
    msg_info "Abilito SSH nel container..."
    pct exec "$ctid" -- bash -c 'export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq openssh-server >/dev/null && sed -i "s/^#\?PermitRootLogin.*/PermitRootLogin yes/" /etc/ssh/sshd_config && systemctl enable --now ssh >/dev/null 2>&1'
    msg_ok "SSH attivo"
  fi

  msg_info "Installo $APP nel container (può richiedere un minuto)..."
  local script
  script="$(curl -fsSL "$RAW/install/financeapp-install.sh")" || die "Impossibile scaricare lo script di installazione da $RAW"
  pct exec "$ctid" -- env FA_REPO="$FA_REPO" FA_BRANCH="$FA_BRANCH" TZ_NAME="$tz" AUTO_UPDATES="$var_auto_updates" bash -c "$script"

  local ip=""
  for i in $(seq 1 15); do
    ip="$(pct exec "$ctid" -- hostname -I 2>/dev/null | awk '{print $1}')"
    [[ -n "$ip" ]] && break
    sleep 1
  done

  echo
  msg_ok "$APP installata con successo!"
  echo -e "  Apri:      ${GN}http://${ip:-<ip-del-container>}:8080${CL}"
  echo -e "  Container: $ctid ($var_hostname)   Console: pct enter $ctid"
  echo -e "  Aggiorna:  dentro il container  ${BL}update${CL}   oppure dall'host  ${BL}bash -c \"\$(curl -fsSL $RAW/ct/financeapp.sh)\" -- update $ctid${CL}"
  echo -e "  Al primo accesso scegli la password."
}

# ---------------------------------------------------------------- main

case "${1:-${FA_ACTION:-install}}" in
  update) check_env; do_update "${2:-}" ;;
  install) header; check_env; do_install ;;
  -h | --help | help) sed -n '2,8p' "$0" 2>/dev/null || echo "Uso: financeapp.sh [install|update [CTID]]" ;;
  *) die "Azione sconosciuta: ${1:-}. Usa: install | update [CTID]" ;;
esac
