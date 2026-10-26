$scriptFolder = Split-Path -Path $MyInvocation.MyCommand.Definition -Parent
$projectRoot = Split-Path $scriptFolder

Push-Location $projectRoot

$results = go test ./... -v -json | ForEach-Object { $_ | ConvertFrom-Json }
$results | Where-Object { $_.action -in @('fail','pass')} | Sort-Object Package,Test| Format-Table Action,Elapsed,Package,Test -AutoSize | Out-Host
if ($results.action -contains 'fail') {
    exit -1
}

Pop-Location
