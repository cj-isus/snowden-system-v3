# Project-local, pinned public tools. No global installation, hooks or application dependency updates.
param([switch]$Apply, [switch]$GoTools, [switch]$OfficialGoProxy, [switch]$SourceGitleaks)
$ErrorActionPreference = 'Stop'
$taskProjectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$taskLock = Get-Content -LiteralPath (Join-Path $taskProjectRoot '.harness/tools.lock.json') -Raw | ConvertFrom-Json
if ($taskLock.schemaVersion -ne 1) { throw 'Unsupported tools lock' }
if (-not $Apply) {
    [pscustomobject]@{ applied=$false; platform=$taskLock.platform; downloads=$taskLock.downloads; goToolsRequested=[bool]$GoTools; sourceGitleaks=[bool]$SourceGitleaks; officialGoProxy=[bool]$OfficialGoProxy; goTools=$taskLock.goTools } | ConvertTo-Json -Depth 6
    return
}
if ($env:OS -ne 'Windows_NT' -or -not [Environment]::Is64BitProcess -or $env:PROCESSOR_ARCHITECTURE -ne 'AMD64') { throw 'Locked binaries require Windows amd64; choose reviewed platform assets before installation' }
function Resolve-TaskLocalPath([string]$RelativePath) {
    $taskCandidate = [System.IO.Path]::GetFullPath((Join-Path $taskProjectRoot $RelativePath))
    if (-not $taskCandidate.StartsWith($taskProjectRoot + [System.IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Path outside project' }
    $taskProbe = $taskCandidate
    while ($taskProbe -ne $taskProjectRoot) {
        if (Test-Path -LiteralPath $taskProbe) {
            if ((Get-Item -LiteralPath $taskProbe -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Reparse-point destination is not allowed' }
        }
        $taskProbe = Split-Path -Parent $taskProbe
    }
    return $taskCandidate
}
Add-Type -AssemblyName System.IO.Compression.FileSystem
$taskToolsDir = Resolve-TaskLocalPath '.tools'
[System.IO.Directory]::CreateDirectory($taskToolsDir) | Out-Null
$taskReceiptPath = Resolve-TaskLocalPath '.tools/installed.json'
$taskInstalled = if (Test-Path -LiteralPath $taskReceiptPath) { @((Get-Content -LiteralPath $taskReceiptPath -Raw | ConvertFrom-Json).tools) } else { @() }
function Save-TaskToolReceipt($Entry) {
    $script:taskInstalled = @($script:taskInstalled | Where-Object { $_.name -ne $Entry.name }) + @($Entry)
    $taskRecord = [pscustomobject]@{ installedAt=[DateTime]::UtcNow.ToString('o'); tools=$script:taskInstalled }
    [System.IO.File]::WriteAllText($taskReceiptPath, ($taskRecord | ConvertTo-Json -Depth 6), (New-Object System.Text.UTF8Encoding($false)))
}
foreach ($taskTool in $taskLock.downloads) {
    if ($SourceGitleaks -and $taskTool.name -eq 'gitleaks') { continue }
    if ($taskTool.name -notmatch '^[a-z0-9-]+$' -or $taskTool.entry -notmatch '^[a-z0-9-]+\.exe$') { throw 'Invalid tool name' }
    if ($taskTool.url -notmatch '^https://github\.com/[^/]+/[^/]+/releases/download/') { throw 'Only pinned official GitHub release assets are allowed' }
    $taskArchive = Resolve-TaskLocalPath ('.tools/' + $taskTool.name + '-' + $taskTool.version + '.zip')
    if (-not (Test-Path -LiteralPath $taskArchive) -or (Get-FileHash -LiteralPath $taskArchive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskTool.sha256) {
        Invoke-WebRequest -Uri $taskTool.url -OutFile $taskArchive -UseBasicParsing -TimeoutSec 180
    }
    if ((Get-FileHash -LiteralPath $taskArchive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskTool.sha256) { throw ('SHA256 mismatch: ' + $taskTool.name) }
    $taskBinDir = Resolve-TaskLocalPath ('.tools/' + $taskTool.name)
    [System.IO.Directory]::CreateDirectory($taskBinDir) | Out-Null
    $taskExe = Resolve-TaskLocalPath ('.tools/' + $taskTool.name + '/' + $taskTool.entry)
    $taskZip = [System.IO.Compression.ZipFile]::OpenRead($taskArchive)
    try {
        $taskEntry = $taskZip.GetEntry($taskTool.entry)
        if (-not $taskEntry) { throw ('Expected executable not found: ' + $taskTool.entry) }
        [System.IO.Compression.ZipFileExtensions]::ExtractToFile($taskEntry, $taskExe, $true)
        foreach ($taskLicenseName in @('LICENSE', 'LICENSE.txt')) {
            $taskLicense = $taskZip.GetEntry($taskLicenseName)
            if ($taskLicense) { [System.IO.Compression.ZipFileExtensions]::ExtractToFile($taskLicense, (Join-Path $taskBinDir $taskLicenseName), $true) }
        }
    } finally { $taskZip.Dispose() }
    Save-TaskToolReceipt ([pscustomobject]@{ name=$taskTool.name; version=$taskTool.version; archiveSha256=$taskTool.sha256; executableSha256=(Get-FileHash -LiteralPath $taskExe -Algorithm SHA256).Hash.ToLowerInvariant() })
}
if ($GoTools -or $SourceGitleaks) {
    $taskOldGoBin = $env:GOBIN
    $taskOldGoProxy = $env:GOPROXY
    try {
        if ($OfficialGoProxy) { $env:GOPROXY = 'https://proxy.golang.org' }
        $taskGoEntries = @()
        if ($GoTools) { $taskGoEntries += @($taskLock.goTools | ForEach-Object { [pscustomobject]@{name=$_.name;module=$_.module;version=$_.version;bin='.tools/go'} }) }
        if ($SourceGitleaks) {
            $taskFallback = ($taskLock.downloads | Where-Object name -eq 'gitleaks').sourceFallback
            $taskGoEntries += [pscustomobject]@{name='gitleaks';module=$taskFallback.module;version=$taskFallback.version;bin='.tools/gitleaks'}
        }
        foreach ($taskTool in $taskGoEntries) {
            $env:GOBIN = Resolve-TaskLocalPath $taskTool.bin
            [System.IO.Directory]::CreateDirectory($env:GOBIN) | Out-Null
            & go install ($taskTool.module + '@' + $taskTool.version)
            if ($LASTEXITCODE -ne 0) { throw ('go install failed: ' + $taskTool.name) }
            $taskExe = Resolve-TaskLocalPath ($taskTool.bin + '/' + $taskTool.name + '.exe')
            $taskVersion = $taskTool.version
            if ($taskTool.name -eq 'gitleaks') { $taskVersion = $taskVersion.TrimStart('v') }
            Save-TaskToolReceipt ([pscustomobject]@{ name=$taskTool.name; version=$taskVersion; module=$taskTool.module; moduleVersion=$taskTool.version; executableSha256=(Get-FileHash -LiteralPath $taskExe -Algorithm SHA256).Hash.ToLowerInvariant() })
        }
    } finally { $env:GOBIN = $taskOldGoBin; $env:GOPROXY = $taskOldGoProxy }
}
$taskRecord = [pscustomobject]@{ installedAt=[DateTime]::UtcNow.ToString('o'); tools=$taskInstalled }
[System.IO.File]::WriteAllText((Resolve-TaskLocalPath '.tools/installed.json'), ($taskRecord | ConvertTo-Json -Depth 6), (New-Object System.Text.UTF8Encoding($false)))
$taskRecord | ConvertTo-Json -Depth 6
