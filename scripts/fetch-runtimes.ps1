# Udviklings-runtimes til Windows (færdigbyggede). Release-builds bygger
# piper og whisper.cpp fra kildekode i CI og signerer dem (beslutning #014).
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$tmp = Join-Path $env:TEMP 'openlst-runtimes'
New-Item -ItemType Directory -Force $tmp, runtime | Out-Null

# Piper (MIT-udgaven, rhasspy/piper)
$p = Join-Path $tmp 'piper.zip'
Invoke-WebRequest 'https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_windows_amd64.zip' -OutFile $p -UseBasicParsing
Write-Host "piper sha256: $((Get-FileHash $p).Hash)"
Expand-Archive $p -DestinationPath runtime -Force   # -> runtime\piper\piper.exe

# whisper.cpp CPU-build (x64)
$w = Join-Path $tmp 'whisper.zip'
Invoke-WebRequest 'https://github.com/ggml-org/whisper.cpp/releases/download/v1.9.2/whisper-bin-x64.zip' -OutFile $w -UseBasicParsing
Write-Host "whisper sha256: $((Get-FileHash $w).Hash)"
$wd = Join-Path $tmp 'whisper'
Expand-Archive $w -DestinationPath $wd -Force
New-Item -ItemType Directory -Force runtime\whisper | Out-Null
Get-ChildItem $wd -Recurse -Include whisper-cli.exe, *.dll | Copy-Item -Destination runtime\whisper -Force
Write-Host 'TODO: fastlås SHA-256 for runtimes, når versionerne er verificeret på Surface Go.'
