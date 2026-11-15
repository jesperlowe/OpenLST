#!/usr/bin/env bash
# Bygger en Windows-testpakke på macOS/Linux:
#   dist/OpenLST-Setup-<version>.exe     per-user installer (ingen admin)
#   dist/OpenLST-portable-<version>.zip  samme filer som zip
#
# ⚠️ Testbuild: binærerne er IKKE kodesigneret, og piper/whisper er hentet som
# færdige releases. Release-builds skal bygges og signeres i CI (#014, #015).
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-0.1.0-test}"
CACHE=build/cache
STAGE=build/win/OpenLST
PIPER_URL=https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_windows_amd64.zip
PIPER_SHA=f3c58906402b24f3a96d92145f58acba6d86c9b5db896d207f78dc80811efcea
WHISPER_URL=https://github.com/ggml-org/whisper.cpp/releases/download/v1.9.2/whisper-bin-x64.zip
WHISPER_SHA=49dcc16de826f20bd53d44f947a1ae49dfa81f86cad67a64d80820cb192d674a

fetch() { # url sha
  local f="$CACHE/$(basename "$1")"
  [ -f "$f" ] || curl -fL --retry 3 -o "$f" "$1"
  echo "$2  $f" | shasum -a 256 -c - >/dev/null || { echo "SHA-256 passer ikke: $f"; exit 1; }
  echo "$f"
}

mkdir -p "$CACHE" dist
rm -rf build/win && mkdir -p "$STAGE/runtime/whisper" "$STAGE/models/tts" "$STAGE/models/stt"

echo "• Modeller"
shasum -a 256 -c scripts/models.sha256 >/dev/null
cp models/tts/da_DK-talesyntese-medium.onnx models/tts/da_DK-talesyntese-medium.onnx.json "$STAGE/models/tts/"
[ -f models/tts/da_DK-talesyntese-medium.MODEL_CARD ] && cp models/tts/da_DK-talesyntese-medium.MODEL_CARD "$STAGE/models/tts/"
cp models/stt/ggml-*.bin "$STAGE/models/stt/"

echo "• Piper (Windows)"
unzip -q "$(fetch $PIPER_URL $PIPER_SHA)" -d "$STAGE/runtime"   # -> runtime/piper/
rm -rf "$STAGE/runtime/piper/pkgconfig"

echo "• whisper.cpp (Windows, alle CPU-varianter inkl. SSE4.2/x64 uden AVX)"
W=build/whisper-x64 && rm -rf $W && unzip -q "$(fetch $WHISPER_URL $WHISPER_SHA)" -d $W
cp $W/Release/whisper-cli.exe $W/Release/whisper.dll $W/Release/ggml.dll $W/Release/ggml-base.dll $W/Release/ggml-cpu-*.dll "$STAGE/runtime/whisper/"

echo "• Browser-extension"
./scripts/sync-extension.sh >/dev/null
cp -R extension "$STAGE/extension"

echo "• openlst.exe"
LDFLAGS="-s -w -X openlst/backend/server.Version=$VERSION"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS -H windowsgui" -o "$STAGE/openlst.exe" ./backend/cmd/openlst

cp LICENSE THIRD_PARTY_LICENSES.md "$STAGE/"
printf '@echo off\r\nrem Udvikler-benchmark. Resultatet gemmes i %%LOCALAPPDATA%%\\OpenLST og åbnes i Notesblok.\r\n"%%~dp0openlst.exe" --bench\r\n' > "$STAGE/Benchmark.cmd"
mkdir -p "$STAGE/bench"
printf 'Læg egne optagelser her til benchmark:\r\n  navn.wav  (16-bit mono WAV, gerne 16 kHz)\r\n  navn.txt  (det der bliver sagt)\r\n' > "$STAGE/bench/LÆS MIG.txt"

echo "• Pakker"
rm -f installer/setup/payload/OpenLST.zip
(cd build/win && zip -qr -X ../../installer/setup/payload/OpenLST.zip OpenLST)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "dist/OpenLST-Setup-$VERSION.exe" ./installer/setup
cp installer/setup/payload/OpenLST.zip "dist/OpenLST-portable-$VERSION.zip"
rm -f installer/setup/payload/OpenLST.zip

(cd dist && shasum -a 256 "OpenLST-Setup-$VERSION.exe" "OpenLST-portable-$VERSION.zip" > "SHA256SUMS-$VERSION.txt")
ls -lh dist/*"$VERSION"*
