# snowden.system -- packaging (Phase A3).
# Full cycle: tagged build -> NSIS installer.
# Usage:
#   powershell -ExecutionPolicy Bypass -File tools\package.ps1               # build + installer
#   powershell -ExecutionPolicy Bypass -File tools\package.ps1 -NoInstaller  # build only
#
# Guarantees:# - exe is built with the sing-box tags (with_utls,with_gvisor,with_quic,with_clash_api);
#   with_clash_api enables the read-only metrics controller (V2-048/F12); without
#   the tag the app starts the same way but metrics report unavailable.
#   - wintun.dll is placed next to the exe (TUN breaks on a clean machine without it);
#   - no secrets go into the package (vault lives in %AppData%; the installer
#     ships the exe + identical recovery copy + pinned native helper + wintun.dll).
# NOTE: keep this file pure ASCII (PLAN V2-022: PS5.1 reads UTF-8 w/o BOM as ANSI).

param(
    [switch]$NoInstaller
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot   # windows/
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw "This packaging entry point requires a Windows build host."
}

Write-Host "== snowden.system packaging ==" -ForegroundColor Cyan

# --- 1. Backend checks ---
Write-Host "[1/4] go test / go vet..." -ForegroundColor Yellow
Push-Location $root
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed" }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet failed" }
} finally { Pop-Location }

# --- 2. Tagged build ---
# NOTE: deliberately WITHOUT -nsis. The wails default installer is machine-scope
# and ships only the exe (no wintun.dll -> TUN breaks on a clean machine).
# Our installer (build/windows/installer.nsi) is the single source of installers.
Write-Host "[2/4] wails build (tags + version ldflags)..." -ForegroundColor Yellow
Push-Location $root
try {
    # Build outside build/bin: Wails can replace that directory. The small
    # native helper imports only the observation codec, never the VPN engine.
    $probeStage = Join-Path $root ".artifacts\native-helper-build"
    [System.IO.Directory]::CreateDirectory($probeStage) | Out-Null
    $probeStaged = Join-Path $probeStage "snowden-native-probe.exe"
    # This package ships amd64 wintun; both executable targets must match it.
    # The codec-only helper needs no C toolchain. Restore caller environment
    # before Wails runs and on every helper build failure.
    $probePreviousGOOS = $env:GOOS
    $probePreviousGOARCH = $env:GOARCH
    $probePreviousCGO = $env:CGO_ENABLED
    try {
        $env:GOOS = "windows"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "0"
        go build -trimpath -o $probeStaged ./cmd/snowden-native-probe
        if ($LASTEXITCODE -ne 0) { throw "native helper build failed" }
    } finally {
        $env:GOOS = $probePreviousGOOS
        $env:GOARCH = $probePreviousGOARCH
        $env:CGO_ENABLED = $probePreviousCGO
    }
    $probeSHA = (Get-FileHash -LiteralPath $probeStaged -Algorithm SHA256).Hash.ToLowerInvariant()

    # F15 (V2-050): the running version must be machine-readable for the update
    # chain (strict-increase policy + anti-downgrade floor).
    $wailsJson0 = Get-Content (Join-Path $root "wails.json") -Raw | ConvertFrom-Json
    $appVersion = $wailsJson0.info.productVersion
    if (-not $appVersion) { $appVersion = "2.0.0" }
    $ldflags = "-X main.runningVersion=$appVersion -X main.nativeProbeHelperSHA256=$probeSHA"
    wails build -platform "windows/amd64" -tags "with_utls,with_gvisor,with_quic,with_clash_api" -ldflags "$ldflags"
    if ($LASTEXITCODE -ne 0) { throw "wails build failed" }
} finally { Pop-Location }

