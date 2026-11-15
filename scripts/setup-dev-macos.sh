#!/usr/bin/env bash
# Udviklingsmiljø på macOS (Apple Silicon).
# Piper's macOS-release virker ikke på Apple Silicon, så til udvikling bruges
# Python-udgaven (piper-tts, GPL-3.0) i et lokalt venv. Den distribueres ALDRIG.
set -euo pipefail
cd "$(dirname "$0")/.."
brew install go whisper-cpp
python3 -m venv runtime/piper-venv
runtime/piper-venv/bin/pip install --quiet piper-tts
./scripts/fetch-models.sh
echo "Klar. Start med: go run ./backend/cmd/openlst"
