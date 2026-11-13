###########################
# Powershell Aliases

###########################
# Folder Variables
$scriptFolder = Split-Path -Path $MyInvocation.MyCommand.Definition -Parent
$projectRoot = Split-Path $scriptFolder
$binFolder = Join-Path $projectRoot "Bin"
if (!(Test-Path $binFolder)) { mkdir $binFolder | Out-Null}

###########################
# File Lists

###########################
# Build Information
$buildStr = $env:VER
if ($buildStr -eq "") {
  $buildStr = "vLocal"
}

$arch = "-" + $env:GOOS + "-" + $env:GOARCH
if ($arch -eq "--") {
  $arch = ""
}
$ext = ""
if ($arch.StartsWith("windows") -or $arch -eq "") {
  $ext = ".exe"
}

###########################
# Full program compliation
Write-Progress -Activity "Program Compliation"
Get-ChildItem (Join-Path $projectRoot "cmd") -directory | ForEach-Object {
  Push-Location -Path $_
  Write-Progress -Activity "Program Compliation" -CurrentOperation $_
  $commit = git rev-parse --short HEAD
  $buildDate = $buildData.TimeStamp
  $ldStr = @"
-X main.commit="$commit" -X main.version="$buildStr" -X main.builddate="$buildDate"
"@
  $outFile = Join-Path $binFolder ($_.Name + $arch + $ext)
  go build -ldflags "$ldStr" -o $outFile #-json
  if ($LASTEXITCODE -ne 0) {
    exit 123453
  }
  Pop-Location
}
Write-Progress -Activity "Program Compliation" -Completed
