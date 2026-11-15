---
name: browser-extension
description: Brug når der arbejdes på OpenLST's Edge/Chrome-extension i extension/ – Manifest V3, kontekstmenu "Læs med OpenLST", diktering i input/textarea/contenteditable, offscreen documents, pairing med localhost-servicen.
---

# Edge/Chrome-extension (MV3) – milestone 8

## Struktur
```
extension/
  manifest.json        # MV3
  background.js        # service worker: kontekstmenu, API-kald, token
  offscreen.html/.js   # lydafspilning + mikrofonoptagelse (service worker har ingen DOM/audio)
  content.js           # 🎤-knap ved fokuseret felt, indsættelse ved cursor
  setup.html/.js       # tjek af OpenLST + mikrofontilladelse (åbnes ved installation)
  lib/                 # KOPIER af frontend/sentences.js, wav.js, recorder-worklet.js – rediger i frontend/ og kør scripts/sync-extension.sh
```

## Manifest
- `permissions`: kun `contextMenus`, `storage`, `offscreen` (testet i `extension_sync_test.go`). Ingen `tabs` – tab-URL'er er derfor `null` i service workeren.
- `host_permissions`: kun `http://127.0.0.1:8765/*` og `http://localhost:8765/*`.
- Fast extension-ID `bmhpeociclipbhackmagnbakkmipbhpn` via `key` i manifest. Privat nøgle ligger uden for repo: `~/.openlst/extension-key.pem` (opbevares af WorkingOnIT).
- Fremhævning: CSS Custom Highlight API (`::highlight(openlst-sentence)` i `content.css`) – ændrer ikke sidens DOM.
- Værktøjslinjen: lukket shadow DOM + `adoptedStyleSheets` (ikke blokeret af sidens CSP). Knapper har `mousedown.preventDefault()`, så fokus bliver i sidens tekstfelt.
- Test: headless Edge med `--load-extension` virker (Chrome 137+ har fjernet flaget). Lyd kan ikke testes headless på macOS (lyd-uret står stille).

## Flow
- Læs: `contextMenus.onClicked` → `selectionText` → `fetch('/api/tts', {headers:{'X-OpenLST-Token':…}})` → send WAV til offscreen document → afspil.
- Diktér: content script viser 🎤 ved fokus på `input[type=text|search|email]`, `textarea`, `[contenteditable]`. Optagelse sker i offscreen document (`reasons:['USER_MEDIA']`), ikke i sidens kontekst – websitet får aldrig lyden.
- Indsæt ved cursor:
  - input/textarea: `el.setRangeText(text, el.selectionStart, el.selectionEnd, 'end')` + `el.dispatchEvent(new Event('input',{bubbles:true}))` (React/Vue).
  - contenteditable: `document.execCommand('insertText', false, text)` (deprecated men eneste der respekterer undo og framework-editors).
- Gem fokus/selection før optagelse og gendan før indsættelse.

## Sikkerhed
- Content scripts må ikke kalde localhost direkte (deres Origin er websitet) – alt går via `chrome.runtime.sendMessage` → service worker.
- Se også localhost-security-skill.

## Microsoft 365 web
Word/Outlook Online bruger egne editorer (ikke ren contenteditable); `execCommand('insertText')` virker ofte, men skal testes. Fallback: kopiér til udklipsholder + vis "Tryk Ctrl+V".
