---
name: windows-installer
description: Brug når der arbejdes med OpenLST's Windows-distribution – MSIX/Microsoft Store, winget, Inno Setup OpenLST-Setup.exe, kodesignering, SBOM/attestation, installation uden admin, autostart, AppLocker/Intune, krydskompilering af Go eller test på Surface Go.
---

# Windows-distribution: uden admin, signeret og betroet

## Ufravigelige krav
- **Aldrig admin.** Ingen UAC-prompt ved installation, drift, opdatering eller afinstallation. Exe-manifest: `requestedExecutionLevel level="asInvoker"`.
- Forbudt: skrivning til `Program Files`, `HKLM`, `C:\Windows`; Windows-services; `netsh` (urlacl/firewall); Scheduled Tasks med højeste rettigheder; binding til andet end `127.0.0.1` (udløser firewall-prompt der kræver admin).
- **Alt signeret** (Authenticode SHA-256 + RFC3161-tidsstempel): openlst.exe, piper.exe, whisper-server.exe, alle DLL'er, MSIX og Setup.exe.
- **Kun CI-builds distribueres.** Ingen binærer bygget på en udviklermaskine.
- Ingen egen auto-updater.

## Testbuild (findes nu)
`scripts/build-windows.sh` (kører på macOS): henter piper/whisper-releases (SHA-256-låst), samler `build/win/OpenLST/`, bygger `openlst.exe` (`-H windowsgui`, log til `%LOCALAPPDATA%\OpenLST\openlst.log`, fejl via MessageBox) og `installer/setup` (Go-installer med `//go:embed all:payload`) → `dist/OpenLST-Setup-<v>.exe`. Logik i `backend/install/` (Extract med zip-slip-beskyttelse, PowerShell-genveje via env-vars, HKCU Uninstall-nøgle, `--uninstall` med rå `SysProcAttr.CmdLine` til `cmd.exe`).
- ⚠️ Færdigbyggede piper/whisper kræver VC++ runtime (msvcp140, vcruntime140(_1), vcomp140) → `install.MissingVCRuntime()` advarer. Release: byg med `/MT` (#034).
- ⚠️ Pentium 4415Y har ikke AVX: whisper.cpp-releasen har `ggml-cpu-sse42.dll`/`ggml-cpu-x64.dll` og vælger ved runtime – medtag ALLE `ggml-cpu-*.dll`.
- C++-programmer får relative stier (`procutil.RelArg`, opløser symlinks) pga. æ/ø/å i brugernavne.
- Manifest/ikon/versionsinfo i exe mangler (kræver go-winres e.l. i CI). 64-bit exe udløser ikke installer detection.

## Build (GitHub Actions, windows-latest, trigger: tag)
1. Go: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" ./backend/cmd/openlst` (+ konsol-build til bench).
2. piper og whisper.cpp bygges fra fastlåst commit (CMake, CPU-only, AVX2 – verificér målhardware) – ikke hentet som færdige zip-filer i release-builds.
3. Modeller hentes og verificeres mod SHA-256 i repo.
4. Signér alle PE-filer → pak MSIX (`makeappx`) og Inno Setup (`ISCC`) → signér pakkerne.
5. SBOM (`syft` / `cyclonedx-gomod`) + `actions/attest-build-provenance` + SHA256SUMS.
6. Ingen UPX/packers (giver AV-false positives).

## Signering
- Microsoft Store-MSIX: Microsoft signerer ved indsendelse.
- Uden for Store: Azure Trusted Signing (`azure/trusted-signing-action`) eller SignPath Foundation (gratis til OSS). Nøgle aldrig i repo/secrets som PFX-fil, hvis det kan undgås.
- Verificér: `Get-AuthenticodeSignature .\openlst.exe` → Status `Valid` for hver fil (CI-step der fejler ellers).

## MSIX (primær)
- Full-trust desktop-app (`runFullTrust`) – loopback-begrænsningen gælder kun AppContainer, ikke full trust.
- Autostart: `desktop:Extension Category="windows.startupTask"` (brugeren kan slå fra i Jobliste).
- Pakkemappen er skrivebeskyttet: modeller i pakken ok; config/temp i `%LOCALAPPDATA%\OpenLST` (virtualiseres til pakkens LocalCache).
- winget-manifest peger på Store-ID eller signeret MSIX.

## Inno Setup (alternativ: OpenLST-Setup.exe)
- `PrivilegesRequired=lowest`, `PrivilegesRequiredOverridesAllowed=` (tomt – ingen elevation-dialog), `DefaultDirName={localappdata}\Programs\OpenLST`.
- `ArchitecturesAllowed=x64compatible`, `ArchitecturesInstallIn64BitMode=x64compatible`.
- `SignTool=` + `SignedUninstaller=yes`.
- `[Icons]`: `{userprograms}\OpenLST`, `{userstartup}\OpenLST` (`--background`).
- `[Run]` postinstall nowait → åbner `http://localhost:8765`. `[UninstallRun]`: `openlst.exe --shutdown`.
- Medtag `LICENSE`, `THIRD_PARTY_LICENSES.md`, GPL-tekst for espeak-ng.

## Layout
```
<installmappe>\openlst.exe, runtime\piper\, runtime\whisper\, models\
%LOCALAPPDATA%\OpenLST\config.json   (token, stemme, models_dir) – kun brugerens ACL
```

## openlst.exe opførsel
- Single-instance via `GET 127.0.0.1:8765/api/status`; svarer OpenLST → åbn browser og afslut. Svarer noget andet på porten → vis fejl, kapr ikke porten.
- Stier relativt til `os.Executable()`, aldrig cwd; subprocesser startes med absolut sti (undgå DLL/exe-hijacking via PATH/cwd).
- Subprocesser i Job Object (`KILL_ON_JOB_CLOSE`), `CREATE_NO_WINDOW`.
- Åbn browser: `rundll32 url.dll,FileProtocolHandler http://localhost:8765`.

## Skole-IT
- AppLocker blokerer ofte `%LOCALAPPDATA%` → signering muliggør publisher-regel. Lever per-machine MSIX til Intune.
- `docs/IT-ADMIN.md`: udgivernavn, hashes, kun loopback-trafik, dataflow, signatur- og attestation-verifikation (`gh attestation verify`).

## Test (Surface Go, standardbrugerkonto – ikke admin-konto)
- Installér, kør, afinstallér: ingen UAC-prompt, ingen firewall-dialog, ingen SmartScreen-blokering.
- `netstat -ano | findstr 8765` → kun `127.0.0.1:8765`.
