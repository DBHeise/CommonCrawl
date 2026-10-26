$scriptFolder = Split-Path -Path $MyInvocation.MyCommand.Definition -Parent

## Compile
& $scriptFolder\compile.ps1
if ($LASTEXITCODE -ne 0) {
    exit
}

## Test
& $scriptFolder\test.ps1
if ($LASTEXITCODE -ne 0) {
    exit
}

## Run
& $scriptFolder\run.ps1 | Format-List
