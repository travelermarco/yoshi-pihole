#!/usr/bin/env bash
# Installs the Yoshi Pi-hole menu bar helper as a per-user LaunchAgent.
# Unlike scripts/install.sh (the DNS daemon), this needs NO sudo: it only
# touches files under the current user's home directory, and only runs
# while that user is logged into a GUI session.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_SUPPORT="$HOME/Library/Application Support/YoshiPihole"
APP_BUNDLE="$APP_SUPPORT/YoshiMenuBar.app"
BIN_PATH="$APP_BUNDLE/Contents/MacOS/yoshi-menubar"
LABEL="com.yoshi.pihole.menubar"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"

GO_BIN=""
if command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
elif [[ -x "$HOME/sdk/go/bin/go" ]]; then
  GO_BIN="$HOME/sdk/go/bin/go"
else
  echo "Non trovo il toolchain Go. Esegui prima il bootstrap di Go."
  exit 1
fi

echo "==> Compilazione dell'icona per la barra di stato"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
( cd "$REPO_DIR" && "$GO_BIN" build -o "$BIN_PATH" ./cmd/yoshi-menubar )

echo "==> Scrittura del bundle applicazione (nessuna icona nel Dock, solo barra di stato)"
cat > "$APP_BUNDLE/Contents/Info.plist" <<'PLIST_EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key><string>Yoshi Pi-hole</string>
    <key>CFBundleIdentifier</key><string>com.yoshi.pihole.menubar</string>
    <key>CFBundleExecutable</key><string>yoshi-menubar</string>
    <key>CFBundlePackageType</key><string>APPL</string>
    <key>CFBundleShortVersionString</key><string>0.1.0</string>
    <key>LSUIElement</key><true/>
</dict>
</plist>
PLIST_EOF

echo "==> Scrittura del LaunchAgent (avvio automatico ad ogni login)"
mkdir -p "$HOME/Library/LaunchAgents"
cat > "$PLIST" <<PLIST_EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>${LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${BIN_PATH}</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>ProcessType</key><string>Interactive</string>
    <key>StandardOutPath</key><string>${APP_SUPPORT}/menubar.log</string>
    <key>StandardErrorPath</key><string>${APP_SUPPORT}/menubar.err.log</string>
</dict>
</plist>
PLIST_EOF

echo "==> Avvio dell'icona nella barra di stato"
launchctl bootout "gui/$(id -u)/$LABEL" >/dev/null 2>&1 || true
launchctl bootstrap "gui/$(id -u)" "$PLIST"
launchctl kickstart -k "gui/$(id -u)/$LABEL"

echo "Fatto. Icona attiva nella barra di stato in alto, partirà automaticamente ad ogni login."
echo "Per rimuoverla: launchctl bootout gui/\$(id -u)/$LABEL && rm \"$PLIST\""