$bin = Join-Path $root "build\bin"
$exe = Join-Path $bin "snowden-system.exe"
if (-not (Test-Path $exe)) { throw "exe not found: $exe" }
$recovery = Join-Path $bin "snowden-recovery.exe"
# Protocol5 companion is an exact byte copy of this signed-application target,
# not a separately built EXE or an old backup. Hash equality is packaging
# identity only; the runtime still requires actual local update-role/SDK proof.
$recoveryAppSHA = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant()
Copy-Item -LiteralPath $exe -Destination $recovery -Force
if ((Get-FileHash -LiteralPath $recovery -Algorithm SHA256).Hash.ToLowerInvariant() -ne $recoveryAppSHA -or
    (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant() -ne $recoveryAppSHA) {
    throw "recovery companion copy changed during packaging"
}
$probe = Join-Path $bin "snowden-native-probe.exe"
if ((Get-FileHash -LiteralPath $probeStaged -Algorithm SHA256).Hash.ToLowerInvariant() -ne $probeSHA) {
    throw "native helper changed after main application pinning"
}
Copy-Item -LiteralPath $probeStaged -Destination $probe -Force
if ((Get-FileHash -LiteralPath $probe -Algorithm SHA256).Hash.ToLowerInvariant() -ne $probeSHA) {
    throw "packaged native helper differs from main application pin"
}

# --- 3. wintun.dll next to the exe ---
Write-Host "[3/4] wintun.dll..." -ForegroundColor Yellow
$wintun = Join-Path $bin "wintun.dll"
if (-not (Test-Path $wintun)) {
    throw "wintun.dll not found in build\bin - put official 0.14.1 amd64 there (see docs/INFRASTRUCTURE.md)"
}
Write-Host "     wintun.dll: $((Get-Item $wintun).Length) bytes"

# --- 4. NSIS installer (exe+recovery copy+native helper+wintun.dll, per-user) ---
if (-not $NoInstaller) {
    Write-Host "[4/4] NSIS installer..." -ForegroundColor Yellow

    # version from wails.json
    $wailsJson = Get-Content (Join-Path $root "wails.json") -Raw | ConvertFrom-Json
    $version = $wailsJson.info.productVersion
    if (-not $version) { $version = "2.0.0" }

    # makensis: PATH, the Wails cache, then the repository-local portable
    # toolchain used by repeatable builds. The latter is gitignored.
    $makensisPath = $null
    $makensis = Get-Command makensis -ErrorAction SilentlyContinue
    if ($makensis) {
        $makensisPath = $makensis.Source
    } else {
        $wailsDir = Join-Path $env:LOCALAPPDATA "wails"
        if (Test-Path $wailsDir) {
            $found = Get-ChildItem -Path $wailsDir -Recurse -Filter makensis.exe -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($found) { $makensisPath = $found.FullName }
        }
    }
    if (-not $makensisPath) {
        $repoToolDir = Join-Path (Split-Path -Parent $root) ".tmp\nsis"
        if (Test-Path $repoToolDir) {
            $found = Get-ChildItem -Path $repoToolDir -Recurse -Filter makensis.exe -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($found) { $makensisPath = $found.FullName }
        }
    }
    if (-not $makensisPath) {
        Write-Warning "makensis not found (PATH / %LOCALAPPDATA%\wails / .tmp\nsis). Install NSIS 3.x - the installer will build on the next run."
        Write-Host "     Build finished without installer: $exe"
        exit 0
    }

    $nsi = Join-Path $root "build\windows\installer.nsi"

    # NSIS reads non-BOM files as ANSI -> Cyrillic UI becomes mojibake.
    # Ensure UTF-8 BOM (same class of lesson as V2-022 for PS1).
    $nsiBytes = [System.IO.File]::ReadAllBytes($nsi)
    if ($nsiBytes.Length -lt 3 -or $nsiBytes[0] -ne 0xEF -or $nsiBytes[1] -ne 0xBB -or $nsiBytes[2] -ne 0xBF) {
        Write-Host "     installer.nsi has no UTF-8 BOM - adding it (NSIS needs it for Cyrillic)" -ForegroundColor DarkYellow
        [System.IO.File]::WriteAllBytes($nsi, [byte[]](0xEF,0xBB,0xBF) + $nsiBytes)
    }

    & $makensisPath "/DAPP_VERSION=$version" "/DEXE_PATH=$exe" "/DRECOVERY_PATH=$recovery" "/DPROBE_PATH=$probe" "/DWINTUN_PATH=$wintun" $nsi
    if ($LASTEXITCODE -ne 0) { throw "makensis failed" }

    # remove a stray wails default installer if a previous -nsis run left one
    $wailsDefault = Get-ChildItem -Path $bin -Filter "snowden.system-amd64-installer.exe" -ErrorAction SilentlyContinue
    if ($wailsDefault) {
        Remove-Item $wailsDefault.FullName -Force
        Write-Host "     removed stray wails default installer (superseded by our per-user one)" -ForegroundColor DarkYellow
    }

    $setup = Join-Path $bin "snowden-system-setup-$version.exe"
    Write-Host ""
    Write-Host "== Done ==" -ForegroundColor Green
    Write-Host "  exe:       $exe"
    if (Test-Path $setup) { Write-Host "  installer: $setup" }
}
