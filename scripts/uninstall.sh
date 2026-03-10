#!/usr/bin/env bash
# GhostSpell uninstaller for macOS and Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/uninstall.sh | bash
#
# What it does:
#   1. Stops GhostSpell if it's running
#   2. Removes the app binary (macOS: /Applications/GhostSpell.app, Linux: /usr/local/bin/ghostspell)
#   3. Removes config, logs, and all app data
#
set -euo pipefail

info()  { printf '\033[1;34m%s\033[0m\n' "$*"; }
ok()    { printf '\033[1;32m%s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m%s\033[0m\n' "$*"; }

detect_os() {
    case "$(uname -s)" in
        Darwin) echo "darwin" ;;
        Linux)  echo "linux" ;;
        *)      echo "unsupported" ;;
    esac
}

# --- Stop running instance --------------------------------------------------

info "Stopping GhostSpell if running..."
pkill -f GhostSpell 2>/dev/null || pkill -f ghostspell 2>/dev/null || true
sleep 1

# --- Platform-specific removal ----------------------------------------------

os=$(detect_os)

case "$os" in
    darwin)
        info "Removing /Applications/GhostSpell.app..."
        if [ -d /Applications/GhostSpell.app ]; then
            rm -rf /Applications/GhostSpell.app 2>/dev/null || {
                info "Need admin permission..."
                sudo rm -rf /Applications/GhostSpell.app
            }
        fi

        info "Removing app data (~/Library/Application Support/GhostSpell/)..."
        rm -rf "$HOME/Library/Application Support/GhostSpell"
        ;;

    linux)
        info "Removing /usr/local/bin/ghostspell..."
        if [ -w /usr/local/bin ]; then
            rm -f /usr/local/bin/ghostspell
        else
            sudo rm -f /usr/local/bin/ghostspell
        fi

        info "Removing app data (~/.config/GhostSpell/)..."
        rm -rf "$HOME/.config/GhostSpell"
        ;;

    *)
        warn "Unsupported OS. Remove GhostSpell manually."
        exit 1
        ;;
esac

ok ""
ok "GhostSpell has been uninstalled."
ok ""

if [ "$os" = "darwin" ]; then
    warn "NOTE: macOS privacy permissions must be removed manually."
    echo "  Open System Settings and remove GhostSpell from both:"
    echo "  1. Privacy & Security > Accessibility"
    echo "  2. Privacy & Security > Input Monitoring"
    echo ""
    info "Opening Privacy & Security settings..."
    open "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility" 2>/dev/null || true
fi
