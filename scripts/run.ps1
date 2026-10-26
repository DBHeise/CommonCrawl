$scriptFolder = Split-Path -Path $MyInvocation.MyCommand.Definition -Parent
$projectRoot = Split-Path $scriptFolder

$workFolder = Join-Path $projectRoot "cmd\Service"
#Start-Process -FilePath (Join-Path $workFolder 'Service.exe') -WorkingDirectory $workFolder -ArgumentList @("run", "--config", (Join-Path $workFolder "config.json"))
