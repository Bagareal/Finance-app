# Finance App

App web leggera per gestire conti, spese ed entrate. Un solo binario Go + SQLite (circa 15 MB di RAM),
interfaccia in italiano per desktop e mobile, installabile come PWA.

**Funzioni:** conti multipli e trasferimenti, macro-categorie e sottocategorie (crea/elimina),
entrate e spese ricorrenti, riepilogo mensile e annuale di guadagnato / speso / risparmiato con grafici,
import/export CSV, backup del database, accesso con una password.

## Installazione su Proxmox VE

Sull'host Proxmox, come root:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Finance-app/main/ct/financeapp.sh)"
```

Lo script crea un container LXC Debian 13 (1 core, 256 MB RAM, 2 GB disco; se il tuo Proxmox non ha
ancora il template Debian 13 usa il 12), installa l'app come servizio systemd e mostra l'indirizzo, di
solito `http://IP:8080`. Al primo accesso scegli la password.

Le impostazioni avanzate permettono di cambiare ID, hostname, CPU, RAM, disco, storage, bridge, IP e
gateway, DNS e dominio di ricerca, VLAN, MTU, password di root e SSH.

## Accesso: password, PIN e biometria

Il primo accesso su ogni dispositivo richiede la password. Poi, da *Impostazioni → Sblocco rapido*, puoi
attivare un PIN (4-6 cifre, unico per tutti i dispositivi) e, se apri l'app in **HTTPS**, l'impronta o il
Face ID del telefono. Il PIN e la biometria funzionano solo sui dispositivi che hanno già fatto il login con
la password; dopo 5 PIN sbagliati il dispositivo viene dimenticato e serve di nuovo la password.
L'app si blocca da sola dopo 5 minuti in background e a ogni riapertura a freddo.
Cambiare la password revoca tutti i dispositivi e va ripetuta l'attivazione di PIN e biometria.

La biometria (WebAuthn) richiede un contesto sicuro: apri l'app tramite un reverse proxy con HTTPS
(Nginx Proxy Manager, Caddy…), Tailscale (`tailscale serve`) o un dominio con certificato. Con `http://IP:8080`
resta disponibile solo il PIN. Il reverse proxy deve passare gli header `Host`, `X-Forwarded-Host` e
`X-Forwarded-Proto`.

## Aggiornamento

Dentro il container:

```bash
update
```

Oppure dall'host Proxmox:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Finance-app/main/ct/financeapp.sh)" -- update [CTID]
```

L'aggiornamento scarica l'ultima release, verifica il checksum, salva una copia del database in
`/opt/financeapp/data/backups` (ne tiene 5) e ripristina la versione precedente se la nuova non parte.

## Dati e configurazione

- Database: `/opt/financeapp/data/finance.db` (un solo file: per il backup basta copiarlo, oppure
  usa *Impostazioni → Scarica backup*).
- Porta: `/opt/financeapp/financeapp.env` (`FA_ADDR=:8080`), poi `systemctl restart financeapp`.
- Log: `journalctl -u financeapp`.
- L'app non usa HTTPS: tienila in LAN o dietro un reverse proxy / VPN.

## Sviluppo

```bash
go test ./...
FA_DATA=./data FA_ADDR=127.0.0.1:8080 go run ./cmd/financeapp
```

Per pubblicare una versione: `git tag v1.0.0 && git push origin v1.0.0`. GitHub Actions compila i binari
(linux amd64 e arm64) e crea la release.

Nota: lo script `ct/financeapp.sh` non usa `build.func` di community-scripts perché quello scarica lo
script di installazione da un URL fisso del repo community-scripts; usa i comandi nativi `pveam`/`pct`.
