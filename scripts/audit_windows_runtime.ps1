<#
Windows built-in runtime audit (bounded observations, not a full EDR).
Run only with an isolated, disposable CI runner / VM and dummy data.
Captures process descendants, TCP/UDP endpoints, scheduled task changes,
Windows Run/RunOnce values and Startup-folder file changes.
Does not capture TLS payloads, short-lived sockets between samples,
arbitrary file reads, or prove no malicious code exists.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Executable,
    [Parameter(Mandatory = $true)][string]$ReportPath,
    [ValidateRange(4, 120)][int]$DurationSeconds = 16
)

$ErrorActionPreference = 'Stop'
$exe = (Resolve-Path -LiteralPath $Executable).Path
$reportParent = Split-Path -Path $ReportPath -Parent
if ($reportParent) { New-Item -ItemType Directory -Path $reportParent -Force | Out-Null }

function Persistence-Snapshot {
    $registry = @()
    $keys = @(
        'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run',
        'HKCU:\Software\Microsoft\Windows\CurrentVersion\RunOnce',
        'HKLM:\Software\Microsoft\Windows\CurrentVersion\Run',
        'HKLM:\Software\Microsoft\Windows\CurrentVersion\RunOnce',
        'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run'
    )
    foreach ($key in $keys) {
        if (-not (Test-Path $key)) { continue }
        try {
            $data = Get-ItemProperty -LiteralPath $key
            foreach ($field in $data.PSObject.Properties) {
                if ($field.Name -notlike 'PS*') {
                    $registry += "$key/$($field.Name)=$($field.Value)"
                }
            }
        } catch {}
    }
    $startup = @()
    foreach ($dir in @(
        [Environment]::GetFolderPath('Startup'),
        [Environment]::GetFolderPath('CommonStartup')
    )) {
        if (-not $dir -or -not (Test-Path $dir)) { continue }
        try {
            $startup += @(Get-ChildItem -LiteralPath $dir -File -Recurse -ErrorAction SilentlyContinue |
                ForEach-Object { "$($_.FullName)|$($_.Length)|$($_.LastWriteTimeUtc.ToString('o'))" })
        } catch {}
    }
    $tasks = @()
    try {
        $tasks = @(Get-ScheduledTask -ErrorAction Stop |
            Where-Object { $_.State -ne 'Disabled' } |
            ForEach-Object { "$($_.TaskPath)$($_.TaskName)" })
    } catch {
        Write-Warning "Scheduled task inventory not available: $($_.Exception.Message)"
    }
    return @{
        registry = @($registry | Sort-Object -Unique)
        startup = @($startup | Sort-Object -Unique)
        tasks = @($tasks | Sort-Object -Unique)
    }
}

function Added-Items($before, $after) {
    return @($after | Where-Object { $before -notcontains $_ })
}

$baseline = Persistence-Snapshot
$start = (Get-Date).ToUniversalTime()
$seenProcesses = @{}
$seenTCP = @{}
$seenUDP = @{}
$errors = @()
$app = $null

try {
    $app = Start-Process -FilePath $exe -WorkingDirectory (Split-Path $exe -Parent) -PassThru
    $rootPID = [int]$app.Id
    $finish = (Get-Date).AddSeconds($DurationSeconds)
    do {
        $all = @(Get-CimInstance Win32_Process -ErrorAction Stop)
        $tracked = @{}
        $tracked[$rootPID] = $true
        # Include descendants even if parent started earlier in the sample.
        for ($i = 0; $i -lt 4; $i++) {
            foreach ($p in $all) {
                if ($tracked.ContainsKey([int]$p.ParentProcessId)) {
                    $tracked[[int]$p.ProcessId] = $true
                }
            }
        }
        foreach ($p in $all) {
            if ($tracked.ContainsKey([int]$p.ProcessId)) {
                $seenProcesses["$($p.ProcessId)|$($p.Name)|$($p.ExecutablePath)"] = $true
            }
        }
        try {
            foreach ($c in @(Get-NetTCPConnection -ErrorAction Stop)) {
                if ($tracked.ContainsKey([int]$c.OwningProcess) -and
                    $c.State -in @('Established','SynSent','Listen')) {
                    $seenTCP["$($c.OwningProcess)|$($c.LocalAddress):$($c.LocalPort)|$($c.RemoteAddress):$($c.RemotePort)|$($c.State)"] = $true
                }
            }
        } catch { $errors += "TCP telemetry: $($_.Exception.Message)" }
        try {
            foreach ($u in @(Get-NetUDPEndpoint -ErrorAction Stop)) {
                if ($tracked.ContainsKey([int]$u.OwningProcess)) {
                    $seenUDP["$($u.OwningProcess)|$($u.LocalAddress):$($u.LocalPort)"] = $true
                }
            }
        } catch { $errors += "UDP telemetry: $($_.Exception.Message)" }
        Start-Sleep -Milliseconds 450
    } while ((Get-Date) -lt $finish -and -not $app.HasExited)
} finally {
    if ($app -and -not $app.HasExited) {
        # Stop this disposable test instance only.
        Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue
    }
}
Start-Sleep -Milliseconds 400
$after = Persistence-Snapshot
$addedRegistry = @(Added-Items $baseline.registry $after.registry)
$addedStartup = @(Added-Items $baseline.startup $after.startup)
$addedTasks = @(Added-Items $baseline.tasks $after.tasks)
$report = @{
    description = 'Bounded Windows telemetry of disposable GUI launch; not a malware clearance'
    UTCStart = $start.ToString('o')
    durationSeconds = $DurationSeconds
    imageName = Split-Path $exe -Leaf
    processEntries = @($seenProcesses.Keys | Sort-Object)
    tcp = @($seenTCP.Keys | Sort-Object)
    udp = @($seenUDP.Keys | Sort-Object)
    persistenceAdded = @{
        registry = $addedRegistry
        startupFiles = $addedStartup
        scheduledTasks = $addedTasks
    }
    samplingErrors = @($errors | Select-Object -Unique)
    limitations = @(
        'Polling can miss short-lived connections and processes',
        'No TLS payload inspection or general file access traces',
        'No attribution for OS background changes outside this process tree',
        'Synthetic GUI launch is not a live provider login/playback session'
    )
}
$report | ConvertTo-Json -Depth 7 | Out-File -LiteralPath $ReportPath -Encoding utf8
Write-Host "Runtime behavior report: $ReportPath"
Write-Host "Descendants: $($seenProcesses.Count); TCP endpoints: $($seenTCP.Count); UDP: $($seenUDP.Count)"
Write-Host "Added persistence: registry=$($addedRegistry.Count), startup=$($addedStartup.Count), scheduledTasks=$($addedTasks.Count)"
if ($addedRegistry.Count -gt 0 -or $addedStartup.Count -gt 0 -or $addedTasks.Count -gt 0) {
    throw 'Unreviewed persistence differences found in disposable runtime audit; inspect JSON evidence'
}
if ($errors.Count -gt 0) {
    Write-Warning 'Some telemetry queries failed; audit data is incomplete'
}
