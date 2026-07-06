#!/usr/bin/env bash
# Installs Yoshi Pi-hole as a root LaunchDaemon bound to 127.0.0.1:53 and
# redirects this Mac's system DNS to it. Requires sudo. Never run
# automatically by an agent — this is an explicit, user-initiated step.
set -euo pipefail

PREFIX="/usr/local/yoshi-pihole"
PLIST="/Library/LaunchDaemons/com.yoshi.pihole.plist"
LABEL="com.yoshi.pihole"
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DNS_BACKUP="$PREFIX/data/dns-backup.json"
ASSUME_YES=0

for arg in "$@"; do
  case "$arg" in
    --yes|-y) ASSUME_YES=1 ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Questo script deve essere eseguito con sudo:"
  echo "  sudo scripts/install.sh"
  exit 1
fi

cat <<EOF
========================================================================
 Yoshi Pi-hole - installazione

 Questo script:
   1. Compila e installa il binario in $PREFIX
   2. Crea un LaunchDaemon di sistema (com.yoshi.pihole) eseguito come root,
      necessario per usare la porta 53
   3. Cambia il DNS di sistema di questo Mac su 127.0.0.1 (con backup delle
      impostazioni attuali, ripristinabili con scripts/uninstall.sh)
========================================================================
EOF

if [[ "$ASSUME_YES" -ne 1 ]]; then
  read -r -p "Procedere? [y/N] " reply
  [[ "$reply" =~ ^[Yy]$ ]] || { echo "Annullato."; exit 0; }
fi

echo "==> Creazione directory in $PREFIX"
mkdir -p "$PREFIX/bin" "$PREFIX/data" "$PREFIX/config" "$PREFIX/logs"

REAL_USER="${SUDO_USER:-$USER}"
GO_BIN=""
if command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
elif [[ -x "/Users/$REAL_USER/sdk/go/bin/go" ]]; then
  GO_BIN="/Users/$REAL_USER/sdk/go/bin/go"
else
  echo "Non trovo il toolchain Go (né nel PATH né in ~/sdk/go/bin)."
  echo "Esegui prima il bootstrap di Go, poi rilancia questo script."
  exit 1
fi

echo "==> Compilazione del binario (go build)"
# Build to a path the unprivileged user can write (their own repo checkout),
# then install it into the root-owned $PREFIX with the right owner/perms —
# building directly into $PREFIX as a dropped-privilege user would fail since
# $PREFIX was just created by root above.
BUILD_TMP="$REPO_DIR/.yoshi-pihole-build"
( cd "$REPO_DIR" && sudo -u "$REAL_USER" "$GO_BIN" build -o "$BUILD_TMP" ./cmd/yoshi-pihole )
install -m 755 "$BUILD_TMP" "$PREFIX/bin/yoshi-pihole"
rm -f "$BUILD_TMP"

if [[ ! -f "$PREFIX/config/config.yaml" ]]; then
  echo "==> Scrittura configurazione di produzione (porta 53)"
  sed \
    -e 's/^\( *listen: \)"127\.0\.0\.1:15353"/\1"127.0.0.1:53"/' \
    "$REPO_DIR/config/config.yaml" > "$PREFIX/config/config.yaml"
  # storage.data_dir must be absolute once installed system-wide.
  sed -i '' -e "s#^\( *data_dir: \).*#\1\"$PREFIX/data\"#" "$PREFIX/config/config.yaml"
else
  echo "==> Configurazione esistente in $PREFIX/config/config.yaml, non sovrascritta"
fi

echo "==> Scrittura LaunchDaemon $PLIST"
cat > "$PLIST" <<PLIST_EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>${LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${PREFIX}/bin/yoshi-pihole</string>
        <string>serve</string>
        <string>--config</string>
        <string>${PREFIX}/config/config.yaml</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>UserName</key><string>root</string>
    <key>StandardOutPath</key><string>${PREFIX}/logs/yoshi-pihole.log</string>
    <key>StandardErrorPath</key><string>${PREFIX}/logs/yoshi-pihole.err.log</string>
</dict>
</plist>
PLIST_EOF
chmod 644 "$PLIST"

echo "==> Avvio del demone"
launchctl bootout system "$PLIST" >/dev/null 2>&1 || true
launchctl bootstrap system "$PLIST"
launchctl kickstart -k "system/$LABEL"

echo "==> Verifica che il resolver risponda su 127.0.0.1:53..."
ok=0
for i in 1 2 3 4 5 6 7 8 9 10; do
  if dig @127.0.0.1 -p 53 +time=1 +tries=1 example.com >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 1
done

if [[ "$ok" -ne 1 ]]; then
  echo "ERRORE: il demone non risponde su 127.0.0.1:53. Controlla i log in $PREFIX/logs/."
  echo "Il DNS di sistema NON è stato modificato."
  exit 1
fi
echo "    OK: il resolver risponde."

echo "==> Backup delle impostazioni DNS attuali in $DNS_BACKUP"
mkdir -p "$(dirname "$DNS_BACKUP")"
echo "{" > "$DNS_BACKUP"
first=1
services="$(networksetup -listallnetworkservices | tail -n +2 | grep -v '^\*')"
while IFS= read -r service; do
  [[ -z "$service" ]] && continue
  current="$(networksetup -getdnsservers "$service" 2>/dev/null || true)"
  if [[ "$current" == *"aren't any"* || -z "$current" ]]; then
    current=""
  fi
  json_servers="$(echo "$current" | sed '/^$/d' | awk '{printf "\"%s\",", $0}' | sed 's/,$//')"
  [[ "$first" -eq 0 ]] && echo "," >> "$DNS_BACKUP"
  first=0
  printf '  "%s": [%s]' "$service" "$json_servers" >> "$DNS_BACKUP"
done <<< "$services"
echo "" >> "$DNS_BACKUP"
echo "}" >> "$DNS_BACKUP"

echo "==> Reindirizzamento DNS di sistema a 127.0.0.1"
while IFS= read -r service; do
  [[ -z "$service" ]] && continue
  echo "    - $service -> 127.0.0.1"
  networksetup -setdnsservers "$service" 127.0.0.1
done <<< "$services"

cat <<EOF

========================================================================
 Installazione completata.

 Dashboard: http://127.0.0.1:8080
 Log:       $PREFIX/logs/

 Nota bene:
  - Se usi iCloud Private Relay, potrebbe ignorare il resolver locale:
    disattivalo per questa rete (Impostazioni > ID Apple > iCloud > Relay privato).
  - Se una VPN (es. WireGuard) è attiva, potrebbe imporre il proprio DNS
    e avere la precedenza: verificalo con "scutil --dns".
  - Per disinstallare e ripristinare tutto: sudo scripts/uninstall.sh

 Icona nella barra di stato (facoltativa, NON richiede sudo):
    scripts/install-menubar.sh
========================================================================
EOF
