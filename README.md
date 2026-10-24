# Finance App

A small, fast personal finance app you host yourself. Track accounts, income and expenses, with
categories, recurring transactions and monthly / yearly summaries, plus an optional section to follow the
earnings of a freelance activity. It ships as a single Go binary with an
embedded SQLite database (around 15 MB of RAM), works on desktop and mobile, and installs on
**Proxmox VE** with one command.

The interface is available in **English** (the default) and **Italian**, and amounts can be shown in
**EUR, USD, AUD, NZD or GBP**. Language and currency are app-wide settings (*Settings → Language and
currency*). The currency only sets the symbol and number format: nothing is converted.

## Features

- **Accounts**: several accounts (checking, cash, savings, card, investments) with an opening balance,
  archiving, and transfers between accounts. A transfer is neither income nor expense.
  Accounts can be **grouped** (for example a bank with a current account and a savings account): the group
  shows one line with the total and opens to show each account. To create a group, type its name in the
  account's *Group* field; the **+** on the group row then adds another account already inside it.
  An account can have its **value updated by hand** (for example investments): each update is saved with its
  date, the app shows the gain or loss and a small history chart, and transfers made after the last update
  are added to the value. Gains and losses are not counted as income or expenses.
  An account can be marked **Do not use for expenses**: it is then left out of the account list when you add an
  expense, but stays available for income (an interest, for example) and for transfers.
- **Categories**: two levels (category and subcategory), separate for income and expenses. Create, rename
  and delete them; deleting a category in use asks where to move its transactions.
- **Transactions**: add, edit or delete any transaction, including past ones. Filter by month, account,
  category and type.
- **Recurring transactions**: weekly, monthly or yearly income, expenses and transfers, with an optional end
  date. Missed occurrences are created when the server starts. Editing or deleting a transaction that came
  from a recurrence asks whether to change only that one, it and the following ones, or all of them.
  Editing or deleting the rule itself from the *Recurring* page asks whether to touch only the next ones or also
  the transactions already created.
- **Summary**: earned, spent and saved for any month or year, with a chart by category and a monthly trend.
- **Pro**: a separate section (in the side menu, or under *More* on a phone) for a freelance or small business
  activity. It is independent from accounts and transactions and changes none of their figures.
  In *Setup* create your clients and your own deduction categories (tax, contributions, VAT… named as your country
  needs: nothing is built in). Then record each payment with its date, client and gross amount, and split it between
  your categories. The net is gross minus the split, computed for you, and a payment can be edited or deleted at any
  time, including past ones. *Month* and *Year* show gross and net; tap a total, a payment or a month to see how much
  went to each category. Deleting a client or category that has payments only hides it from new payments: the old ones
  keep its name and totals. Pro data is not part of the CSV export, but it is in the database backup.
- **Privacy**: a button hides every balance.
- **Sign-in**: a single password, plus an optional PIN and biometric unlock (see below).
- **Data**: CSV import and export, and a database backup download.
- **Mobile friendly**: responsive layout, installable as a PWA, light and dark theme.

## Install on Proxmox VE

Run this on the Proxmox host, as root:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Finance-app/main/ct/financeapp.sh)"
```

The script creates an unprivileged LXC container (Debian 13, or Debian 12 if your Proxmox does not have the
Debian 13 template yet; 1 core, 256 MB RAM, 2 GB disk), installs the app as a systemd service and prints the
address, usually `http://<container-ip>:8080`. On first visit you choose the password.

The advanced option lets you change the container ID, hostname, CPU, RAM, disk, storage, bridge, IP and
gateway, DNS and search domain, VLAN, MTU, root password, SSH access, and whether Debian security updates
install automatically. Every setting can also be given as an environment variable, for example
`var_cpu=2 var_ram=512`.

## Update

Inside the container:

```bash
update            # updates the app, then the Debian packages
update --app      # updates only the app
```

From the Proxmox host:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Bagareal/Finance-app/main/ct/financeapp.sh)" -- update [CTID]
```

The app update downloads the latest release, verifies its SHA-256 checksum, backs up the database to
`/opt/financeapp/data/backups` (the last 5 are kept) and rolls back to the previous version if the new one does
not start. Automatic security updates (`unattended-upgrades`) are enabled by default.

## Sign-in, PIN and biometrics

The first sign-in on every device needs the password. After that, *Settings → Quick unlock* lets you turn on:

- a **PIN** of 4 to 6 digits, the same on all your devices;
- **biometric unlock** (fingerprint, Face ID, Windows Hello) through WebAuthn, on the current device.

The PIN and biometrics only work on devices that already signed in with the password. After 5 wrong PINs the
device is forgotten and the password is required again. The app locks itself after 5 minutes in the
background and on a cold start. Changing the password signs out every device and turns PIN and biometrics off
until you set them up again.

Biometric unlock needs a secure context, so open the app over **HTTPS**: a reverse proxy (Nginx Proxy
Manager, Caddy, ...), Tailscale (`tailscale serve`) or a domain with a certificate. Over plain
`http://<ip>:8080` only the PIN is available. The proxy must forward the `Host`, `X-Forwarded-Host` and
`X-Forwarded-Proto` headers.

## Data and configuration

- Database: `/opt/financeapp/data/finance.db`, one file. Copy it to back up, or use *Settings → Download backup*.
- Port: `/opt/financeapp/financeapp.env` (`FA_ADDR=:8080`), then `systemctl restart financeapp`.
- Logs: `journalctl -u financeapp`.
- The app does not serve HTTPS itself. Keep it on your LAN or behind a reverse proxy or VPN.

### CSV format

Columns: `date, type, amount, account, destination_account, category, subcategory, note`. The Italian names
(`data, tipo, importo, conto, conto_destinazione, categoria, sottocategoria, nota`) are accepted too, and
export uses the app language. `type` is `income`, `expense` or `transfer` (or `entrata`, `uscita`,
`trasferimento`); when it is empty, a negative amount is an expense. Dates are `YYYY-MM-DD`. Missing accounts
and categories are created. An account in a group is written as `Group · Name`. The whole file is validated
before anything is imported.

## Development

Requires Go 1.27 or newer. No Node or build step: the frontend is plain JavaScript with Preact and htm,
embedded in the binary.

```bash
go test ./...
FA_DATA=./data FA_ADDR=127.0.0.1:8080 go run ./cmd/financeapp
```

Layout:

```
cmd/financeapp/     entry point
internal/store/     SQLite schema and all business logic (with tests)
internal/server/    JSON API, authentication, PIN and WebAuthn (with tests)
web/                frontend, embedded into the binary
ct/financeapp.sh    Proxmox host script (create the container, update it)
install/            scripts that run inside the container
```

To publish a version, push a tag: `git tag v1.0.1 && git push origin v1.0.1`. GitHub Actions runs the tests,
builds the binaries (linux amd64 and arm64), and publishes the release with a `checksums.txt` file.

`ct/financeapp.sh` does not use the community-scripts `build.func`, because that downloads the installer
from a fixed community-scripts URL and so cannot install third-party apps. It uses the native `pveam` and
`pct` commands instead.
