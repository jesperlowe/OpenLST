#!/usr/bin/env bash
# Kopierer fælles JavaScript fra frontend/ til extension/lib/.
# frontend/ er originalen; extension_sync_test.go fejler, hvis de afviger.
set -euo pipefail
cd "$(dirname "$0")/.."
for f in sentences.js wav.js recorder-worklet.js; do cp "frontend/$f" "extension/lib/$f"; done
echo "extension/lib opdateret"
