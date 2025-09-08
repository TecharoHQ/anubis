#!/usr/bin/env bash

set -euo pipefail

CERT_PATH="/usr/local/share/ca-certificates/minica/minica.pem"
CERT_NAME="minica"
TRUST_FLAGS="C,,"

FIREFOX_DIR="$HOME/.mozilla/firefox"
PALEMOON_DIR="$HOME/.moonchild productions/pale moon"

echo "🔄 Updating system CA certificates..."
update-ca-certificates

# 🌀 Trigger Pale Moon to create its profile if needed
if command -v palemoon &>/dev/null; then
  echo "🚀 Launching Pale Moon to initialize profile..."
  palemoon &>/dev/null &
  PALEMOON_PID=$!

  # Wait up to 20 seconds for prefs.js to be created
  for i in {1..20}; do
    set +e
    PROFILE_DIR=$(grep Path ~/.moonchild\ productions/pale\ moon/profiles.ini | cut -d= -f2)
    PREFS_FILE="$HOME/.moonchild productions/pale moon/$PROFILE_DIR/prefs.js"

    if [[ -f "$PREFS_FILE" ]]; then
      set -e
      echo "✅ prefs.js found at: $PREFS_FILE"
      break
    fi

    sleep 5
  done

  kill $PALEMOON_PID 2>/dev/null || true
  wait $PALEMOON_PID 2>/dev/null || true

  if [[ ! -f "$PREFS_FILE" ]]; then
    echo "❌ prefs.js not found. Pale Moon did not fully initialize."
    exit 1
  fi
else
  echo "⚠️ Pale Moon is not installed or not in PATH. Skipping profile bootstrap."
fi

echo 'user_pref("security.cert_pinning.enforcement_level", 0);' >>"$PREFS_FILE"

echo "✅ TLS cert validation disabled in Pale Moon profile: $PROFILE_DIR"

# 🔧 Ensure certutil is installed
if ! command -v certutil &>/dev/null; then
  if [ -f /etc/debian_version ]; then
    echo "🔧 'certutil' not found. Installing via apt..."
    apt-get update
    apt-get install -y libnss3-tools
  else
    echo "❌ 'certutil' not found and install is only supported on Debian-based systems."
    exit 1
  fi
fi

import_cert_to_profiles() {
  local base_dir="$1"
  local browser_name="$2"
  local profile_glob="$3"

  if [ ! -d "$base_dir" ]; then
    echo "⚠️  $browser_name profile directory not found: $base_dir"
    return
  fi

  echo "📌 Searching for $browser_name profiles in: $base_dir"

  local found=0

  for profile in "$base_dir"/$profile_glob; do
    if [ ! -d "$profile" ]; then
      continue
    fi

    found=1
    local db_path="sql:$profile"
    echo "🔍 Processing $browser_name profile: $profile"

    if certutil -L -d "$db_path" | grep -q "^$CERT_NAME"; then
      echo "  ✅ Certificate '$CERT_NAME' already exists in profile."
      continue
    fi

    certutil -A -n "$CERT_NAME" -t "$TRUST_FLAGS" -i "$CERT_PATH" -d "$db_path"
    echo "  ➕ Added certificate '$CERT_NAME' to $browser_name profile."
  done

  if [ "$found" -eq 0 ]; then
    echo "⚠️  No $browser_name profiles found in: $base_dir"
  fi
}

import_cert_to_profiles "$FIREFOX_DIR" "Firefox" "*.default*"
import_cert_to_profiles "$PALEMOON_DIR" "Pale Moon" "*.*"

echo "✅ Done. Firefox and Pale Moon profiles updated with '$CERT_NAME' certificate."
