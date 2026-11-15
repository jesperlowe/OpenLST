# Tredjepartskomponenter

> ⚠️ **Status: UDKAST.** Ingen komponent er endeligt verificeret. Alle skal kontrolleres ved kilden før distribution (milestone 7).

| Komponent | Version | Licens (forventet) | Kilde | Bundles | Status |
|---|---|---|---|---|---|
| Piper (rhasspy/piper) | 2023.11.14-2 | MIT | https://github.com/rhasspy/piper | Windows | Ikke verificeret |
| espeak-ng (i Piper) | 1.52 | GPL-3.0-or-later | https://github.com/espeak-ng/espeak-ng | Windows | Ikke verificeret – separat program (aggregat); GPL-tekst og kildehenvisning skal med |
| piper-phonemize | 1.2 | MIT | https://github.com/rhasspy/piper-phonemize | Windows | Ikke verificeret |
| ONNX Runtime | 1.14.1 | MIT | https://github.com/microsoft/onnxruntime | Windows | Ikke verificeret |
| da_DK-talesyntese-medium | – | Datasæt: CC0 (ifølge MODEL_CARD) | https://huggingface.co/rhasspy/piper-voices | Windows | ⚠️ Finjusteret fra en_US-lessac – lessac-datasættets licens skal tjekkes |
| whisper.cpp / ggml | 1.9.x | MIT | https://github.com/ggml-org/whisper.cpp | Windows | Ikke verificeret |
| Whisper-weights (ggml-tiny-q5_1) | – | MIT | https://github.com/openai/whisper | Windows | Ikke verificeret |
| piper-tts (Python, piper1-gpl) | 1.8.0 | GPL-3.0 | https://github.com/OHF-Voice/piper1-gpl | **Nej** – kun lokal udvikling på macOS | – |

| golang.org/x/sys | v0.50.0 | BSD-3-Clause | https://pkg.go.dev/golang.org/x/sys | Ja (i openlst.exe) | Ikke verificeret |
| Microsoft Visual C++ Runtime | 2015–2022 | Microsoft-licens | – | **Nej** – krævet af testbuildets piper/whisper; release bygges med /MT (#034) | – |

Go-standardbiblioteket (BSD-3-Clause) indgår i `openlst.exe`.
