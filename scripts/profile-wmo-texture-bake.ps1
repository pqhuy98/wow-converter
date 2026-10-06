param(
    [string]$Wmo = 'world\wmo\expansion11\troll\12tr_amani_hub03.wmo',
    [string]$OutputDirectory = ''
)

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$golangDir = Join-Path $repoRoot 'golang'
if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $OutputDirectory = Join-Path $repoRoot "tmp\texture-bake-performance\profiles\$stamp"
}
$OutputDirectory = [System.IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null

$oldWmo = $env:TEXTURE_BAKE_PROFILE_WMO
$oldOutput = $env:TEXTURE_BAKE_PROFILE_OUTPUT
$oldGoCache = $env:GOCACHE
try {
    $env:TEXTURE_BAKE_PROFILE_WMO = $Wmo
    $env:TEXTURE_BAKE_PROFILE_OUTPUT = Join-Path $OutputDirectory 'export'
    $env:GOCACHE = Join-Path $repoRoot 'tmp\go-cache'
    Push-Location $golangDir
    try {
        go test -tags=integration_tests ./internal/converter/wowmodel/direct/wmo `
            -run '^TestProfileWMOTextureBake$' -count=1 `
            -o (Join-Path $OutputDirectory 'profile.test.exe') `
            -cpuprofile (Join-Path $OutputDirectory 'cpu.pprof') `
            -memprofile (Join-Path $OutputDirectory 'heap.pprof') `
            -v
        if ($LASTEXITCODE -ne 0) {
            throw "go test failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }
}
finally {
    $env:TEXTURE_BAKE_PROFILE_WMO = $oldWmo
    $env:TEXTURE_BAKE_PROFILE_OUTPUT = $oldOutput
    $env:GOCACHE = $oldGoCache
}

Write-Host "Profiles and conversion artifacts: $OutputDirectory"

