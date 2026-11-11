<#
.SYNOPSIS
    校验工具包清单并构建 bin/tool.exe。
.DESCRIPTION
    需要 PATH 中的 Go。请从生成的工具包项目运行该脚本。
.EXAMPLE
    ./scripts/build.ps1
#>

#requires -Version 7.4
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'check-identity.ps1')
$package = Get-Content -LiteralPath (Join-Path $repo 'bmc-package.json') -Raw | ConvertFrom-Json -AsHashtable
if ($package -isnot [System.Collections.IDictionary] -or $package.type -cne 'toolplugin') {
    throw 'bmc-package.json 必须是工具插件清单。'
}
Assert-PackageIdentity $package $repo

Push-Location -LiteralPath $repo
try {
    $null = New-Item -ItemType Directory -Path 'bin' -Force
    & go build -trimpath -o bin/tool.exe ./cmd/tool
    if ($LASTEXITCODE -ne 0) {
        throw "go build 构建失败（退出码 $LASTEXITCODE）。"
    }
} finally {
    Pop-Location
}
