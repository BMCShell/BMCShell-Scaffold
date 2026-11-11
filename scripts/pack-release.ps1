<#
.SYNOPSIS
    打包 BMCShell-Scaffold 便携版 ZIP，并生成 SHA-256 校验清单。
.PARAMETER Version
    必填。发布使用的语义化版本号。
.PARAMETER Binary
    Windows x64 脚手架程序，默认使用仓库根目录的 BMCShell-Scaffold.exe。
.PARAMETER OutputDirectory
    输出目录，默认使用仓库根目录的 dist。
.EXAMPLE
    ./scripts/pack-release.ps1 -Version 1.0.0 -OutputDirectory ./dist
#>

#requires -Version 7.4
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidatePattern('\A(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z')]
    [string] $Version,
    [string] $Binary = (Join-Path $PSScriptRoot '../BMCShell-Scaffold.exe'),
    [string] $OutputDirectory = (Join-Path $PSScriptRoot '../dist')
)

$ErrorActionPreference = 'Stop'
$binaryFile = Get-Item -LiteralPath $Binary
if ($binaryFile -isnot [IO.FileInfo]) {
    throw "路径必须指向文件：$Binary"
}
$reader = [IO.BinaryReader]::new($binaryFile.OpenRead())
try {
    if ($reader.BaseStream.Length -lt 64 -or $reader.ReadUInt16() -ne 0x5A4D) {
        throw "文件必须是 Windows 可执行程序：$Binary"
    }
    $reader.BaseStream.Position = 0x3C
    $peOffset = $reader.ReadInt32()
    if ($peOffset -lt 64 -or $peOffset -gt ($reader.BaseStream.Length - 24)) {
        throw "可执行文件头无效：$Binary"
    }
    $reader.BaseStream.Position = $peOffset
    if ($reader.ReadUInt32() -ne 0x4550 -or $reader.ReadUInt16() -ne 0x8664) {
        throw "文件必须是 Windows x64 可执行程序：$Binary"
    }
    $reader.BaseStream.Position = $peOffset + 22
    $characteristics = $reader.ReadUInt16()
    if (-not ($characteristics -band 0x0002) -or ($characteristics -band 0x2000)) {
        throw "文件必须是可运行的程序：$Binary"
    }
} finally {
    $reader.Dispose()
}

$repo = Split-Path -Parent $PSScriptRoot
$files = [ordered]@{
    'BMCShell-Scaffold.exe' = $binaryFile.FullName
    'README.md' = Join-Path $repo 'README.md'
    'THIRD_PARTY_NOTICES.txt' = Join-Path $repo 'THIRD_PARTY_NOTICES.txt'
}
$license = Join-Path $repo 'LICENSE'
if (Test-Path -LiteralPath $license -PathType Leaf) {
    $files['LICENSE'] = $license
}

$output = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputDirectory)
$null = [IO.Directory]::CreateDirectory($output)
$platform = 'windows-amd64'
$baseName = "BMCShell-Scaffold-$Version-$platform"
$archivePath = Join-Path $output "$baseName.zip"
$manifestPath = Join-Path $output "$baseName.manifest.json"
if ((Test-Path -LiteralPath $archivePath) -or (Test-Path -LiteralPath $manifestPath)) {
    throw "该版本的发行包已存在。请使用新版本号或其他输出目录。"
}

$stagingName = [guid]::NewGuid().ToString('N')
$stagedArchive = Join-Path $output ".$stagingName.archive.tmp"
$stagedManifest = Join-Path $output ".$stagingName.manifest.tmp"
$archivePublished = $false
try {
    $stream = [IO.File]::Open($stagedArchive, [IO.FileMode]::CreateNew)
    try {
        $zip = [IO.Compression.ZipArchive]::new($stream, [IO.Compression.ZipArchiveMode]::Create, $true)
        try {
            foreach ($file in $files.GetEnumerator()) {
                $null = [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $file.Value, "BMCShell-Scaffold/$($file.Key)")
            }
        } finally {
            $zip.Dispose()
        }
    } finally {
        $stream.Dispose()
    }
    $manifest = [ordered]@{
        schema = 1
        product = 'BMCShell-Scaffold'
        version = $Version
        platform = $platform
        artifact = [IO.Path]::GetFileName($archivePath)
        sha256 = (Get-FileHash -LiteralPath $stagedArchive -Algorithm SHA256).Hash.ToLowerInvariant()
    }
    [IO.File]::WriteAllText($stagedManifest, ($manifest | ConvertTo-Json) + "`n")
    [IO.File]::Move($stagedArchive, $archivePath)
    $archivePublished = $true
    [IO.File]::Move($stagedManifest, $manifestPath)
} catch {
    if ($archivePublished) { [IO.File]::Delete($archivePath) }
    throw
} finally {
    [IO.File]::Delete($stagedArchive)
    [IO.File]::Delete($stagedManifest)
}

Get-Item -LiteralPath $archivePath, $manifestPath
