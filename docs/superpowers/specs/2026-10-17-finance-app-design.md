# Finance App — Specifica di design

Repo: https://github.com/Bagareal/Finance-app (vuota al 2026-10-17)

## Obiettivo
App web di finanza personale, leggera e veloce, per un solo utente, installabile su Proxmox
come LXC con uno script in stile community-scripts, con comando di update. Alternativa semplice a EZBookkeeping.
Usabile da desktop e da mobile (responsive, PWA). Interfaccia in italiano, solo EUR.

## Funzioni
- Conti multipli (corrente, contanti, risparmio, carta), saldo iniziale, archiviazione.
- Trasferimenti tra conti (non contano come spesa né come entrata).
- Macro-categorie e sottocategorie (due livelli): crea, rinomina, elimina. Se una categoria ha movimenti,
  l'eliminazione richiede di scegliere una categoria di destinazione per spostarli.
- Movimenti: entrata, uscita, trasferimento; filtro per conto, categoria, periodo; modifica, elimina.
- Ricorrenze (entrate e uscite): frequenza settimanale, mensile, annuale; data inizio, fine opzionale; pausa.
  Un job all'avvio e poi ogni ora genera i movimenti scaduti, compresi gli arretrati se il server era spento.
- Riepilogo mese e anno: guadagnato, speso, risparmiato (= entrate − uscite); grafico a torta per categoria
  e barre per mese, in SVG senza librerie.
- Import/export CSV; download del backup (file SQLite).
- Login con una sola password; prima apertura = scelta della password.
- PWA installabile (manifest + service worker minimale).

Fuori scopo (YAGNI): multi-utente, multi-valuta, budget, riconciliazione bancaria, tag.

## Architettura
- Un binario Go; API JSON più frontend statico incorporato con `go:embed`.
- Frontend: Preact + htm caricati come file locali, nessun build step, nessun npm.
- DB: SQLite (`modernc.org/sqlite`, senza CGO), file in `/opt/financeapp/data/finance.db`.
- Importi in centesimi (interi). Il saldo dei conti si calcola dai movimenti, non è salvato.
- Un trasferimento = due righe collegate (uscita dal conto A, entrata sul conto B) marcate come trasferimento.
- Sessione: cookie HttpOnly, SameSite=Lax; password con hash bcrypt/argon2.
- Mobile first: barra in basso e pulsante "+" su telefono, barra laterale su desktop; tema chiaro/scuro.
- Porta 8080, configurabile con variabile d'ambiente.

### Schermate
Home (mese corrente + saldi + ultimi movimenti), Movimenti, Riepilogo, Conti, Categorie, Ricorrenti, Impostazioni.

## Distribuzione e installazione
- `.github/workflows/release.yml`: al push di un tag `v*` compila `financeapp-linux-amd64` e `financeapp-linux-arm64`
  e li pubblica nella release GitHub di Bagareal/Finance-app.
- `ct/financeapp.sh`: gira sull'host Proxmox. Usa `build.func` di community-scripts per creare l'LXC
  (Debian 12, non privilegiato, 1 core, 256 MB RAM, 2 GB disco di default). Contiene anche la funzione `update_script`.
- `install/financeapp-install.sh`: gira nel container. Scarica l'ultimo binario dalla release,
  crea `/opt/financeapp`, l'utente di servizio e `financeapp.service` (systemd), salva la versione in `/opt/financeapp/version`.
- Comando di update: dentro il container, `update` (rimanda a `update_script`) confronta la versione installata con
  l'ultima release, ferma il servizio, copia un backup del DB, sostituisce il binario, riavvia.
  Dal host Proxmox si può rilanciare `ct/financeapp.sh` scegliendo update.
- Gli URL puntano a `raw.githubusercontent.com/Bagareal/Finance-app/main/...` (configurabile in testa agli script).
- Limite noto: non esiste un Proxmox su cui provare gli script; li verificherà l'utente. L'app e l'API
  si provano in locale su Windows.

## Test
- Test Go per: calcolo saldi, trasferimenti, generazione ricorrenze (incluso recupero arretrati), eliminazione categoria con spostamento, import/export CSV.
- Prova reale del server con richieste all'API; controllo del frontend nel browser.
- `shellcheck`/`bash -n` sugli script di installazione.

## Struttura repo
```
cmd/financeapp/main.go
internal/{db,api,auth,recurring,csvio}/
web/            (index.html, app.js, style.css, manifest, sw.js — incorporati)
ct/financeapp.sh
install/financeapp-install.sh
.github/workflows/release.yml
docs/superpowers/specs/
README.md
```
