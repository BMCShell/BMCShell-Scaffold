#requires -Version 7.4
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'check-identity.ps1')
$package = Get-Content -LiteralPath (Join-Path $repo 'bmc-package.json') -Raw | ConvertFrom-Json -AsHashtable
if ($package -isnot [System.Collections.IDictionary] -or $package.type -cne 'toolplugin') {
    throw 'Expected a tool plugin package manifest.'
}
Assert-PackageIdentity $package $repo

Push-Location -LiteralPath $repo
try {
    $null = New-Item -ItemType Directory -Path 'bin' -Force
    & go build -trimpath -o bin/tool.exe ./cmd/tool
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed with exit code $LASTEXITCODE."
    }
} finally {
    Pop-Location
}
