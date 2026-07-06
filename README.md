# 🐉 Yoshi Pi-hole

A **local** ad/tracker blocker, inspired by [Pi-hole](https://pi-hole.net/), that runs entirely on a single Mac — no Raspberry Pi, no dedicated network device. A single Go binary handles the DNS sinkhole, the REST API, and the web dashboard; a status bar icon shows whether blocking is active.

Not affiliated with the Pi-hole project: this is a personal project written from scratch, inspired by its architecture and feature set.

## How it works

The Mac becomes its own DNS resolver: requests to ad/tracking domains are blocked locally (null response or NXDOMAIN), everything else is forwarded normally to an upstream resolver (Cloudflare/Quad9). This blocks ads at the operating-system level — browsers, apps, everything — not just in the browser.

## Features

- **DNS engine** (`miekg/dns`) supporting hosts-file, plain-domain, and Adblock Plus (`||domain^`) blocklist formats
- **Default blocklist**: [StevenBlack hosts](https://github.com/StevenBlack/hosts), updatable from the dashboard
- **Manual whitelist/blacklist**, exact or **regex** (with Pi-hole-style extensions: `;querytype=`, `;invert`, `;reply=`)
- **Groups and clients**, live query log, stats
- **Bedtime mode**: timed disable (30s/5m/30m, or indefinite)
- **Local REST API** (no authentication: single-user, single-machine tool, bound to `127.0.0.1` only)
- **Web dashboard** embedded in the binary (no Node/build step)
- **Status bar icon** showing blocking state with quick controls, auto-starts at login

## Architecture

```
cmd/yoshi-pihole/     main binary: DNS engine + API + dashboard
cmd/yoshi-menubar/    status bar icon (separate process, user session)
internal/dns/         DNS server (miekg/dns), upstream forwarding
internal/gravity/     blocklist fetching and parsing, regex extensions
internal/matcher/     in-memory matching engine (allow/deny, exact/regex)
internal/db/          two SQLite databases: gravity.db (lists/groups/clients) and queries.db (log)
internal/api/         REST API
internal/service/     bedtime mode (timed disable)
web/                  static dashboard (HTML/CSS/JS), embedded via go:embed
scripts/              install/uninstall
```

The DNS daemon runs as a **root system LaunchDaemon**, required to use port 53. The status bar icon runs as a **user LaunchAgent** instead, since a root process has no access to the GUI session.

## Requirements

- macOS (built and tested on Apple Silicon)
- [Go](https://go.dev/dl/) 1.22 or later (build-time only)
- Xcode Command Line Tools (`xcode-select --install`) — needed by `cgo` for the status bar icon

## Installation

### 1. DNS engine (requires sudo)

```sh
sudo scripts/install.sh
```

Builds the binary, installs it as a root LaunchDaemon listening on `127.0.0.1:53`, and redirects the Mac's system DNS to `127.0.0.1` (automatically backing up the previous settings). Asks for confirmation before changing anything.

### 2. Status bar icon (optional, no sudo needed)

```sh
scripts/install-menubar.sh
```

### Uninstall

```sh
sudo scripts/uninstall.sh                                    # DNS daemon + restore system DNS
launchctl bootout gui/$(id -u)/com.yoshi.pihole.menubar       # status bar icon
```

## Usage

- **Dashboard**: <http://127.0.0.1:8080> — stats, query log, whitelist/blacklist/regex management, blocklists, groups, clients
- **Status bar**: click the icon to open the dashboard or pause blocking for a while
- **Local development** (without installing anything as a daemon): `go run ./cmd/yoshi-pihole serve` — uses `config/config.yaml`, DNS port `15353` to avoid clashing with mDNSResponder/Bonjour (which always owns port 5353 on macOS)

## Notes

- **iCloud Private Relay** may ignore a local DNS resolver: if blocking doesn't seem to work, disable it for the network you're on.
- An **active VPN** (e.g. WireGuard) may push its own DNS with higher precedence: check with `scutil --dns`.
- No telemetry: everything (including the query log) stays on local disk, under `/usr/local/yoshi-pihole/data/`.

## License

[MIT](LICENSE)
