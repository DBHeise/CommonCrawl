###########################
# Powershell Aliases

###########################
# Folder Variables
$scriptFolder = Split-Path -Path $MyInvocation.MyCommand.Definition -Parent
$projectRoot = Split-Path $scriptFolder

###########################
# File Lists

###########################
# Build Information
$buildInfoJson = Join-Path $scriptFolder "builddata.json"
$buildData = (Get-Content $buildInfoJson -raw | ConvertFrom-Json)
$buildData.Version.Build += 1
$buildData.TimeStamp = [DateTime]::Now.ToString("yyyy-MM-ddTHH:mm:ss.fffffffzzz")
$buildData | ConvertTo-Json -Compress | Set-Content $buildInfoJson -Encoding Ascii
$buildStr = $buildData.Version.Major.ToString() + "." + $buildData.Version.Minor.ToString() + "." +  $buildData.Version.Patch.ToString() + "." +  $buildData.Version.Build.ToString()


###########################
# Full program compliation
Write-Progress -Activity "Program Compliation"
gci (Join-Path $projectRoot "cmd") -directory | % {
  Push-Location -Path $_
  Write-Progress -Activity "Program Compliation" -CurrentOperation $_
  $commit = git rev-parse --short HEAD
  $buildDate = $buildData.TimeStamp
  $ldStr = @"
-X main.commit="$commit" -X main.version="$buildStr" -X main.builddate="$buildDate"
"@
  go build -ldflags "$ldStr"
  if ($LASTEXITCODE -ne 0) {
    exit 123453
  }
  Pop-Location
}
Write-Progress -Activity "Program Compliation" -Completed
