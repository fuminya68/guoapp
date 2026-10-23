# Ephemeral Windows CI only: process-correlated WFP 5156 & DNS diagnostics.
# Reports metadata, no TLS payload or authentic user secrets.
[CmdletBinding()]
param(
  [Parameter(Mandatory=$true)][string]$Executable,
  [Parameter(Mandatory=$true)][string]$ReportPath,
  [ValidateRange(30,300)][int]$DurationSeconds=120
)
$ErrorActionPreference='Stop'
$exe=(Resolve-Path -LiteralPath $Executable).Path
New-Item -ItemType Directory -Force -Path (Split-Path $ReportPath -Parent) | Out-Null
$report=@{
  title='Extended Windows malware-behavior network triage'
  testedExecutable=Split-Path $exe -Leaf
  durationSeconds=$DurationSeconds
  wfpControlVerified=$false
  wfpAppConnections=@()
  sampledChildProcesses=@()
  dnsCandidates=@()
  dnsEvents=@()
  reviewFlags=@()
  errors=@()
  limits=@('WFP event 5156 records sockets, not HTTPS domain, payload or SNI',
    'DNS cache and DNS Operational entries are candidates only; CDN IPs may be shared',
    'Connection records and short-lived process sampling cannot prove no malware',
    'Synthetic startup without passwords or live provider interaction',
    'No inbound firewall exceptions or user secrets were supplied')
}
$app=$null
$wfpChanged=$false
function Fields($event) {
  $x=[xml]$event.ToXml()
  $h=@{}
  foreach($entry in @($x.Event.EventData.Data)){
    if($entry.Name){$h[[string]$entry.Name]=[string]$entry.'#text'}
  }
  return $h
}
function Parse-Pid([string]$value) {
  if($value -match '^0x[0-9a-f]+$'){
    return [Convert]::ToInt32($value.Substring(2),16)
  }
  $n=0
  if([int]::TryParse($value,[ref]$n)){return $n}
  return -1
}
function Get-Connections([datetime]$from) {
  $list=@()
  $events=@(Get-WinEvent -FilterHashtable @{LogName='Security';Id=5156;StartTime=$from} -MaxEvents 35000 -ErrorAction SilentlyContinue)
  foreach($ev in $events) {
    $f=Fields $ev
    if(-not $f.ContainsKey('ProcessID')) {continue}
    $list+= [pscustomobject]@{
      utc=$ev.TimeCreated.ToUniversalTime().ToString('o')
      pid=(Parse-Pid $f['ProcessID'])
      application=$f['Application']
      remoteIP=$f['DestAddress']
      remotePort=$f['DestPort']
      protocol=$f['Protocol']
      direction=$f['Direction']
    }
  }
  return @($list)
}
function DNS-Snapshot {
  $result=@()
  try {
    $result=@(Get-DnsClientCache -ErrorAction Stop |
      Where-Object {$_.Entry -and $_.RecordData} |
      ForEach-Object { [pscustomobject]@{hostname=$_.Entry;address=$_.RecordData;type=$_.Type} })
  } catch { $report.errors+= ('DNS cache: '+$_.Exception.Message) }
  return @($result)
}
try {
  $identity=[Security.Principal.WindowsIdentity]::GetCurrent()
  $principal=[Security.Principal.WindowsPrincipal]::new($identity)
  if(-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'WFP connection audit requires elevated disposable Windows runner'
  }
  $policy=& auditpol /set /subcategory:'Filtering Platform Connection' /success:enable /failure:disable 2>&1
  if($LASTEXITCODE -ne 0){throw "Unable to enable WFP connection audit: $policy"}
  $wfpChanged=$true
  $cacheBefore=@(DNS-Snapshot)
  $control=(Get-Date).AddSeconds(-1)
  # The control must exercise a real outbound connection by this PowerShell PID.
  try { Invoke-WebRequest -Uri 'https://example.com/' -UseBasicParsing -TimeoutSec 12 | Out-Null }
  catch { Write-Warning ('Control HTTP request failed: '+$_.Exception.Message) }
  for($trial=0;$trial -lt 8;$trial++) {
    Start-Sleep -Milliseconds 500
    $controlHits=@(Get-Connections $control | Where-Object {
      $_.pid -eq $PID -and $_.remotePort -eq '443'
    })
    if($controlHits.Count -gt 0) { $report.wfpControlVerified=$true; break }
  }
  if(-not $report.wfpControlVerified){
    throw 'FAIL CLOSED: WFP 5156 logging did not capture the instrumented PowerShell network control'
  }
  $start=(Get-Date).AddMilliseconds(-200)
  $app=Start-Process -FilePath $exe -WorkingDirectory (Split-Path $exe -Parent) -PassThru
  $rootId=[int]$app.Id
  $seenPids=@{}
  $seenPids[$rootId]=$true
  $end=(Get-Date).AddSeconds($DurationSeconds)
  while((Get-Date) -lt $end) {
    $procs=@(Get-CimInstance Win32_Process -ErrorAction Stop)
    for($pass=0;$pass -lt 3;$pass++) {
      foreach($p in $procs) {
        if($seenPids.ContainsKey([int]$p.ParentProcessId)) {
          if([int]$p.ProcessId -ne $rootId) {
            $entry=@{name=$p.Name;pid=[int]$p.ProcessId;parent=[int]$p.ParentProcessId}
            if(-not @($report.sampledChildProcesses|Where-Object{$_.pid -eq $entry.pid}).Count) {
              $report.sampledChildProcesses+= $entry
            }
          }
          $seenPids[[int]$p.ProcessId]=$true
        }
      }
    }
    Start-Sleep -Milliseconds 850
  }
  if(-not $app.HasExited) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
  Start-Sleep -Seconds 1
  $connections=@(Get-Connections $start | Where-Object {$seenPids.ContainsKey([int]$_.pid)})
  $report.wfpAppConnections=@($connections | Sort-Object utc,remoteIP,remotePort -Unique)
  $cacheAfter=@(DNS-Snapshot)
  $before=@{}
  foreach($item in $cacheBefore){$before["$($item.hostname)|$($item.address)"]=$true}
  $report.dnsCandidates=@($cacheAfter | Where-Object {
    -not $before.ContainsKey("$($_.hostname)|$($_.address)")
  } | Sort-Object hostname,address -Unique)
  try {
    $events=@(Get-WinEvent -FilterHashtable @{
      LogName='Microsoft-Windows-DNS-Client/Operational';StartTime=$start
    } -MaxEvents 6000 -ErrorAction SilentlyContinue)
    foreach($event in $events){
      $f=Fields $event
      $name=($f['QueryName'], $f['Name'] | Where-Object {$_} | Select-Object -First 1)
      if($name){
        $report.dnsEvents+=@{utc=$event.TimeCreated.ToUniversalTime().ToString('o');
          eventId=$event.Id;name=[string]$name}
      }
    }
  } catch { $report.errors+=('DNS events: '+$_.Exception.Message) }
  $badChild=@($report.sampledChildProcesses|Where-Object {
    $_.name -match '^(?i:powershell|pwsh|cmd|wscript|cscript|mshta|rundll32)\.exe$'
  })
  if($badChild.Count -gt 0) {
    $report.reviewFlags+= 'Child shell/script-host process observed; inspect evidence'
  }
  Write-Host "WFP positive control: $($report.wfpControlVerified)"
  Write-Host "App WFP events: $($report.wfpAppConnections.Count)"
  Write-Host "Candidate DNS resolutions: $($report.dnsCandidates.Count)"
  Write-Host "Child processes: $($report.sampledChildProcesses.Count)"
  if($report.reviewFlags.Count -gt 0){throw 'Malware triage review flag: unexpected child interpreter'}
} catch {
  $report.errors+= $_.Exception.Message
  throw
} finally {
  if($app -and -not $app.HasExited) {Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue}
  # Disposable Windows CI runner is destroyed after job. No changes on user PC.
  $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
}
