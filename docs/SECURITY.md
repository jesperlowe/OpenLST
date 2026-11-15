# Sikkerhed og privatliv

## Privatliv

- Ingen tekst, lyd eller transskription forlader computeren.
- Ingen telemetry, analytics eller eksterne kald. Siden har en Content-Security-Policy, der kun tillader egen adresse.
- Indhold logges aldrig – kun længde og tidsforbrug.
- Midlertidige lydfiler ligger i private temp-mapper og slettes straks efter brug, også ved fejl.
- `privacy_test.go` fejler, hvis koden indeholder adresser uden for computeren.

## Adgang til det lokale API

Enhver hjemmeside i browseren kan sende requests til `127.0.0.1`. Derfor:

1. Servicen lytter kun på `127.0.0.1:8765` – aldrig `0.0.0.0`.
2. `Host` skal være `localhost:8765` eller `127.0.0.1:8765` (beskytter mod DNS-rebinding).
3. `/api/*` kræver en `Origin` fra allowlisten: egen side, OpenLST's egen extension (fast ID `bmhpeociclipbhackmagnbakkmipbhpn`) og evt. ekstra ID'er i `config.json`. Ingen wildcard-CORS.
4. Browseren sætter `Origin` selv; hverken hjemmesider eller andre extensions kan forfalske den. Derfor bruges ingen token (beslutning #031).
5. `Content-Type` valideres (JSON / WAV), så simple cross-site formular-requests afvises.
6. Grænser for størrelse og varighed på input.
7. whisper.cpp køres som kommando pr. diktering – ikke som server – så der ikke åbnes flere porte.

## Browser-extension

- Kun `host_permissions` til `127.0.0.1:8765` / `localhost:8765`; ingen `tabs`-, `history`- eller `<all_urls>`-host-rettighed.
- Indholdsscriptet kalder aldrig OpenLST direkte; alt går via extensionens service worker og offscreen-dokument.
- Mikrofonen optages i offscreen-dokumentet – hjemmesiden får aldrig adgang til lyden.
- Fremhævning bruger CSS Custom Highlight API og ændrer ikke sidens indhold.

## Ingen administratorrettigheder

OpenLST installeres og kører som almindelig bruger. Binding til `127.0.0.1` udløser ingen firewall-dialog.

## Rapportér sårbarheder

Kontakt WorkingOnIT privat (ikke via offentlige issues).
