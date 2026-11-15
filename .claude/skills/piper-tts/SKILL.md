---
name: piper-tts
description: Brug når der arbejdes med OpenLST's tekst-til-tale – Piper CLI, danske stemmer (da_DK-talesyntese-medium), hastighed (length_scale), sætningssplit, TTS-wrapper i backend/tts eller /api/tts.
---

# Piper TTS i OpenLST

## Runtime
- Brug MIT-udgaven af Piper (rhasspy/piper release `2023.11.14-2`, `piper_windows_amd64.zip` / `piper_macos_*.tar.gz`). `OHF-Voice/piper1-gpl` er GPL-3.0 – brug den ikke uden licensbeslutning.
- Stemmer: huggingface.co/rhasspy/piper-voices → `da/da_DK/talesyntese/medium/` (`.onnx` + `.onnx.json` + `MODEL_CARD`). Verificér SHA-256 i fetch-script.
- Sample rate læses fra `.onnx.json` → `audio.sample_rate` (talesyntese medium: 22050). Hardcod aldrig.

## Udvikling på macOS
Piper's macOS-release virker ikke på Apple Silicon ("bad CPU type"). Brug Python-udgaven `piper-tts` (piper1-gpl, GPL-3.0) i `runtime/piper-venv` – **kun lokalt, distribueres aldrig**. `scripts/setup-dev-macos.sh` sætter det op.

## Protokol i OpenLST (`backend/tts/piper.go`)
`piper -m <model> --output_dir <tmp> --length_scale <ls>` som langlivet proces. Én tekstlinje på stdin → én WAV i `<tmp>`; stien meldes på stdout (piper.exe) eller stderr (Python, `INFO:__main__:Wrote <sti>`). Stien matches ud fra `<tmp>` (mellemrum i brugernavne!), filen læses og slettes. Ny hastighed = genstart af processen (~0,3 s på M5).

## CLI
```sh
echo 'Jeg cykler i skole i morgen.' | piper -m da_DK-talesyntese-medium.onnx --length_scale 1.11 -f out.wav
# rå 16-bit mono PCM på stdout:
piper -m model.onnx --output_raw < tekst.txt
# langlivet proces, én JSON pr. linje på stdin (model indlæses én gang):
piper -m model.onnx --json-input --output_raw
#   {"text":"...", "length_scale":1.11}
```
Andre flag: `--sentence_silence <sek>`, `--noise_scale`, `--noise_w`, `--espeak_data <dir>` (kræves hvis espeak-ng-data ikke ligger ved siden af exe).

## Regler
- `length_scale = 1 / speed`; speed klemmes til [0.6, 1.4], default 0.9.
- Én persistent Piper-proces pr. stemme; serialisér requests (mutex/kø); genstart ved crash; kill ved shutdown.
- Med `--output_raw` er der ingen længde-header: læs til næste request er færdig er upålideligt → foretræk én kortlivet `-f`-kørsel pr. sætning hvis framing bliver et problem, og mål forskellen (model-load ~0.5–2 s på Surface Go).
- Frontend splitter i sætninger (`Intl.Segmenter('da',{granularity:'sentence'})` + fallback); backend modtager én sætning ad gangen → giver præcis highlight.
- Log aldrig teksten – kun antal tegn og ms.
- Wrap PCM i 44-byte WAV-header før retur som `audio/wav`.
