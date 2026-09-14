# install.ps1 — installs NetGuard auto-heal as three scheduled tasks (run elevated)
#   NetGuard-AutoHeal            (SYSTEM)  DNS/DoH healing: logon + every 2 minutes
#   NetGuard-AutoHeal-NetEvent   (SYSTEM)  DNS/DoH healing immediately on network
#                                          reconnect (Wi-Fi/cable change) — DHCP
#                                          rolls DNS back to the router and any
#                                          static DoH pair is lost (incident
#                                          2026-09-09); 2-minute polling is too
#                                          slow after a network switch.
#   NetGuard-AutoHeal-User       (user)    stale-proxy heal in the user session:
#                                          logon + every 2 minutes
# Why user+SYSTEM split: HKCU and WinINET refresh are per-user; a SYSTEM task
# would edit the wrong hive. DNS/DoH needs admin, so it runs as SYSTEM. All
# share one log. Also enables system-wide DoH policy (encrypted where templates
# are known, plaintext fallback elsewhere - safe on every network).

$ErrorActionPreference = 'Stop'

$src     = Join-Path $PSScriptRoot 'netguard.ps1'
$destDir = Join-Path $env:ProgramData 'netguard'
$dest    = Join-Path $destDir 'netguard.ps1'

# 0. Who is the console user (task must run in their session)
$consoleUser = (Get-CimInstance Win32_ComputerSystem).UserName
if (-not $consoleUser) { $consoleUser = "$env:COMPUTERNAME\$env:USERNAME" }
Write-Output "Console user: $consoleUser"

# 1. Deploy script
New-Item -ItemType Directory -Path $destDir -Force | Out-Null
Copy-Item -Path $src -Destination $dest -Force
Write-Output "Deployed: $dest"

# 2. System-wide DoH policy
netsh dns set global doh=yes
Write-Output 'System-wide DoH policy: enabled'

# 3a. SYSTEM task: DNS/DoH healing (logon + every 2 minutes)
$action    = New-ScheduledTaskAction -Execute 'powershell.exe' `
    -Argument "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$dest`""
$principal = New-ScheduledTaskPrincipal -UserId 'S-1-5-18' -LogonType ServiceAccount -RunLevel Highest
$settings  = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -StartWhenAvailable -ExecutionTimeLimit (New-TimeSpan -Minutes 5) -MultipleInstances IgnoreNew
$triggers  = @(
    (New-ScheduledTaskTrigger -AtLogOn),
    (New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes 2) -RepetitionDuration (New-TimeSpan -Days 3650))
)
Register-ScheduledTask -TaskName 'NetGuard-AutoHeal' -Action $action -Principal $principal `
    -Settings $settings -Trigger $triggers -Force | Out-Null
Write-Output 'Task registered: NetGuard-AutoHeal (SYSTEM, DNS/DoH)'

# 3b. SYSTEM task: DNS/DoH healing immediately on network reconnect.
# Event trigger: NetworkProfile/Operational event 10000 (network connected).
# That log is enabled by default (verified live 2026-09-10), unlike
# Microsoft-Windows-NDIS/Operational which is disabled out of the box. Built
# from XML because Register-ScheduledTask has no cmdlet for event triggers.
$netEventXml = @"
<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.3" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>NetGuard: DNS/DoH heal immediately after a network reconnect.</Description>
  </RegistrationInfo>
  <Triggers>
    <EventTrigger>
      <Enabled>true</Enabled>
      <Subscription>&lt;QueryList&gt;&lt;Query Id="0" Path="Microsoft-Windows-NetworkProfile/Operational"&gt;&lt;Select Path="Microsoft-Windows-NetworkProfile/Operational"&gt;*[System[EventID=10000]]&lt;/Select&gt;&lt;/Query&gt;&lt;/QueryList&gt;</Subscription>
    </EventTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>S-1-5-18</UserId>
      <LogonType>ServiceAccount</LogonType>
      <RunLevel>Highest</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <Enabled>true</Enabled>
    <ExecutionTimeLimit>PT5M</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>powershell.exe</Command>
      <Arguments>-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "$dest"</Arguments>
    </Exec>
  </Actions>
</Task>
"@
Register-ScheduledTask -TaskName 'NetGuard-AutoHeal-NetEvent' -Xml $netEventXml -Force | Out-Null
Write-Output 'Task registered: NetGuard-AutoHeal-NetEvent (SYSTEM, DNS/DoH on network reconnect)'

# 3c. User task: stale-proxy healing in the user's own session
$actionU   = New-ScheduledTaskAction -Execute 'powershell.exe' `
    -Argument "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$dest`" -UserMode"
$principalU = New-ScheduledTaskPrincipal -UserId $consoleUser -LogonType Interactive
$settingsU  = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -StartWhenAvailable -ExecutionTimeLimit (New-TimeSpan -Minutes 5) -MultipleInstances IgnoreNew
$triggersU = @(
    (New-ScheduledTaskTrigger -AtLogOn),
    (New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval (New-TimeSpan -Minutes 2) -RepetitionDuration (New-TimeSpan -Days 3650))
)
Register-ScheduledTask -TaskName 'NetGuard-AutoHeal-User' -Action $actionU -Principal $principalU `
    -Settings $settingsU -Trigger $triggersU -Force | Out-Null
Write-Output "Task registered: NetGuard-AutoHeal-User ($consoleUser, proxy heal)"

# 4. Run both modes now (SYSTEM run heals DNS if needed; user run heals proxy now)
Write-Output '--- First run: SYSTEM mode ---'
& powershell -NoProfile -ExecutionPolicy Bypass -File $dest
Write-Output "Exit code: $LASTEXITCODE"
Write-Output '--- First run: user mode ---'
& powershell -NoProfile -ExecutionPolicy Bypass -File $dest -UserMode
Write-Output "Exit code: $LASTEXITCODE"

# 5. Verify task registration
Get-ScheduledTask -TaskName 'NetGuard-AutoHeal*' | ForEach-Object {
    $info = $_ | Get-ScheduledTaskInfo
    '{0,-28} state={1,-8} last={2} result={3} next={4}' -f `
        $_.TaskName, $_.State, $info.LastRunTime, $info.LastTaskResult, $info.NextRunTime
}
