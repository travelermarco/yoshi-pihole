#!/usr/bin/env bash
# Reverses scripts/install.sh: stops the LaunchDaemon, restores this Mac's
# original DNS settings, and (with --purge) removes installed files.
set -euo pipefail

PREFIX="/usr/local/yoshi-pihole"
PLIST="/Library/LaunchDaemons/com.yoshi.pihole.plist"
LABEL="com.yoshi.pihole"
DNS_BACKUP="$PREFIX/data/dns-backup.json"
PURGE=0
ASSUME_YES=0

for arg in "$@"; do
  case "$arg" in
    --purge) PURGE=1 ;;
    --yes|-y) ASSUME_YES=1 ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Questo script deve essere eseguito con sudo:"
  echo "  sudo scripts/uninstall.sh"
  exit 1
fi

echo "========================================================================"
echo " Yoshi Pi-hole - disinstallazione"
echo " Verranno: fermato il demone, ripristinato il DNS di sistema originale."
[[ "$PURGE" -eq 1 ]] && echo " --purge: verrà rimossa anche $PREFIX (dati/log/config inclusi)."
echo "========================================================================"

if [[ "$ASSUME_YES" -ne 1 ]]; then
  read -r -p "Procedere? [y/N] " reply
  [[ "$reply" =~ ^[Yy]$ ]] || { echo "Annullato."; exit 0; }
fi

echo "==> Arresto del demone"
launchctl bootout system "$PLIST" >/dev/null 2>&1 || true
rm -f "$PLIST"

if [[ -f "$DNS_BACKUP" ]]; then
  echo "==> Ripristino delle impostazioni DNS originali da $DNS_BACKUP"
  # Backup format is the simple JSON this project's own install.sh writes:
  #   { "Service Name": ["dns1","dns2"], "Other Service": [] }
  while IFS= read -r line; do
    [[ "$line" =~ ^\ *\"([^\"]+)\"\:\ *\[(.*)\]\ *,?\ *$ ]] || continue
    service="${BASH_REMATCH[1]}"
    list="${BASH_REMATCH[2]}"
    servers="$(echo "$list" | tr -d '"' | tr ',' ' ')"
    if [[ -z "$(echo "$servers" | tr -d '[:space:]')" ]]; then
      echo "    - $service -> DHCP (Empty)"
      networksetup -setdnsservers "$service" Empty || true
    else
      echo "    - $service -> $servers"
      # shellcheck disable=SC2086
      networksetup -setdnsservers "$service" $servers || true
    fi
  done < "$DNS_BACKUP"
else
  echo "==> Nessun backup DNS trovato in $DNS_BACKUP: ripristino su DHCP (Empty) per tutti i servizi attivi"
  networksetup -listallnetworkservices | tail -n +2 | grep -v '^\*' | while IFS= read -r service; do
    [[ -z "$service" ]] && continue
    networksetup -setdnsservers "$service" Empty || true
  done
fi

echo "==> Pulizia cache DNS"
dscacheutil -flushcache || true
killall -HUP mDNSResponder || true

if [[ "$PURGE" -eq 1 ]]; then
  echo "==> Rimozione $PREFIX"
  rm -rf "$PREFIX"
else
  echo "==> $PREFIX mantenuta (dati/config/log). Usa --purge per rimuoverla."
fi

echo "Disinstallazione completata."
echo ""
echo "Nota: questo script rimuove solo il demone DNS (root)."
echo "Per rimuovere anche l'icona nella barra di stato, come utente normale (senza sudo):"
echo "  launchctl bootout gui/\$(id -u)/com.yoshi.pihole.menubar"
echo "  rm -rf ~/Library/LaunchAgents/com.yoshi.pihole.menubar.plist \"\$HOME/Library/Application Support/YoshiPihole\""
