#!/usr/bin/env bash
set -euo pipefail

APP_NAME="pather"

if [[ -d "/opt/homebrew/bin" ]]; then
  BIN_DIR="/opt/homebrew/bin"
else
  BIN_DIR="/usr/local/bin"
fi

BIN_PATH="$BIN_DIR/$APP_NAME"

echo "Building $APP_NAME..."
go build -o "$APP_NAME" cmd/pather/main.go

echo "Installing to $BIN_PATH..."
sudo mkdir -p "$BIN_DIR"
sudo mv "$APP_NAME" "$BIN_PATH"

echo "Done."
echo "Installed at: $BIN_PATH"
echo
echo "Try:"
echo "  pather -b"
