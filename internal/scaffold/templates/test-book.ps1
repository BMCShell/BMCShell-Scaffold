<#
.SYNOPSIS
    在临时数据目录安装工具书，验证列表、帮助、搜索、详情和版本命令。
.PARAMETER BmcExe
    必填。用于测试的 BMCShell.exe 路径。
.EXAMPLE
    ./scripts/test-book.ps1 -BmcExe ../BMCShell/build/BMCShell.exe
#>

#requires -Version 7.4
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string] $BmcExe
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
$bmc = (Get-Item -LiteralPath $BmcExe).FullName
$project = Split-Path -Parent $PSScriptRoot
$package = Get-Content -LiteralPath (Join-Path $project 'bmc-package.json') -Raw | ConvertFrom-Json
$bookName = $package.shortName ?? $package.name

# 修改示例数据或查询规则时，同步更新这些测试值和下方的查询。
$entryId = '1'
$entryName = '示例条目'
$keyword = '示例'
$entryContent = '请替换为你的工具书正文。'

function Assert-BmcCommand([string[]] $Command, [string] $ExpectedText, [int] $ExpectedExitCode = 0) {
    $output = & $bmc @Command 2>&1 | Out-String
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne $ExpectedExitCode -or ($ExpectedText -and -not $output.Contains($ExpectedText))) {
        throw "bmc $($Command -join ' ') 的预期退出码为 $ExpectedExitCode，实际为 $exitCode，预期输出包含 '$ExpectedText'。`n$output"
    }
}

$previousHome = $env:BMCSHELL_HOME
$previousContext = $env:BMCSHELL_CONTEXT
$previousEncoding = [Console]::OutputEncoding
$testHome = [IO.Directory]::CreateTempSubdirectory('bmcshell-book-test-')
try {
    $env:BMCSHELL_HOME = $testHome.FullName
    $env:BMCSHELL_CONTEXT = $null
    [Console]::OutputEncoding = [Text.Encoding]::UTF8

    Assert-BmcCommand @('plugin', 'install', '--file', $project)
    Assert-BmcCommand @('book', '-l') "book`t$bookName`t$($package.version)"
    Assert-BmcCommand @('book', $bookName, '--help') '--id'
    Assert-BmcCommand @('book', $bookName, '-l') $entryName
    Assert-BmcCommand @('book', $bookName, '-s', $keyword) $entryName
    Assert-BmcCommand @('book', $bookName, $entryId) $entryContent
    Assert-BmcCommand @('book', $bookName, '--id', $entryId) $entryContent
    Assert-BmcCommand @('book', $bookName, '--name', $entryName) $entryContent
    Assert-BmcCommand @('book', $bookName, '-s', '__bmcshell_book_test_no_match__') -ExpectedExitCode 4
    Assert-BmcCommand @('book', $bookName, '-v') $package.version
} finally {
    $env:BMCSHELL_HOME = $previousHome
    $env:BMCSHELL_CONTEXT = $previousContext
    [Console]::OutputEncoding = $previousEncoding
    Remove-Item -LiteralPath $testHome.FullName -Recurse -Force
}

Write-Output "工具书 $bookName 测试通过。"
