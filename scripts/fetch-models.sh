#!/usr/bin/env bash
# Henter danske modeller og verificerer SHA-256. Kør fra repo-roden.
set -euo pipefail
cd "$(dirname "$0")/.."
PIPER=https://huggingface.co/rhasspy/piper-voices/resolve/main/da/da_DK/talesyntese/medium
WHISPER=https://huggingface.co/ggerganov/whisper.cpp/resolve/main
mkdir -p models/tts models/stt
get() { [ -f "$2" ] || curl -fL --retry 3 -o "$2" "$1"; }
get "$PIPER/da_DK-talesyntese-medium.onnx"      models/tts/da_DK-talesyntese-medium.onnx
get "$PIPER/da_DK-talesyntese-medium.onnx.json" models/tts/da_DK-talesyntese-medium.onnx.json
get "$PIPER/MODEL_CARD"                         models/tts/da_DK-talesyntese-medium.MODEL_CARD
get "$WHISPER/ggml-tiny-q5_1.bin"               models/stt/ggml-tiny-q5_1.bin
shasum -a 256 -c scripts/models.sha256
