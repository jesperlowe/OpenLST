# OpenLST – implementeringsplan

Kilde: kravspecifikation (sektion 1–21). Denne plan fastlægger tekniske valg og rækkefølge.
Udgiver: **WorkingOnIT**. Status 2026-11-15: **M0–M6 bygget, M7 som testinstaller (usigneret), M8 som første extension** – mangler test på Surface Go. Se `docs/TEST-SURFACE-GO.md`. Alle beslutninger og begrundelser dokumenteres i Obsidian-vaulten (`Wiki/Personal/App or Code/OpenLST/00 - Start/OpenLST - Beslutningslog.md`).
Referencehardware: Surface Go 1 (Pentium 4415Y, 4 GB RAM, ingen GPU/NPU).

## 1. Tekniske hovedvalg

| Område | Valg | Begrundelse |
|---|---|---|
| Backend | **Go** – én statisk `openlst.exe` | ~10–15 MB RAM, opstart < 100 ms, ingen runtime, krydskompilering fra macOS → Windows, `net/http` i stdlib |
| TTS | **Piper** CLI (`piper.exe`, MIT-udgaven 2023.11.14-2) som langlivet subprocess | Model indlæses én gang; tekst ind via stdin (`--json-input`), rå PCM ud (`--output_raw`) |
| STT | **whisper.cpp** `whisper-cli` én gang pr. diktering (revideret #003) | Ingen ekstra HTTP-port som hjemmesider kan nå; RAM frigives mellem dikteringer. Model-load pr. kald måles på Surface Go |
| STT-model | `ggml-tiny-q5_1.bin`, `-l da`, `-t` = fysiske kerner (2) | Mindst mulig latency på 2-kerne CPU |
| Lydoptagelse | AudioWorklet → 16 kHz mono PCM → WAV kodet i JS | Ingen ffmpeg nødvendig; whisper.cpp kræver 16 kHz WAV |
| Frontend | HTML + CSS + vanilla JS, serveret via Go `embed` | Krav sektion 10 |
| Ventetid | Kort første bid (≤ ~50 tegn, delt ved komma), 2 bidder hentes foran, første bid forberedes mens eleven skriver, cache pr. tekst, Piper varmes op ved start og kører med høj prioritet | Eleven må ikke vente (krav 2026-11-16) |
| Highlight V1 | Tekst splittes i sætninger i frontend; én TTS-request pr. sætning; afspilningskø styrer highlight | Highlight følger lyd præcist uden timing-gæt; kort tid til første lyd |
| Hastighed | Afspillerens `playbackRate` med bevaret tonehøjde (0.6–1.4, default 0.9); Piper kører altid ved 1.0 (revideret 2026-11-16) | Øjeblikkelig – ingen genstart af Piper, ingen ny lyd ved skift |
| Installer | **Signeret MSIX** (Microsoft Store + winget) som primær kanal; **signeret** Inno Setup `OpenLST-Setup.exe` (per-user) som alternativ | Begge uden admin; MSIX er Windows' mest betroede installationsform (se §2a) |
| Autostart | MSIX `windows.startupTask` / Startup-mappe (ikke Windows-service) | En Windows-service kræver admin at installere; per-user proces er nok |
| Extension | Manifest V3, Edge + Chrome | Sektion 13–15 |

## 2. Sikkerhedsmodel (sektion 15)

1. Lyt kun på `127.0.0.1:8765` (aldrig `0.0.0.0`).
2. **Host-header-check**: kun `localhost:8765` / `127.0.0.1:8765` accepteres → beskytter mod DNS-rebinding.
3. **Origin-check** på alle `/api/*`: tilladt er
   - `http://localhost:8765` / `http://127.0.0.1:8765` (egen UI)
   - `chrome-extension://<kendt-id>` (konfigureret liste, ingen wildcard)
   - Alle andre Origins → 403. Requests uden Origin på POST → 403.
4. CORS-headere sendes kun til Origins fra listen ovenfor – aldrig `*`.
5. ~~Pairing-token~~ – droppet (#031): browseren sætter selv `Origin` på extension-kald, og hverken websites eller andre extensions kan forfalske den. Allowlisten over extension-ID'er er beskyttelsen.
6. Private Network Access-preflight (`Access-Control-Request-Private-Network`) besvares kun for tilladte Origins.
7. Upload-størrelse og varighed begrænses (fx 60 s / 2 MB lyd, 5.000 tegn tekst).

## 2a. Uden administrator – og stadig sikker og betroet

**Krav:** Installation, opdatering, drift og afinstallation skal kunne udføres af en almindelig bruger uden admin. Samtidig skal Windows, antivirus og skolens IT kunne stole på programmet.

### Hvorfor der ikke kræves admin
| Område | Løsning |
|---|---|
| Installationssted | `%LOCALAPPDATA%\Programs\OpenLST` (EXE) eller MSIX per-user pakke – ingen skrivning til `Program Files`, `HKLM` eller systemmapper |
| Netværk | Binding til `127.0.0.1` udløser **ikke** Windows Firewall-prompt (det gør `0.0.0.0`, og den kræver admin). Go `net/http` bruger ikke http.sys → ingen `netsh urlacl` |
| Port | 8765 > 1024; ingen reservation nødvendig |
| Autostart | Per-user (Startup-mappe / MSIX StartupTask) – ikke Windows-service, ikke `HKLM\Run`, ikke Scheduled Task med højeste rettigheder |
| Konfiguration | `%LOCALAPPDATA%\OpenLST\` – kun den aktuelle bruger har adgang (standard-ACL) |
| Mikrofon | Håndteres af browseren; ingen driver eller systemindstilling |
| Opdatering | Store/winget opdaterer per-user; EXE-varianten geninstalleres per-user. **Ingen egen auto-updater** (stor angrebsflade) |

### Hvad gør den "betroet"
1. **Kodesignering (Authenticode, SHA-256 + tidsstempel)** af alle .exe/.dll vi distribuerer – også `piper.exe`, `whisper-server.exe` og deres DLL'er. Tredjepartsbinærer **bygges selv fra kildekode i CI** (fastlåst commit) og signeres, så hele pakken har én udgiver.
   - Microsoft Store: Microsoft signerer MSIX'en – gratis udviklerkonto (individuel) / organisationskonto.
   - Uden for Store: **Azure Trusted Signing** (lav månedlig pris, EU-organisationer kan bruges) eller **SignPath Foundation** (gratis for open-source). Nøglen ligger i HSM/cloud – aldrig som fil i repo eller på en udviklermaskine.
   - Bemærk: siden 2024 giver EV-certifikater ikke længere øjeblikkelig SmartScreen-tillid; omdømme opbygges over tid. Store-distribution undgår SmartScreen-advarslen helt.
2. **Sporbar build-kæde (supply chain)**
   - Al release-build sker i GitHub Actions fra tag; ingen manuelt byggede binærer.
   - Afhængigheder fastlåst (Go-moduler via `go.sum`, piper/whisper.cpp via commit-hash, modeller via SHA-256).
   - **SBOM** (CycloneDX/SPDX) og **GitHub artifact attestations** (SLSA build provenance) pr. release; SHA-256-sum publiceres.
   - Branch protection, påkrævet review, signerede commits/tags, Dependabot og CodeQL på repoet.
3. **Skole-IT (Intune/AppLocker/WDAC)**
   - Mange skole-PC'er blokerer programmer i `%LOCALAPPDATA%` med AppLocker. Signeret kode gør det muligt at tillade OpenLST med én **publisher-regel** i stedet for hash-regler.
   - Tilbyd også en per-machine MSIX/MSI, som IT kan rulle ud via Intune (her er det IT, ikke barnet, der har admin).
   - `docs/IT-ADMIN.md`: udgivernavn, hashes, netværksadfærd (kun loopback), dataflow, hvordan man verificerer signatur og attestation.
4. **Mindste rettigheder under drift:** kører som almindelig bruger, ingen elevation (`asInvoker`-manifest i exe), ingen UAC-prompt nogensinde. Subprocesser arver samme begrænsede token og lukkes via Job Object.
5. **Antivirus:** Upload release-binærer til Microsoft Defender-analyse (false positive-indsendelse) før første release; undgå UPX-pakning og andre tricks der ligner malware.

## 3. Privatliv (sektion 9)

- Ingen udgående netværkskald i koden overhovedet – håndhæves med en test, der grepper efter `http.Get`/eksterne URL'er, og med en manuel trafiktest (Wireshark/Resource Monitor).
- Lyd skrives til `os.CreateTemp` og slettes med `defer os.Remove` (også ved fejl); helst sendes lyden direkte i hukommelsen til whisper-server uden disk.
- Ingen logning af tekst/transskription – kun længder og tidsforbrug.

## 4. Projektstruktur

```text
OpenLST/
├── backend/
│   ├── cmd/openlst/main.go        # entrypoint, config, flag
│   ├── server/                    # http, routing, security middleware, embed frontend
│   ├── tts/                       # Engine-interface + piper-implementering
│   ├── stt/                       # Engine-interface + whisper.cpp-implementering
│   ├── models/                    # model-registry (scanner models/-mappen)
│   └── bench/                     # developer benchmark (kun via CLI-flag/skjult endpoint)
├── frontend/  index.html app.js audio-recorder-worklet.js styles.css
├── models/    tts/ stt/           # ikke i git – hentes af scripts/fetch-models
├── runtime/   piper/ whisper/     # binære filer – ikke i git
├── extension/                     # MV3 (milestone 8)
├── installer/ openlst.iss
├── scripts/   fetch-models.(sh|ps1)  fetch-runtimes.(sh|ps1)  bench.ps1
├── docs/      PLAN.md  API.md  SECURITY.md  BENCHMARK.md
├── THIRD_PARTY_LICENSES.md
├── LICENSE (MIT)
└── README.md
```

Engine-interfaces (udvidelsespunkt for flere stemmer/modeller/engines):

```go
type TTSEngine interface {
    Voices() []Voice
    Synthesize(ctx context.Context, req TTSRequest) (PCM, error) // senere: + ord-timings
}
type STTEngine interface {
    Transcribe(ctx context.Context, wav io.Reader, lang string) (Transcript, error)
}
```

Modeller findes via `config.json` (`models_dir`) + scanning: `models/tts/*.onnx` (+ `.onnx.json`), `models/stt/ggml-*.bin`.

## 5. API (sektion 8)

| Endpoint | Input | Output |
|---|---|---|
| `GET /api/status` | – | `{"status":"ready","tts":true,"stt":true,"voices":[…],"stt_model":"tiny-q5_1"}` |
| `POST /api/tts` | `{"text","speed","language","voice?"}` | `audio/wav` (V1); senere streaming |
| `POST /api/stt` | `audio/wav` (16 kHz mono) | `{"text","language"}` |
| `GET /api/voices` | – | liste over installerede stemmer |

## 6. Milestones

Hver milestone afsluttes med noget, der kan køres og testes. Bygges i rækkefølge – ikke parallelt.

### M0 – Fundament (½ dag)
- `git init`-commit, MIT LICENSE, README-skelet, `.gitignore` (models/, runtime/).
- `scripts/fetch-models` og `scripts/fetch-runtimes` (macOS til udvikling, Windows til mål) med SHA-256-verifikation.
- **Licens-research** (se §8) – blokerer ikke kode, men skal være færdig før M7.

### M1 – Piper TTS fra CLI
- Hent `piper` + `da_DK-talesyntese-medium`.
- Verificér: `echo "Jeg cykler i skole i morgen." | piper -m … --length_scale 1.11 -f test.wav`.
- Mål generationstid og RAM på Mac – og så snart som muligt på Surface Go.
- **Done:** forståelig dansk WAV.

### M2 – whisper.cpp STT fra CLI
- Hent/byg whisper.cpp (Windows: officielt release-zip, AVX2 – 4415Y har AVX2, men tjek), `ggml-tiny-q5_1.bin`.
- `whisper-cli -m ggml-tiny-q5_1.bin -l da -t 2 -nt -f test.wav`.
- Rundtur: M1's WAV → resample 16 kHz → whisper → tekst.
- **Done:** "Jeg cykler i skole i morgen." genkendes.

### M3 – Local API (Go)
- `net/http`-server på 127.0.0.1:8765 med security-middleware (§2) fra dag ét.
- Piper-wrapper (persistent proces, mutex/kø, genstart ved crash).
- Whisper-wrapper (whisper-server lazy-start, idle-timeout).
- Tests: Go unit-tests for middleware (Origin/Host), sætningssplit, WAV-header; integrationstest bag build-tag.
- **Done:** `curl` kan lave TTS og STT; fremmed Origin får 403.

### M4 – Browserinterface
- Én side: tekstfelt, 🔊 Læs, 🎤 Dikter, ⏸ Pause, ⏹ Stop, hastighedsskyder.
- Store knapper (min. 64 px), høj kontrast, dyslexi-venlig skrift (fx Atkinson Hyperlegible/Lexend – lokalt bundlet, ingen Google Fonts), stor linjeafstand.
- Diktering: AudioWorklet → WAV → `/api/stt` → indsæt ved cursor; status "Lytter…" / "Skriver…".
- **Done:** Acceptance test 1 + 2 på Mac.

### M5 – Hastighed, pause, fortsæt, stop, gentag
- Afspilning via Web Audio (`AudioContext.suspend/resume`) eller `<audio>`-kø.
- Prefetch af næste sætning mens nuværende afspilles.
- Hastighed huskes i `localStorage`.

### M6 – Highlight sætning-for-sætning
- `Intl.Segmenter('da', {granularity:'sentence'})` med regex-fallback; danske forkortelser (fx, bl.a., f.eks., kl.).
- Visning: under oplæsning vises teksten i et læsevindue med `<span>` pr. sætning; tilbage til redigering ved stop.
- Forbered ord-niveau: `Synthesize` returnerer på sigt timings (Piper phoneme-alignment eller whisper-align på TTS-output).

### M7 – Windows installer

**Testbuild (2026-11-15):** `scripts/build-windows.sh` → `dist/OpenLST-Setup-<v>.exe` (Go-installer med indlejret zip, per-user, ingen admin, genveje, HKCU-afinstallation, `openlst.exe --uninstall`). Usigneret; bruger færdigbyggede piper/whisper med krav om VC++-runtime (#034).

- Krydskompilér `GOOS=windows GOARCH=amd64`.
- Inno Setup-script: installér til `%LOCALAPPDATA%\Programs\OpenLST`, Start-menu-genvej "OpenLST" (starter service hvis nødvendig + åbner browser), Startup-genvej, afinstallation.
- Single-instance: hvis port 8765 allerede svarer med OpenLST-status → åbn blot browseren.
- Bygges **kun** i GitHub Actions (windows-latest) fra tag: piper + whisper.cpp fra kildekode → signering af alle binærer → MSIX + Inno-EXE → signering af pakker → SBOM + attestation.
- MSIX indsendes til Microsoft Store; winget-manifest publiceres.
- `docs/IT-ADMIN.md` til skolers IT.
- **Done:** Acceptance test 1–4 på Surface Go, offline, **installeret som standardbruger uden admin**, uden SmartScreen-blokering, og `Get-AuthenticodeSignature` = Valid på alle binærer.

### M8 – Edge/Chrome-extension: læse- og skrivestøtte på alle hjemmesider

**Mål:** En browser-extension der fungerer som et IntoWords-lignende værktøj – læse- og skrivestøtte direkte på de hjemmesider, eleven bruger – men hvor al tale behandles af OpenLST **lokalt** på computeren via `http://127.0.0.1:8765`. Ingen cloud, ingen konto.

**Arkitektur**

```text
Hjemmeside ── content.js (værktøjslinje, markering, fremhævning, indsæt tekst)
                 │ chrome.runtime-beskeder (websitet kan ikke se dem)
                 ▼
            background.js (service worker: højreklik, genveje, ikon)
                 │
                 ▼
            offscreen.html (afspiller lyd, optager mikrofon)
                 │ fetch med Origin chrome-extension://<OpenLST-ID>
                 ▼
            OpenLST på 127.0.0.1:8765 → Piper / whisper.cpp
```

**Bygget i første version (0.1.0)**
- [x] Værktøjslinje på alle sider (vis/skjul med ikonet eller `Alt+Shift+O`): Læs, Pause/Fortsæt, Stop, Dikter, hastighed −/+
- [x] Læs markeret tekst – også via højreklik "Læs med OpenLST" og `Alt+Shift+L`
- [x] Klik-og-læs: uden markering læses det afsnit, der sidst er klikket i
- [x] Sætnings-fremhævning direkte på siden med CSS Custom Highlight API (sidens DOM ændres ikke)
- [x] Diktering ind i `input`, `textarea` og `contenteditable` ved cursor (fokus bevares i feltet)
- [x] Mikrofon optages i extensionens offscreen-dokument – websitet får aldrig adgang til lyden
- [x] Opsætningsside: tjek af OpenLST + mikrofontilladelse
- [x] Fast extension-ID via `key` i manifestet; serveren tillader kun dette ID (+ evt. ID'er i config)
- [x] Minimale rettigheder: `contextMenus`, `storage`, `offscreen` og kun `host_permissions` til 127.0.0.1:8765

**Næste trin mod IntoWords-niveau**
- [ ] 🎤-knap ved siden af det aktive tekstfelt (ikke kun i værktøjslinjen)
- [ ] Ord-for-ord-fremhævning (kræver ord-timings fra TTS)
- [ ] Læs mens du skriver: læs ord/sætning op, når eleven skriver mellemrum/punktum
- [ ] Ordforslag (lokal dansk ordliste/sprogmodel – ingen cloud)
- [ ] Læs PDF'er i browseren (PDF.js-visning med samme værktøjslinje)
- [ ] Microsoft 365 web (Word/OneNote/Outlook online): test indsættelse og læsning i deres editorer
- [ ] Google Docs (canvas-baseret – kræver særlig håndtering)
- [ ] Læs billeder/skærmtekst (lokal OCR)
- [ ] Indstillinger: farve på fremhævning, skrift, stemmevalg
- [ ] Udgivelse i Edge Add-ons og Chrome Web Store (WorkingOnIT); skole-IT kan tvangsinstallere via Intune/GPO

### M9 – Bedre dansk diktering: danske modeller og hardwareacceleration (NPU)

Forudsætning: M0–M8 virker godt på Surface Go **uden** NPU. Det er og bliver minimumskravet; alt i M9 er tilvalg, der aktiveres automatisk, når hardwaren findes.

**Del A – dansk-trænede Whisper-modeller (først, hjælper alle maskiner)**
- Problem: Whisper Tiny/Base er svage på dansk (benchmark på syntetisk tale: Tiny Q5 79 % WER, Base Q5 43 %).
- Undersøg dansk fine-tunede Whisper-modeller (fx fra Alexandra Instituttets CoRal-projekt): kvalitet, størrelse, licens, og om de kan konverteres til ggml/kvantiseres til whisper.cpp.
- Sammenlign med benchmark på rigtige danske børneoptagelser (med samtykke) – ikke kun syntetisk tale.
- [ ] Liste over kandidater med licens
- [ ] Konvertering + kvantisering (Q5/Q8)
- [ ] Benchmark på Surface Go: WER og ventetid

**Del B – NPU som "hurtig profil" (senere)**
- Gevinst primært for **diktering**: en større/bedre model (Small/Medium/dansk) kan køre uden at belaste CPU'en, og med lavere strømforbrug. Oplæsning (Piper) får lille gevinst – den er allerede hurtig, og VITS' dynamiske input passer dårligt til NPU'er.
- Ny motor bag det eksisterende `STTEngine`-interface; whisper.cpp på CPU forbliver standard og fallback.
- Ved opstart: registrér hardware → vælg hurtigste motor, der virker → fald tilbage til CPU ved fejl. Benchmarken sammenligner motorerne.
- Kandidater (skal verificeres i prototype):
  - Intel Core Ultra (AI Boost): OpenVINO (whisper.cpp's OpenVINO-encoder eller OpenVINO GenAI's Whisper-pipeline)
  - Qualcomm Snapdragon X: ONNX Runtime + QNN
  - AMD Ryzen AI: ONNX Runtime + Vitis AI
  - Windows ML (Windows 11): kan vælge backend pr. maskine – undersøg om det erstatter tre separate integrationer
- Første prototype: én leverandør (sandsynligvis Intel Core Ultra). Kræver en testmaskine.
- Runtime-pakker pr. leverandør installeres kun, hvor de bruges (installerens størrelse).
- Alt kører stadig 100 % lokalt; ingen ændring i privatliv eller sikkerhed.

## 7. Benchmark (sektion 19)

- `openlst.exe bench --stt-models tiny-q5_1,tiny-q8_0,base-q5_1 --audio testdata/*.wav --runs 5`
- Måler: model load time, TTS-generationstid pr. tegn, STT-tid, lydvarighed, RTF, peak RAM (Windows: `PeakWorkingSetSize` via `GetProcessMemoryInfo` på subprocess), CPU-tid (user+kernel).
- Output: Markdown-tabel + CSV i `bench-results/`. Ikke synlig i børne-UI.
- Testdata: få korte danske optagelser (egne, med samtykke) + forventet tekst → også WER.

## 8. Licenser – skal verificeres før distribution

| Komponent | Forventet | Bemærkning / risiko |
|---|---|---|
| whisper.cpp | MIT | lav risiko |
| Whisper-weights (OpenAI) | MIT | lav risiko |
| Piper (rhasspy/piper, arkiveret) | MIT | Ny udvikling (OHF-Voice/piper1-gpl) er **GPL-3.0** – brug MIT-udgaven eller vurdér |
| piper-phonemize / **espeak-ng** | **GPL-3.0** | Bundles i piper.exe. Distribution som separat program (subprocess) = aggregat → egen kode kan forblive MIT, men installer skal medføre GPL-tekst + kildehenvisning |
| ONNX Runtime | MIT | lav risiko |
| `da_DK-talesyntese-medium` | **ukendt – tjek MODEL_CARD** | Datasættets licens afgør om modellen må redistribueres |
| Skrifttype (fx Atkinson Hyperlegible) | SIL OFL | ok |

Resultat dokumenteres i `THIRD_PARTY_LICENSES.md`.

## 9. Risici

| Risiko | Afbødning |
|---|---|
| STT-latency på 4415Y for høj | Mål tidligt (M2 på rigtig hardware). Fallback: kortere optagelser, VAD-trim af stilhed, `-t 2`, greedy decoding, evt. `-ac` reduceret audio context |
| Tiny-modellens danske nøjagtighed er svag | Benchmark tiny vs base; dansk fine-tunede Whisper-modeller og evt. NPU på nyere maskiner (M9) |
| RAM-pres med Edge + 4 GB | Lazy-start whisper, idle-unload, Piper medium (~60 MB) |
| Antivirus/SmartScreen blokerer ukendt exe | Store-distribution (MSIX); ellers Trusted Signing/SignPath + Defender-indsendelse (§2a) |
| AppLocker/WDAC på skole-PC'er blokerer `%LOCALAPPDATA%` | Publisher-regel via signering; per-machine pakke til Intune (§2a.3) |
| Mikrofonadgang | `localhost` regnes som secure context → `getUserMedia` virker |

## 10. Udviklingsmiljø

Udvikling sker på macOS (Go, Piper og whisper.cpp findes alle til macOS). Surface Go bruges til M2-måling, acceptance-tests og benchmarks. Tidlig hardware-måling er vigtigste enkelt-risikoreduktion.
