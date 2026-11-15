---
name: localhost-security
description: Brug ved ændringer i OpenLST's HTTP-server, middleware, CORS, Origin/Host-checks, extension-pairing-token eller alt der påvirker hvem der kan kalde localhost:8765. Også privacy-regler (ingen data forlader maskinen).
---

# Localhost-sikkerhed og privatliv

## Trusselsmodel
Enhver hjemmeside i brugerens browser kan sende requests til `127.0.0.1:8765`. Uden beskyttelse kan et website få oplæst/transskriberet indhold eller misbruge ressourcer. DNS-rebinding kan omgå same-origin.

## Krav (skal altid holde)
1. `Listen("tcp", "127.0.0.1:8765")` – aldrig `0.0.0.0`, `:8765` eller `[::]`.
2. Host-header ∈ {`localhost:8765`, `127.0.0.1:8765`} ellers 421/403 (DNS-rebinding).
3. `/api/*`: Origin ∈ allowlist {`http://localhost:8765`, `http://127.0.0.1:8765`, `chrome-extension://<id>` fra config}. Ellers 403. POST uden Origin → 403. `GET /api/status` uden Origin må tillades (curl/diagnostik) men skal ikke afsløre andet end status.
4. CORS: echo kun en tilladt Origin + `Vary: Origin`. Aldrig `Access-Control-Allow-Origin: *`. Ingen `Allow-Credentials`.
5. Private Network Access: svar på `Access-Control-Request-Private-Network: true` med `Access-Control-Allow-Private-Network: true` kun for tilladte Origins.
6. **Ingen token** (beslutning #031): browseren sætter `Origin` på extension-kald, og den kan ikke forfalskes af websites eller andre extensions. OpenLST's eget extension-ID (`config.ExtensionID`, fra `key` i `extension/manifest.json`) er altid tilladt; ekstra ID'er via `extension_ids` i config. `extension_sync_test.go` tjekker at manifest-ID og server-ID passer.
7. Body-grænser: `http.MaxBytesReader` (tekst ~20 KB, lyd ~2 MB). Timeouts på server (Read/Write/Idle).
8. Content-Type valideres (`application/json` til TTS, `audio/wav` til STT) – undgår "simple request"-CSRF.

## Privatliv
- Ingen udgående netværkskald fra backend eller frontend. Ingen CDN, ingen Google Fonts, ingen analytics. Alle assets embedded.
- Log aldrig tekst, lyd eller transskription.
- Test: Go-test der fejler hvis kildekoden indeholder `http://`/`https://` URL'er ud over localhost-allowlist (undtagen docs/scripts).

## Test hver ændring med
```sh
curl -i -X POST -H 'Origin: https://evil.example' -H 'Content-Type: application/json' -d '{"text":"x"}' http://127.0.0.1:8765/api/tts   # forvent 403
curl -i -H 'Host: evil.example:8765' http://127.0.0.1:8765/api/status                                                                      # forvent 403/421
```
