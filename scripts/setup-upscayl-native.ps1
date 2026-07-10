param(
    [string]$SourceRoot = (Join-Path (Split-Path -Parent $PSScriptRoot) "node_modules\upscayl-node\dist\upscaler\sub-classes")
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$outRoot = Join-Path $repoRoot "bin\upscayl"
$modelsSrc = Join-Path $SourceRoot "model-manager\models"
$winSrc = Join-Path $SourceRoot "driver\command-upscayl\resources\win\bin"
$linuxSrc = Join-Path $SourceRoot "driver\command-upscayl\resources\linux\bin"

if (-not (Test-Path $modelsSrc)) {
    throw "upscayl-node models not found at $modelsSrc. Run npm install first."
}

New-Item -ItemType Directory -Force -Path (Join-Path $outRoot "win"), (Join-Path $outRoot "linux"), (Join-Path $outRoot "models") | Out-Null

Copy-Item -Force (Join-Path $modelsSrc "*.bin") (Join-Path $outRoot "models")
Copy-Item -Force (Join-Path $modelsSrc "*.param") (Join-Path $outRoot "models")

if (Test-Path $winSrc) {
    Copy-Item -Force (Join-Path $winSrc "*") (Join-Path $outRoot "win")
}

if (Test-Path $linuxSrc) {
    Copy-Item -Force (Join-Path $linuxSrc "upscayl-bin") (Join-Path $outRoot "linux")
}

Write-Host "Installed upscayl runtime to $outRoot"
