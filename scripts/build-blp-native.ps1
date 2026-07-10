param(
    [string]$Configuration = "Release"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$standalone = Join-Path $repoRoot "node-images-blp\standalone"
$thirdParty = Join-Path $repoRoot "node-images-blp\gyp\third-party"
$outDir = Join-Path $repoRoot "bin\blp-native"
$buildDir = Join-Path $standalone "build-msvc"

New-Item -ItemType Directory -Force -Path $outDir, $buildDir | Out-Null

$vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
if (-not (Test-Path $vswhere)) {
    throw "Visual Studio vswhere.exe not found. Install VS Build Tools or use Linux build-blp-native.sh."
}

$vsPath = & $vswhere -latest -property installationPath
$vcvars = Join-Path $vsPath "VC\Auxiliary\Build\vcvars64.bat"
if (-not (Test-Path $vcvars)) {
    throw "vcvars64.bat not found at $vcvars"
}

$commonCFlags = @(
    "/nologo", "/EHsc", "/O2", "/MD", "/W3",
    "/DHAVE_PNG", "/DHAVE_BLP", "/DBLPENCODE_EXPORTS",
    "/I`"$standalone`"",
    "/I`"$thirdParty\libpng`"",
    "/I`"$thirdParty\zlib`""
)

$cppSources = @(
    (Join-Path $standalone "pixel_array.cc"),
    (Join-Path $standalone "blp_encode_api.cc"),
    (Join-Path $standalone "blp_codecs.cc")
)

$zlibSources = Get-ChildItem (Join-Path $thirdParty "zlib\*.c") | Where-Object { $_.Name -ne "example.c" } | ForEach-Object { $_.FullName }
$pngSources = Get-ChildItem (Join-Path $thirdParty "libpng\*.c") | Where-Object { $_.Name -ne "pngtest.c" } | ForEach-Object { $_.FullName }
$cSources = $zlibSources + $pngSources

$objects = @()
foreach ($src in $cppSources) {
    $obj = Join-Path $buildDir (([IO.Path]::GetFileNameWithoutExtension($src)) + ".obj")
    $objects += $obj
    $compileCmd = "cl $($commonCFlags -join ' ') /c `"$src`" /Fo`"$obj`""
    cmd.exe /c "call `"$vcvars`" >nul && $compileCmd"
    if ($LASTEXITCODE -ne 0) { throw "Failed compiling $src" }
}

foreach ($src in $cSources) {
    $obj = Join-Path $buildDir (([IO.Path]::GetFileNameWithoutExtension($src)) + ".obj")
    $objects += $obj
    $compileCmd = "cl $($commonCFlags -join ' ') /TC /c `"$src`" /Fo`"$obj`""
    cmd.exe /c "call `"$vcvars`" >nul && $compileCmd"
    if ($LASTEXITCODE -ne 0) { throw "Failed compiling $src" }
}

$dllPath = Join-Path $outDir "blpencode.dll"
$libPath = Join-Path $outDir "blpencode.lib"
$linkObjs = ($objects | ForEach-Object { "`"$_`"" }) -join " "
$linkCmd = "link /nologo /DLL /OUT:`"$dllPath`" /IMPLIB:`"$libPath`" $linkObjs"
cmd.exe /c "call `"$vcvars`" >nul && $linkCmd"
if ($LASTEXITCODE -ne 0) { throw "Failed linking $dllPath" }

Write-Host "Built $dllPath"
