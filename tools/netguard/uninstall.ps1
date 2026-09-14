# uninstall.ps1 - removes NetGuard auto-heal (run elevated)
#   - unregisters both scheduled tasks (NetGuard-AutoHeal, NetGuard-AutoHeal-User)
#   - optionally deletes %ProgramData%\netguard (script copy + log) with -PurgeData
# Does NOT touch: system DoH policy (netsh dns set global doh=yes is global and
# harmless), adapter DNS settings left by previous heals (they are valid
# known-good resolvers), HKCU proxy state (user data).
# Backup the log before purging if you need history:
#   Copy-Item C:\ProgramData\netguard\netguard.log .\netguard.log.bak

param([switch]$PurgeData)

$ErrorActionPreference = 'Stop'

foreach ($task in 'NetGuard-AutoHeal', 'NetGuard-AutoHeal-User') {
    $existing = Get-ScheduledTask -TaskName $task -ErrorAction SilentlyContinue
    if ($existing) {
        Unregister-ScheduledTask -TaskName $task -Confirm:$false
        Write-Output "Task unregistered: $task"
    } else {
        Write-Output "Task not present (nothing to do): $task"
    }
}

if ($PurgeData) {
    $dir = Join-Path $env:ProgramData 'netguard'
    if (Test-Path $dir) {
        Remove-Item -Path $dir -Recurse -Force
        Write-Output "Data removed: $dir"
    } else {
        Write-Output "Data dir not present: $dir"
    }
} else {
    Write-Output "Data kept: $(Join-Path $env:ProgramData 'netguard') (use -PurgeData to remove)"
}

Write-Output 'NetGuard uninstall complete.'
