# netguard.ps1 - network auto-heal (VPN-friendly)
# Two modes (installed as two scheduled tasks):
#   default (SYSTEM):      DNS-over-HTTPS healing on physical adapters + health log.
#                          Needs admin: HKLM DoH flags + Set-DnsClientServerAddress.
#   -UserMode (user):      stale system proxy heal ONLY. Must run in the user's
#                          session, because HKCU + WinINET refresh are per-user.
# Failure modes covered (seen live 2026-09-09):
#   1. VPN killed without cleanup -> ProxyEnable=1 points to dead 127.0.0.1:1080
#      -> every proxy-aware app hangs. Healed by disabling the stale entry.
#   2. Network blocks plain DNS (port 53) -> resolution dead. Healed by
#      1.1.1.1/8.8.8.8 encrypted-only (DoH over 443) on physical adapters.
# Safety rules:
#   - If something LISTENS on 1080, the proxy entry is a live VPN -> never touch.
#   - Never touches PAC or external (non-loopback) proxy settings.
#   - Never touches DNS while resolution already works.
# Log: %ProgramData%\netguard\netguard.log
# Exit codes: 0 = ok/healed, 2 = fault detected but not healed

param([switch]$UserMode)

$ErrorActionPreference = 'SilentlyContinue'
$LogDir  = Join-Path $env:ProgramData 'netguard'
$LogFile = Join-Path $LogDir 'netguard.log'
if (-not (Test-Path $LogDir)) { New-Item -ItemType Directory -Path $LogDir -Force | Out-Null }
function Log($msg) {
    $line = '{0} {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $msg
    Add-Content -Path $LogFile -Value $line
    if ($UserMode) { Write-Output $line }
    if ((Test-Path $LogFile) -and (Get-Item $LogFile).Length -gt 512KB) {
        Move-Item -Force $LogFile (Join-Path $LogDir 'netguard.old.log')
    }
}
function Toast($title, $text) {
    # Windows 10+ toast via API (visible only in an interactive user session)
    if (-not $UserMode) { return }
    try {
        $ErrorActionPreference = 'Stop'
        [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
        [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
        $xml = New-Object Windows.Data.Xml.Dom.XmlDocument
        $xml.LoadXml("<toast><visual><binding template=`"ToastText02`"><text id=`"1`">$title</text><text id=`"2`">$text</text></binding></visual></toast>")
        $toast = New-Object Windows.UI.Notifications.ToastNotification $xml
        [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Microsoft.Windows.PowerShell').Show($toast)
    } catch { } # toast is best-effort: no toast subsystem -> silent
    $ErrorActionPreference = 'SilentlyContinue'
}

function Test-ProxyListener {
    return @(Get-NetTCPConnection -State Listen -LocalPort 1080 -ErrorAction SilentlyContinue).Count -gt 0
}

# ================= USER MODE: stale system proxy =============================
if ($UserMode) {
    if (Test-ProxyListener) { exit 0 }  # VPN serving traffic -> proxy entry is valid

    $reg = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
    $is = Get-ItemProperty $reg
    if ($is.ProxyEnable -eq 1 -and $is.ProxyServer -match '^(socks=)?127\.0\.0\.1:(\d+)$') {
        $port = [int]$Matches[2]
        if (@(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue).Count -eq 0) {
            Set-ItemProperty $reg -Name ProxyEnable -Value 0
            $sig = @'
[DllImport("wininet.dll", SetLastError = true)]
public static extern bool InternetSetOption(IntPtr hInternet, int dwOption, IntPtr lpBuffer, int dwBufferLength);
'@
            $t = Add-Type -MemberDefinition $sig -Name WinInet -Namespace NetGuardUser -PassThru
            [void]$t::InternetSetOption([IntPtr]::Zero, 39, [IntPtr]::Zero, 0) # SETTINGS_CHANGED
            [void]$t::InternetSetOption([IntPtr]::Zero, 37, [IntPtr]::Zero, 0) # REFRESH
            Log ("HEALED: stale system proxy 127.0.0.1:{0} (no listener) - ProxyEnable=0, browser refresh notified" -f $port)
            Toast 'NetGuard' 'Stale system proxy disabled - browser is back online'
            exit 0
        }
    }
    exit 0
}

# ================= SYSTEM MODE: DNS heal + health ============================
$healed = 0
$problems = 0

if (Test-ProxyListener) {
    Log 'SOCKS listener on 1080 present (VPN active) - skipping all heals this run'
    exit 0
}

function Test-Resolution {
    $r = Resolve-DnsName www.google.com -QuickTimeout -ErrorAction SilentlyContinue
    return @($r | Where-Object { $_.Type -in 'A','AAAA' }).Count -gt 0
}

# DoH candidates: v4 endpoint probed BEFORE committing encrypted-only flags
# (V2-037: in networks where 1.1.1.1/8.8.8.8 DoH is blocked, blindly setting
# encrypted-only bricked resolution completely - "heal" worse than the
# disease). Quad9 9.9.9.9 added as third jurisdiction; Windows knows the
# DoH template for all three.
$dohV4v6 = @(
    @{ v4 = '1.1.1.1';        v6 = '2606:4700:4700::1111' },
    @{ v4 = '8.8.8.8';        v6 = '2001:4860:4860::8888' },
    @{ v4 = '9.9.9.9';        v6 = '2620:fe::fe' }
)

function Test-DoHReachable($ip) {
    try {
        $ErrorActionPreference = 'Stop'
        $r = Invoke-RestMethod -Uri ("https://{0}/dns-query?name=www.google.com&type=A" -f $ip) `
            -Headers @{ accept = 'application/dns-json' } -TimeoutSec 4
        return $null -ne $r.Answer
    } catch { return $false }
}

# $script:goodDoh - ordered list of candidates that answered the live probe.
$script:goodDoh = @()
foreach ($c in $dohV4v6) {
    if (Test-DoHReachable $c.v4) { $script:goodDoh += $c }
}

function Set-InterfaceDoh($guid) {
    # Encrypted-only flags ONLY for resolvers proven reachable this run;
    # setting 0x41 for a blocked endpoint bricks resolution (V2-037).
    $base = 'HKLM:\SYSTEM\CurrentControlSet\Services\Dnscache\InterfaceSpecificParameters'
    foreach ($c in $script:goodDoh) {
        foreach ($ip in @($c.v4, $c.v6)) {
            $p = "$base\$guid\DohInterfaceSettings\Doh\$ip"
            New-Item -Path $p -Force | Out-Null
            # 0x41 = DoH enabled + encrypted-only (no plaintext fallback)
            New-ItemProperty -Path $p -Name DohFlags -PropertyType QWord -Value 0x41 -Force | Out-Null
        }
    }
}

$phys = @(Get-NetAdapter | Where-Object {
    $_.Status -eq 'Up' -and
    $_.InterfaceDescription -notmatch 'Virtual|Hyper-V|TAP|Wintun|Loopback|Bluetooth|Wi-Fi Direct' -and
    $_.Name -notmatch 'snowden|outline|vEthernet|Loopback'
})

foreach ($a in $phys) {
    $idx = $a.ifIndex
    $v4 = Get-DnsClientServerAddress -InterfaceIndex $idx -AddressFamily IPv4
    $servers = @($v4.ServerAddresses)

    if (Test-Resolution) {
        # Working: keep DoH flags present for our known-good pair (idempotent).
        if ($servers -contains '1.1.1.1') { Set-InterfaceDoh $a.InterfaceGuid }
        continue
    }

    if ($servers.Count -eq 0) {
        Log ("PROBLEM: adapter '{0}' (ifIndex {1}) has no DNS servers and resolution fails" -f $a.Name, $idx)
        $problems++
        continue
    }

    if ($script:goodDoh.Count -eq 0) {
        # No DoH endpoint reachable: switching DNS would only make things
        # worse (encrypted-only against a blocked endpoint = dead resolver).
        Log ("PROBLEM: adapter '{0}' - resolution fails and no DoH endpoint reachable from this network; DNS left untouched" -f $a.Name)
        $problems++
        continue
    }

    $newServers = @()
    foreach ($c in $script:goodDoh) { $newServers += @($c.v4, $c.v6) }
    Log ("HEALING: adapter '{0}' (ifIndex {1}, DNS: {2}) - resolution fails, switching to encrypted DoH ({3})" -f
        $a.Name, $idx, ($servers -join ','), (($script:goodDoh | ForEach-Object { $_.v4 }) -join ','))
    Set-DnsClientServerAddress -InterfaceIndex $idx -ServerAddresses $newServers
    Set-InterfaceDoh $a.InterfaceGuid
    Clear-DnsClientCache
    Start-Sleep -Milliseconds 800
    if (Test-Resolution) {
        Log ("HEALED: adapter '{0}' - resolution works via DoH" -f $a.Name)
        $healed++
    } else {
        Log ("PROBLEM: adapter '{0}' - DoH applied but resolution still fails (network may block 443 too)" -f $a.Name)
        $problems++
    }
}

# --- End-to-end health status (log only) -------------------------------------
$sw = [System.Diagnostics.Stopwatch]::StartNew()
try {
    $r = Invoke-WebRequest -Uri 'https://www.google.com/generate_204' -UseBasicParsing -TimeoutSec 8
    $sw.Stop()
    Log ("STATUS: google 204 OK ({0:N2}s)" -f $sw.Elapsed.TotalSeconds)
} catch {
    if (Test-Resolution) {
        Log 'STATUS: DNS resolves but HTTPS to google fails - site/network-level block (VPN required, not a proxy/DNS issue)'
    } else {
        Log 'STATUS: offline or heavily restricted network'
    }
    $problems++
}

Log ("RUN COMPLETE: healed={0} problems={1}" -f $healed, $problems)

if ($problems -gt 0) { exit 2 }
exit 0
