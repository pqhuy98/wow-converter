# Build production Go wow-converter bundle to dist-go/.
# Run from repo root: npm run build:go-app
#
# Includes: Go binary, webui, native bin/, resources/ (incl. map template).
# Excludes: Node deps, blp-preview bindings, MSVC link artifacts, debug DLLs.

param(
    [switch]$SkipWebUI
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$distGo = Join-Path $root "dist-go"
$isWindows = ($IsWindows -or $env:OS -match "Windows")

function Ensure-Dir([string]$Path) {
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
}

function Copy-IfExists([string]$Source, [string]$Destination) {
    if (-not (Test-Path $Source)) {
        Write-Warning "Skipping missing: $Source"
        return
    }
    $parent = Split-Path -Parent $Destination
    if ($parent) { Ensure-Dir $parent }
    Copy-Item -Force $Source $Destination
}

if (-not $SkipWebUI) {
    if (-not (Test-Path "webui/out/index.html")) {
        throw "webui/out is missing. Run 'npm run build:webui' first, or run 'npm run build:go-app' from package.json."
    }
}

Write-Host "Preparing dist-go..."
if (Test-Path $distGo) {
    Remove-Item -Recurse -Force $distGo
}
Ensure-Dir $distGo

$exeName = if ($isWindows) { "wow-converter.exe" } else { "wow-converter" }
Write-Host "Building wow-converter (Go) -> dist-go/$exeName"
Push-Location golang
go build -ldflags "-s -w" -o (Join-Path $distGo $exeName) ./cmd/wow-converter
if ($LASTEXITCODE -ne 0) { Pop-Location; throw "go build failed" }
Pop-Location

Write-Host "Copying webui/out..."
Ensure-Dir (Join-Path $distGo "webui")
Copy-Item -Recurse -Force "webui/out" (Join-Path $distGo "webui/out")

Write-Host "Copying bin/ (runtime only)..."
$binDest = Join-Path $distGo "bin"
Ensure-Dir (Join-Path $binDest "blp-native")
if ($isWindows) {
    Copy-IfExists "bin/blp-native/blpencode.dll" (Join-Path $binDest "blp-native/blpencode.dll")
} else {
    Copy-IfExists "bin/blp-native/libblpencode.so" (Join-Path $binDest "blp-native/libblpencode.so")
}
Copy-IfExists "bin/azerothcore-world.sqlite" (Join-Path $binDest "azerothcore-world.sqlite")

Ensure-Dir (Join-Path $binDest "upscayl/models")
Copy-Item -Force "bin/upscayl/models/*" (Join-Path $binDest "upscayl/models")
if ($isWindows) {
    Ensure-Dir (Join-Path $binDest "upscayl/win")
    Copy-IfExists "bin/upscayl/win/upscayl-bin.exe" (Join-Path $binDest "upscayl/win/upscayl-bin.exe")
    Copy-IfExists "bin/upscayl/win/vcomp140.dll" (Join-Path $binDest "upscayl/win/vcomp140.dll")
} else {
    Ensure-Dir (Join-Path $binDest "upscayl/linux")
    Copy-IfExists "bin/upscayl/linux/upscayl-bin" (Join-Path $binDest "upscayl/linux/upscayl-bin")
}

Write-Host "Copying resources/..."
Copy-Item -Recurse -Force "resources" (Join-Path $distGo "resources")
Copy-Item -Recurse -Force "maps/template-empty.w3x" (Join-Path $distGo "resources/template-empty.w3x")

Write-Host ""
Write-Host "Done: dist-go/"
Write-Host "  Run: .\dist-go\$exeName"
