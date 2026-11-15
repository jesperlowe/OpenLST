---
name: license-audit
description: Brug når tredjepartskomponenter tilføjes eller opdateres (Piper, espeak-ng, ONNX Runtime, whisper.cpp, modeller, skrifttyper, Go-moduler), ved arbejde på THIRD_PARTY_LICENSES.md, eller før en release/installer distribueres.
---

# Licens-audit

Egen kode: MIT. Distribution kræver at hver bundlet komponent er tjekket ved kilden – stol ikke på hukommelse eller denne fil alene.

## Fremgangsmåde for hver komponent
1. Find præcis version/commit/fil der bundles (hash).
2. Læs LICENSE-filen i det konkrete repo/release – og for modeller: `MODEL_CARD` + datasættets licens.
3. Notér: navn, version, URL, licens (SPDX), copyright-holder, krav (attribution, kildetilgængelighed, NOTICE).
4. Tilføj licensteksten til `THIRD_PARTY_LICENSES.md` og kopiér fulde tekster til `licenses/` i installeren.

## Kendte opmærksomhedspunkter
- **espeak-ng (GPL-3.0)** er linket ind i piper/piper-phonemize. Som separat program kaldt via subprocess er det aggregat → OpenLST-kode kan være MIT, men installeren skal medtage GPL-tekst og angive hvor kildekoden kan hentes.
- **Piper**: `rhasspy/piper` (MIT, arkiveret) vs. `OHF-Voice/piper1-gpl` (GPL-3.0). Notér hvilken der bundles.
- **Piper-stemmer**: licens varierer pr. stemme/datasæt – `da_DK-talesyntese` skal tjekkes i dens MODEL_CARD. Ukendt/ikke-kommerciel licens = må ikke bundles; i så fald lad installeren hente modellen separat eller vælg anden stemme.
- **Whisper-weights**: MIT (openai/whisper). ggml-konverteringer fra ggerganov/whisper.cpp – samme licens.
- **ONNX Runtime**: MIT. **whisper.cpp/ggml**: MIT.
- Skrifttyper: SIL OFL 1.1 – må bundles, ikke sælges alene.
- Go-moduler: `go-licenses report ./...` (hvis der overhovedet bruges eksterne moduler – stdlib foretrækkes).

## Output
`THIRD_PARTY_LICENSES.md` med tabel + fulde licenstekster, og et afsnit "Status: verificeret <dato> af <navn>" pr. komponent. Ingen komponent må stå som "forventet" ved release.
