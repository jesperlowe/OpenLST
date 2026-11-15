# Lokalt API

Base: `http://localhost:8765` (lytter kun på `127.0.0.1`).

Adgang: kun fra OpenLST's egen side eller godkendte browser-extensions (se [SECURITY.md](SECURITY.md)). Alle andre får `403`.

## `GET /api/status`

```json
{"status":"ready","tts":true,"stt":true,"app":"OpenLST","version":"dev"}
```

`status` er `degraded`, hvis TTS eller STT mangler.

## `POST /api/tts`

`Content-Type: application/json`

```json
{"text":"Jeg cykler i skole i morgen.","speed":0.9,"language":"da"}
```

- `text`: 1–2000 tegn. Brugerfladen sender én sætning ad gangen.
- `speed`: 0.6–1.4 (standard 0.9). Omsættes til Piper `length_scale = 1/speed`.
- `language`: `da` eller udeladt.

Svar: `audio/wav` (22050 Hz mono for standardstemmen).

## `POST /api/stt`

`Content-Type: audio/wav` – 16 kHz, mono, 16-bit PCM, højst 90 s.

```json
{"text":"Jeg cykler i skole i morgen.","language":"da"}
```

## Fejl

```json
{"error":"Teksten er tom eller for lang."}
```

| Kode | Betydning |
|---|---|
| 400 | Ugyldigt input |
| 403 | Afsender ikke tilladt |
| 413 | For stor/lang optagelse |
| 415 | Forkert Content-Type |
| 503 | Motor ikke installeret |
