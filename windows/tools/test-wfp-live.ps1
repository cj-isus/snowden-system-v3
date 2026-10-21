# Elevated destructive-to-connectivity smoke for the persistent WFP policy.
# The test always removes its owned provider in defer and verifies that direct
# connectivity returns. Run from windows/: powershell -ExecutionPolicy Bypass
# -File tools\test-wfp-live.ps1
param([switch]$Elevated)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root "build\bin"
$testExe = Join-Path $bin "wfp-live.test.exe"
$output = Join-Path $bin "wfp-live.out.txt"

if (-not $Elevated) {
    Push-Location $root
    try {
        go test -c -tags "live,with_utls,with_gvisor,with_quic,with_clash_api" -o $testExe .
        if ($LASTEXITCODE -ne 0) { throw "live test compilation failed" }
    } finally { Pop-Location }

    Remove-Item -LiteralPath $output -Force -ErrorAction SilentlyContinue
    $args = @(
        "-NoProfile", "-ExecutionPolicy", "Bypass",
        "-File", $PSCommandPath, "-Elevated"
    )
    $process = Start-Process powershell.exe -Verb RunAs -Wait -PassThru -ArgumentList $args
    if (Test-Path -LiteralPath $output) { Get-Content -LiteralPath $output }
    exit $process.ExitCode
}

& $testExe '-test.v' '-test.run=^TestLiveWFP(BasePolicy|TUNRouteAndCrashBlock)$' '-test.timeout=180s' *> $output
exit $LASTEXITCODE
