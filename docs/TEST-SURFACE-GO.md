# Test på Surface Go (testbuild 0.1.0-test)

## Installér
1. Kopiér `OpenLST-Setup-0.1.0-test.exe` til Surface Go (USB eller OneDrive).
2. Dobbeltklik. Windows SmartScreen advarer, fordi testversionen **ikke er kodesigneret** endnu:
   klik **Flere oplysninger → Kør alligevel**.
3. Installationen kræver **ikke** administrator. Den lægger filer i `%LOCALAPPDATA%\Programs\OpenLST`
   og åbner OpenLST i browseren.
4. Hvis installeren melder, at *Visual C++ Runtime mangler*, virker oplæsning/diktering ikke i denne
   testversion. Installér "Microsoft Visual C++ Redistributable 2015–2022 (x64)" (kræver admin) og prøv igen.

Alternativ uden installer: pak `OpenLST-portable-0.1.0-test.zip` ud og kør `OpenLST\openlst.exe`.

## Acceptance tests (kravspec §18)
Kør dem først med internet, derefter med **flytilstand slået til**.

| # | Test | Forventet |
|---|---|---|
| 1 | Skriv "Jeg cykler i skole i morgen." → **Læs** | Forståelig dansk tale, sætningen fremhæves |
| 2 | **Dikter** → sig sætningen → **Stop diktat** | Teksten står i feltet (små tegnsætningsforskelle ok) |
| 3 | Flytilstand → gentag 1 og 2 | Virker stadig |
| 4 | Ressourceovervågning → Netværk | `openlst.exe`, `piper.exe`, `whisper-cli.exe` har ingen trafik ud af maskinen |

Prøv også: Pause/Fortsæt, Stop, Gentag (gentager aktuel sætning), hastighed, klik på en sætning under oplæsning.

## Browser-udvidelse (Edge)
1. `edge://extensions` → slå **Udviklertilstand** til → **Indlæs udpakket** →
   vælg `%LOCALAPPDATA%\Programs\OpenLST\extension`.
2. En opsætningsside åbner: tryk **Giv adgang til mikrofon**.
3. Gå til en hjemmeside (fx en artikel) → klik på OpenLST-ikonet → værktøjslinjen vises nederst til højre.
4. Markér tekst → **🔊 Læs** (eller højreklik → *Læs med OpenLST*, eller `Alt+Shift+L`).
5. Klik i et afsnit uden at markere → **🔊 Læs** læser afsnittet.
6. Klik i et tekstfelt (fx en søgeboks) → **🎤 Dikter** → tal → tryk igen.

## Benchmark
Kør `%LOCALAPPDATA%\Programs\OpenLST\Benchmark.cmd`. Resultatet åbner i Notesblok og gemmes i
`%LOCALAPPDATA%\OpenLST\bench-*.md`. Send filen tilbage.

For realistiske STT-tal: optag nogle danske sætninger (16-bit mono WAV) og læg dem i
`%LOCALAPPDATA%\Programs\OpenLST\bench\` sammen med en `.txt` med samme navn, der indeholder teksten.

## Skift dikteringsmodel
Tiny Q5 er standard. Hvis genkendelsen er for dårlig, opret
`%LOCALAPPDATA%\OpenLST\config.json`:

```json
{ "stt_model": "ggml-base-q5_1.bin" }
```

og genstart OpenLST (log af/på, eller luk `openlst.exe` i Jobliste og start fra Start-menuen).

## Fejlsøgning
- Log: `%LOCALAPPDATA%\OpenLST\openlst.log` (indeholder aldrig tekst eller lyd).
- Mikrofon: Indstillinger → Beskyttelse af personlige oplysninger → Mikrofon → tillad for Edge.
- Afinstallér: Indstillinger → Apps → Installerede apps → OpenLST.
