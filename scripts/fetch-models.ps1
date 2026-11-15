# Henter danske modeller og verificerer SHA-256. Kør fra repo-roden.
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$piper = 'https://huggingface.co/rhasspy/piper-voices/resolve/main/da/da_DK/talesyntese/medium'
$whisper = 'https://huggingface.co/ggerganov/whisper.cpp/resolve/main'
New-Item -ItemType Directory -Force models\tts, models\stt | Out-Null
$files = @{
  "$piper/da_DK-talesyntese-medium.onnx"      = 'models\tts\da_DK-talesyntese-medium.onnx'
  "$piper/da_DK-talesyntese-medium.onnx.json" = 'models\tts\da_DK-talesyntese-medium.onnx.json'
  "$whisper/ggml-tiny-q5_1.bin"               = 'models\stt\ggml-tiny-q5_1.bin'
}
foreach ($u in $files.Keys) { if (-not (Test-Path $files[$u])) { Invoke-WebRequest $u -OutFile $files[$u] -UseBasicParsing } }
foreach ($line in Get-Content scripts\models.sha256) {
  $hash, $path = $line -split '\s+', 2
  $actual = (Get-FileHash $path -Algorithm SHA256).Hash.ToLower()
  if ($actual -ne $hash) { throw "SHA-256 passer ikke: $path" }
  Write-Host "OK  $path"
}
