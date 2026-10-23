# Windows Security 4663 SACL bait-file audit, disposable CI runner ONLY.
# Files contain fake sentinel data, never real credentials or wallets.
[CmdletBinding()]
param(
 [Parameter(Mandatory=$true)][string]$Executable,
 [Parameter(Mandatory=$true)][string]$ReportPath,
 [ValidateRange(10,180)][int]$DurationSeconds=35
)
$ErrorActionPreference='Stop'
$exe=(Resolve-Path -LiteralPath $Executable).Path
New-Item -ItemType Directory -Force -Path (Split-Path $ReportPath -Parent) | Out-Null
$report=@{
  auditedExecutable=(Split-Path $exe -Leaf)
  controlReadObserved=$false; appAccessEvents=@(); otherAccessEvents=@()
  errors=@()
  limitations=@('Only decoy paths are observed; arbitrary real file reads are not monitored',
    'Only startup behavior is exercised', 'No TLS payload or memory inspection',
    'Only direct app file accesses are attributed; spawned subprocesses need separate tracing')
}
$canaries=@()
$app=$null
$original=''
$modified=$false
function AccessEvents([datetime]$since) {
  $result=@()
  $events=@(Get-WinEvent -FilterHashtable @{LogName='Security';Id=4663;StartTime=$since} -ErrorAction SilentlyContinue)
  foreach($ev in $events) {
    $fields=@{}
    $xml=[xml]$ev.ToXml()
    foreach($d in @($xml.Event.EventData.Data)){$fields[[string]$d.Name]=[string]$d.'#text'}
    if($canaries -contains $fields['ObjectName']) {
      $result += [PSCustomObject]@{
        File=$fields['ObjectName']
        Process=$fields['ProcessName']
        ProcessId=$fields['ProcessId']
        AccessMask=$fields['AccessMask']
        UTC=$ev.TimeCreated.ToUniversalTime().ToString('o')
      }
    }
  }
  return @($result)
}
try {
  $identity=[Security.Principal.WindowsIdentity]::GetCurrent()
  $admin=[Security.Principal.WindowsPrincipal]::new($identity)
  if(-not $admin.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'File SACL auditing requires disposable elevated Windows CI runner'
  }
  $original=(& auditpol /get /subcategory:'File System' /r | Out-String)
  $policy=(& auditpol /set /subcategory:'File System' /success:enable /failure:enable 2>&1)
  if($LASTEXITCODE -ne 0){throw "Cannot enable File System auditing: $policy"}
  $modified=$true
  $paths=[ordered]@{
    ssh=(Join-Path $HOME '.ssh\id_ed25519')
    chrome=(Join-Path $env:LOCALAPPDATA 'Google\Chrome\User Data\Default\Login Data')
    firefox=(Join-Path $env:APPDATA 'Mozilla\Firefox\Profiles\audit-ci.default-release\logins.json')
    exodus=(Join-Path $env:APPDATA 'Exodus\exodus.wallet\ci-bait.json')
    bitcoin=(Join-Path $env:APPDATA 'Bitcoin\wallet.dat')
  }
  foreach($label in $paths.Keys) {
    $p=$paths[$label]
    New-Item -ItemType Directory -Force -Path (Split-Path $p -Parent) | Out-Null
    if(Test-Path $p){throw "Refusing to overwrite file for $label"}
    [IO.File]::WriteAllText($p,'SYNTHETIC_NON_SECRET_CI_TEST_ONLY')
    $sid=[Security.Principal.SecurityIdentifier]::new(
      [Security.Principal.WellKnownSidType]::WorldSid,$null)
    $rule=[Security.AccessControl.FileSystemAuditRule]::new(
      $sid,[Security.AccessControl.FileSystemRights]::ReadData,
      [Security.AccessControl.AuditFlags]::Success)
    $acl=Get-Acl -LiteralPath $p -Audit
    $acl.AddAuditRule($rule)
    Set-Acl -LiteralPath $p -AclObject $acl
    $canaries+= $p
  }
  $control=(Get-Date).AddSeconds(-1)
  Get-Content -LiteralPath $canaries[0] | Out-Null
  for($i=0;$i -lt 10;$i++) {
    Start-Sleep -Milliseconds 450
    if(@(AccessEvents $control).Count -gt 0) {
      $report.controlReadObserved=$true; break
    }
  }
  if(-not $report.controlReadObserved) {
    throw 'FAIL CLOSED: Security event 4663 did not record synthetic control read'
  }
  $starting=(Get-Date).AddMilliseconds(300)
  $app=Start-Process -FilePath $exe -WorkingDirectory (Split-Path $exe -Parent) -PassThru
  Start-Sleep -Seconds $DurationSeconds
  $observed=@(AccessEvents $starting)
  foreach($item in $observed) {
    $row=@{bait=(Split-Path $item.File -Leaf);process=(Split-Path ([string]$item.Process) -Leaf);
      pid=$item.ProcessId;utc=$item.UTC;accessMask=$item.AccessMask}
    if($row.process -ieq (Split-Path $exe -Leaf)){
      $report.appAccessEvents+= $row
    } else { $report.otherAccessEvents+= $row }
  }
  Write-Host "Verified Windows 4663 canary auditing: $($report.controlReadObserved)"
  Write-Host "Direct app accesses to fake secrets: $($report.appAccessEvents.Count)"
  Write-Host "Other accesses (requires separate review): $($report.otherAccessEvents.Count)"
  if($report.appAccessEvents.Count -gt 0){throw 'Unexpected app access to SSH/browser/wallet decoy'}
} catch {
  $report.errors+= $_.Exception.Message
  throw
} finally {
  if($app -and -not $app.HasExited) {
    Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue
  }
  if($modified -and $original) {
    $wasSuccess=$original -match 'Success'
    $wasFailure=$original -match 'Failure'
    & auditpol /set /subcategory:'File System' /success:$(if($wasSuccess){'enable'}else{'disable'}) /failure:$(if($wasFailure){'enable'}else{'disable'}) | Out-Null
  }
  foreach($p in $canaries){Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue}
  $report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
}
