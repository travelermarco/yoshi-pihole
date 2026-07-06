# 🐉 Yoshi Pi-hole

Un blocco pubblicità/tracker **locale**, ispirato a [Pi-hole](https://pi-hole.net/), che gira interamente su un singolo Mac — senza Raspberry Pi, senza un dispositivo di rete dedicato. Un solo binario Go fa da sinkhole DNS, API REST e dashboard web; un'icona nella barra di stato mostra lo stato del blocco.

Non affiliato al progetto Pi-hole: è un progetto personale scritto da zero, ispirato alla sua architettura e alle sue funzionalità.

## Come funziona

Il Mac diventa il proprio resolver DNS: le richieste verso domini pubblicitari/traccianti vengono bloccate localmente (risposta nulla o NXDOMAIN), tutte le altre vengono inoltrate normalmente a un upstream (Cloudflare/Quad9). Questo blocca gli annunci a livello di sistema operativo — browser, app, tutto — non solo nel browser.

## Funzionalità

- **Motore DNS** (`miekg/dns`) con blocklist in formato hosts, plain-domain e Adblock Plus (`||dominio^`)
- **Blocklist di default**: [StevenBlack hosts](https://github.com/StevenBlack/hosts), aggiornabile dalla dashboard
- **Whitelist/blacklist manuale**, esatta o **regex** (con le estensioni in stile Pi-hole: `;querytype=`, `;invert`, `;reply=`)
- **Gruppi e client**, log query in tempo reale, statistiche
- **Bedtime mode**: disabilitazione del blocco a tempo (30s/5m/30m o indefinita)
- **API REST** locale (nessuna autenticazione: è pensato per un solo utente sulla propria macchina, in ascolto solo su `127.0.0.1`)
- **Dashboard web** integrata nel binario (nessun Node/build step)
- **Icona nella barra di stato** con stato del blocco e controlli rapidi, avvio automatico ad ogni login

## Architettura

```
cmd/yoshi-pihole/     binario principale: motore DNS + API + dashboard
cmd/yoshi-menubar/    icona nella barra di stato (processo separato, sessione utente)
internal/dns/         server DNS (miekg/dns), forwarding upstream
internal/gravity/     fetch e parsing delle blocklist, estensioni regex
internal/matcher/     motore di matching in memoria (allow/deny, esatto/regex)
internal/db/          due SQLite: gravity.db (liste/gruppi/client) e queries.db (log)
internal/api/         API REST
internal/service/     bedtime mode (disabilitazione a tempo)
web/                  dashboard statica (HTML/CSS/JS), incorporata via go:embed
scripts/              installazione/disinstallazione
```

Il demone DNS gira come **LaunchDaemon di sistema (root)**, necessario per usare la porta 53. L'icona nella barra di stato gira invece come **LaunchAgent utente**, perché un processo root non ha accesso alla sessione grafica.

## Requisiti

- macOS (sviluppato e testato su Apple Silicon)
- [Go](https://go.dev/dl/) 1.22 o superiore (solo per compilare)
- Xcode Command Line Tools (`xcode-select --install`) — servono a `cgo` per l'icona nella barra di stato

## Installazione

### 1. Motore DNS (richiede sudo)

```sh
sudo scripts/install.sh
```

Compila il binario, lo installa come LaunchDaemon root in ascolto su `127.0.0.1:53`, e reindirizza il DNS di sistema del Mac su `127.0.0.1` (con backup automatico delle impostazioni precedenti). Chiede conferma prima di modificare qualsiasi impostazione.

### 2. Icona nella barra di stato (facoltativa, NON richiede sudo)

```sh
scripts/install-menubar.sh
```

### Disinstallazione

```sh
sudo scripts/uninstall.sh                                    # demone DNS + ripristino DNS di sistema
launchctl bootout gui/$(id -u)/com.yoshi.pihole.menubar       # icona barra di stato
```

## Uso

- **Dashboard**: <http://127.0.0.1:8080> — statistiche, log query, gestione whitelist/blacklist/regex, blocklist, gruppi, client
- **Barra di stato**: clic sull'icona per aprire la dashboard o disabilitare il blocco a tempo
- **Sviluppo locale** (senza installare nulla come demone): `go run ./cmd/yoshi-pihole serve` — usa `config/config.yaml`, porta DNS `15353` per non entrare in conflitto con mDNSResponder/Bonjour (che occupa sempre la 5353 su macOS)

## Note

- **iCloud Private Relay** può ignorare un resolver DNS locale: se non vedi il blocco funzionare, disattivalo per la rete in uso.
- Una **VPN attiva** (es. WireGuard) può imporre il proprio DNS con precedenza: verifica con `scutil --dns`.
- Nessuna telemetria: tutto (log query incluso) resta sul disco locale, in `/usr/local/yoshi-pihole/data/`.

## Licenza

Da definire.
