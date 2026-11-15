---
name: whisper-stt
description: Brug når der arbejdes med OpenLST's tale-til-tekst – whisper.cpp (whisper-cli/whisper-server), ggml-modeller (tiny/base q5/q8), dansk sprog, 16 kHz WAV, optagelse i browser via AudioWorklet, /api/stt og STT-benchmark.
---

# whisper.cpp STT i OpenLST

## Runtime og modeller
- github.com/ggml-org/whisper.cpp releases (Windows x64 zip; CPU-build). Binærnavne: `whisper-cli`, `whisper-server` (gamle navne `main`/`server` er deprecated).
- Modeller: huggingface.co/ggerganov/whisper.cpp → `ggml-tiny-q5_1.bin`, `ggml-tiny-q8_0.bin`, `ggml-base-q5_1.bin`. Brug kun multilingual (ikke `.en`).

## Input-krav
- **16 kHz, mono, 16-bit PCM WAV.** Andet input fejler eller kræver ffmpeg – undgå ffmpeg.
- Browser: `getUserMedia` → `AudioWorklet` samler Float32 → resample til 16 kHz → Int16 → WAV i JS. Brug ikke MediaRecorder (giver webm/opus).

## CLI
```sh
whisper-cli -m models/stt/ggml-tiny-q5_1.bin -l da -t 2 -nt -f in.wav      # -nt = ingen timestamps
whisper-cli ... -oj -of out                                               # JSON-output
whisper-server -m ggml-tiny-q5_1.bin -l da -t 2 --host 127.0.0.1 --port 8766
curl -F file=@in.wav -F response_format=json -F language=da http://127.0.0.1:8766/inference
```

## Regler
- Sprog altid `da` (aldrig auto-detect – tiny gætter forkert på korte klip).
- `-t` = antal fysiske kerner (Surface Go: 2).
- **OpenLST bruger `whisper-cli` pr. diktering, ikke `whisper-server`** (beslutning #003 revideret): serveren åbner en ekstra port, som hjemmesider kan nå, og holder RAM optaget. Implementering: `backend/stt/whisper.go`.
- Udvikling på Mac: `whisper-cli` fra Homebrew (bruger Metal-GPU – tilføj `-ng` i `stt_extra_args` for at efterligne CPU-only).
- Midlertidige filer: `os.CreateTemp` + `defer os.Remove`, også på fejlstier. Foretræk at streame bytes direkte uden disk.
- Begræns lyd (fx 60 s). Trim stilhed i start/slut før inferens.
- Trim whitespace og fjern whisper-artefakter som `[BLANK_AUDIO]`, `(musik)`.
- Benchmark: RTF = processing_time / audio_duration. Mål model-load separat.
