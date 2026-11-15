# OpenLST

Open-source, lokal læse- og skrivestøtte til danske børn – med særligt fokus på ordblindhed.

- 🔊 **Oplæsning** af dansk tekst med fremhævning af den sætning, der læses
- 🎤 **Diktering**: tal dansk, og teksten skrives i tekstfeltet
- 🔒 **Alt kører på computeren.** Ingen cloud, ingen konto, ingen telemetry
- 💻 Lavet til ældre hardware (reference: Surface Go 1, 4 GB RAM, ingen GPU)

Udgiver: **WorkingOnIT** · Licens: [MIT](LICENSE) · Tredjepart: [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)

## Sådan virker det

```
Edge/Chrome ──► http://localhost:8765 ──► openlst (Go) ──► Piper (TTS)
                    (kun 127.0.0.1)                   └──► whisper.cpp (STT)
```

Browseren er brugerfladen. Tekst og lyd sendes kun til OpenLST på samme computer.

## Kom i gang (udvikling)

### macOS
```sh
./scripts/setup-dev-macos.sh      # go, whisper.cpp, piper (dev), modeller
go run ./backend/cmd/openlst      # åbner http://localhost:8765
```

### Windows-testpakke (bygges på macOS/Linux)
```sh
./scripts/build-windows.sh   # → dist/OpenLST-Setup-<version>.exe + portable zip
```
Installeren kræver ikke administrator. Se [docs/TEST-SURFACE-GO.md](docs/TEST-SURFACE-GO.md).

Flag: `--open=false` (åbn ikke browser), `--background` (autostart), `--bench` (udvikler-benchmark), `--uninstall` (Windows), `--root <mappe>`.

### Browser-extension (Edge/Chrome)
`edge://extensions` → Udviklertilstand → *Indlæs udpakket* → mappen `extension/`.
Fælles JavaScript kopieres fra `frontend/` med `scripts/sync-extension.sh` (en test fejler, hvis de afviger).

## Test

```sh
go test ./...
```

Testene dækker blandt andet adgangskontrollen (fremmede hjemmesider afvises) og en privatlivstest, der fejler, hvis koden indeholder adresser uden for computeren.

## Struktur

```
backend/
  cmd/openlst/   start, single-instance, browser
  server/        HTTP-API og sikkerhed
  tts/           TTS-interface + Piper
  stt/           STT-interface + whisper.cpp
  audio/         WAV
  config/        stier og indstillinger
  bench/         udvikler-benchmark
  install/       per-user installation/afinstallation (Windows)
frontend/        HTML/CSS/vanilla JS (indlejret i exe)
extension/       Edge/Chrome-extension (Manifest V3)
installer/setup/ OpenLST-Setup.exe
models/          modeller (hentes med scripts/, ikke i git)
runtime/         piper/whisper-binærer (hentes, ikke i git)
docs/            plan, API, sikkerhed
```

## Dokumentation

- [docs/PLAN.md](docs/PLAN.md) – plan og milestones
- [docs/API.md](docs/API.md) – lokalt HTTP-API
- [docs/SECURITY.md](docs/SECURITY.md) – sikkerhed og privatliv
- [docs/TEST-SURFACE-GO.md](docs/TEST-SURFACE-GO.md) – test på Surface Go
