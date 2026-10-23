# Sysmon process-correlated DNS and network forensic review.
# Run ONLY on disposable admin Windows CI runner; never on a user's computer.
[CmdletBinding()]
param(
 [Parameter(Mandatory=$true)][string]$Executable,
 [Parameter(Mandatory=$true)][string]$Config,
 [Parameter(Mandatory=$true)][string]$ReportPath,
 [ValidateRange(20,300)][int]$DurationSeconds=120
)
$ErrorActionPreference='Stop'
$exe=(Resolve-Path -LiteralPath $Executable).Path
$conf=(Resolve-Path -LiteralPath $Config).Path
New-Item -ItemType Directory -Path (Split-Path $ReportPath -Parent) -Force | Out-Null
$report=@{
  executable=(Split-Path $exe -Leaf)
  durationSeconds=$DurationSeconds
  microsoftSignatureVerified=$false
  sysmonInstalled=$false
  positiveDnsControl=$false
  positiveConnectionControl=$false
  connections=@()
  dnsQueries=@()
  childProcesses=@()
  filesCreated=@()
  registryChanges=@()
  unexpectedRegistryOperations=@()
  errors=@()
  limitations=@('No TLS decryption or HTTP request bodies',
    'DNS responses can be cached, shared, or intercepted; only exact ProcessGuid is proof of process origin',
    'No monitoring of arbitrary file reads (separate SACL canary audit)',
    'Only startup is exercised, not logged-in playback or long-term persistence',
    'Microsoft-signed Sysmon installed on throwaway Windows CI host only')
}
$sysmon=$null
$serviceInstalled=$false
$app=$null
function Fields($event) {
 $data=[xml]$event.ToXml()
 $items=@{}
 foreach($entry in @($data.Event.EventData.Data)){
   if($entry.Name){$items[[string]$entry.Name]=[string]$entry.'#text'}
 }
 return $items
}
function EventRows([datetime]$start) {
 return @(Get-WinEvent -FilterHashtable @{
   LogName='Microsoft-Windows-Sysmon/Operational'; StartTime=$start
 } -MaxEvents 30000 -ErrorAction SilentlyContinue)
}
try {
 $ident=[Security.Principal.WindowsIdentity]::GetCurrent()
 $principal=[Security.Principal.WindowsPrincipal]::new($ident)
 if(-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)){
   throw 'Sysmon requires disposable elevated Windows CI runner'
 }
 $temp=Join-Path $env:RUNNER_TEMP ('sysmon-ci-'+[guid]::NewGuid().ToString('N'))
 New-Item -ItemType Directory -Path $temp -Force | Out-Null
 $archive=Join-Path $temp 'Sysmon.zip'
 Invoke-WebRequest -Uri 'https://download.sysinternals.com/files/Sysmon.zip' -OutFile $archive -TimeoutSec 90
 Expand-Archive -LiteralPath $archive -DestinationPath (Join-Path $temp 'bin') -Force
 $binary=Get-ChildItem (Join-Path $temp 'bin') -Filter 'Sysmon64.exe' -File -Recurse | Select-Object -First 1
 if(-not $binary){throw 'Official Sysmon64 binary is missing'}
 $sig=Get-AuthenticodeSignature -LiteralPath $binary.FullName
 if($sig.Status -ne 'Valid' -or $sig.SignerCertificate.Subject -notmatch 'Microsoft'){
   throw 'Sysmon executable signature was not Microsoft/Valid'
 }
 $report.microsoftSignatureVerified=$true
 $sysmon=$binary.FullName
 & $sysmon -accepteula -i $conf 2>&1 | Out-Host
 if($LASTEXITCODE -ne 0){throw "Cannot install signed Sysmon, exit $LASTEXITCODE"}
 $serviceInstalled=$true
 $report.sysmonInstalled=$true
 Start-Sleep -Seconds 3

 # Both controls must generate Sysmon events: avoids false reassurance.
 $controlStart=(Get-Date).AddSeconds(-1)
 [System.Net.Dns]::GetHostAddresses('example.com') | Out-Null
 try {Invoke-WebRequest -Uri 'https://example.com/' -UseBasicParsing -TimeoutSec 12 | Out-Null}
 catch {Write-Warning ('Network control request: '+$_.Exception.Message)}
 for($trial=0;$trial -lt 12;$trial++){
   Start-Sleep -Milliseconds 450
   foreach($evt in @(EventRows $controlStart)){
     $f=Fields $evt
     if([string]$f['Image'] -notmatch '(?i)\\(?:powershell|pwsh)\.exe$'){continue}
     if($evt.Id -eq 22 -and $f['QueryName'] -like '*example.com*'){
       $report.positiveDnsControl=$true
     }
     if($evt.Id -eq 3 -and $f['DestinationPort'] -eq '443'){
       $report.positiveConnectionControl=$true
     }
   }
   if($report.positiveDnsControl -and $report.positiveConnectionControl){break}
 }
 if(-not $report.positiveDnsControl -or -not $report.positiveConnectionControl){
   throw "Sysmon positive controls failed: DNS=$($report.positiveDnsControl), Connection=$($report.positiveConnectionControl)"
 }
 $start=(Get-Date).AddMilliseconds(-100)
 $app=Start-Process -FilePath $exe -WorkingDirectory (Split-Path $exe -Parent) -PassThru
 $rootPID=[int]$app.Id
 Start-Sleep -Seconds $DurationSeconds
 if(-not $app.HasExited){Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue}
 Start-Sleep -Seconds 1

 foreach($evt in @(EventRows $start)){
   $f=Fields $evt
   $image=[string]$f['Image']
   $name=[IO.Path]::GetFileName($image)
   if($name -ieq 'zhenguojian.exe'){
     switch($evt.Id){
       3 {$report.connections+= @{
         utc=$f['UtcTime']; remoteIP=$f['DestinationIp']; remoteHostname=$f['DestinationHostname'];
         remotePort=$f['DestinationPort']; processGuid=$f['ProcessGuid'];
         pid=$f['ProcessId']; initiated=$f['Initiated']}}
       22 {$report.dnsQueries+= @{
         utc=$f['UtcTime']; query=$f['QueryName']; queryResults=$f['QueryResults'];
         processGuid=$f['ProcessGuid']; pid=$f['ProcessId']; status=$f['QueryStatus']}}
       11 {$report.filesCreated+= @{
         filename=$f['TargetFilename'];utc=$f['UtcTime']}}
       {$_ -in @(12,13,14)} {$report.registryChanges+= @{
         eventId=$evt.Id; eventType=$f['EventType']; path=$f['TargetObject']; utc=$f['UtcTime']}}
     }
   }
   if($evt.Id -eq 1 -and [IO.Path]::GetFileName([string]$f['ParentImage']) -ieq 'zhenguojian.exe'){
     $report.childProcesses+= @{
       childImage=$f['Image'];parentImage=$f['ParentImage'];
       pid=$f['ProcessId'];utc=$f['UtcTime']}
   }
 }
 Write-Host "Sysmon positive control DNS=$($report.positiveDnsControl), network=$($report.positiveConnectionControl)"
 Write-Host "Player network events: $($report.connections.Count)"
 Write-Host "Player DNS queries: $($report.dnsQueries.Count)"
 Write-Host "Child processes: $($report.childProcesses.Count); writes: $($report.filesCreated.Count); registry edits: $($report.registryChanges.Count)"
 # Sysmon Event 12 CreateKey calls are frequently generated by Windows
 # CryptoAPI querying certificate stores and by the TCP/IP settings API.
 # They are not equivalent to persistent Run/RunOnce entries or value writes.
 # Preserve all events but flag unexpected paths, DeleteKey and SetValue.
 foreach($change in $report.registryChanges) {
   $expectedReadThrough = $change.eventId -eq 12 -and
     $change.eventType -eq 'CreateKey' -and
     ($change.path -match 'SystemCertificates|EnterpriseCertificates' -or
      $change.path -match 'Tcpip\\Parameters')
   if(-not $expectedReadThrough) {
     $report.unexpectedRegistryOperations += $change
   }
 }
 Write-Host "Registry operations requiring manual investigation: $($report.unexpectedRegistryOperations.Count)"
 if($report.childProcesses.Count -gt 0 -or $report.unexpectedRegistryOperations.Count -gt 0){
   throw 'Unexpected child process or registry operation; examine full Sysmon report'
 }
} catch {
 $report.errors+= $_.Exception.Message
 throw
} finally {
 if($app -and -not $app.HasExited){Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue}
 if($serviceInstalled -and $sysmon){
   try {& $sysmon -u force 2>&1 | Out-Null} catch{}
 }
 $report | ConvertTo-Json -Depth 7 | Set-Content -LiteralPath $ReportPath -Encoding UTF8
}
