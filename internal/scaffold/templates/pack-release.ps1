<#
.SYNOPSIS
    校验扩展包清单及资源，生成扩展 ZIP 和 SHA-256 校验清单。
.DESCRIPTION
    工具包应先构建 bin/tool.exe。发布文件包含清单声明的资源。
.PARAMETER OutputDirectory
    输出目录，默认使用扩展包项目的 dist。
.EXAMPLE
    ./scripts/pack-release.ps1 -OutputDirectory ./dist
#>

#requires -Version 7.4
[CmdletBinding()]
param(
    [string] $OutputDirectory = (Join-Path $PSScriptRoot '../dist')
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot 'check-identity.ps1')
$files = [System.Collections.Generic.Dictionary[string, string]]::new([StringComparer]::OrdinalIgnoreCase)

function Add-ReleaseFile([string] $Entry, [string] $Extension = '') {
    if ([string]::IsNullOrWhiteSpace($Entry) -or ($Extension -and -not $Entry.EndsWith($Extension, [StringComparison]::OrdinalIgnoreCase))) {
        throw "扩展包文件 '$Entry' 无效。要求的扩展名为 '$Extension'。"
    }
    $path = $repo
    foreach ($part in $Entry.Split('/')) {
        $base = ($part.Split('.')[0]).TrimEnd(' ')
        if ($part -in '', '.', '..', '.git', 'dist' -or $part -match '[<>:"\\|?*~\p{Cc}]' -or $part -match '[. ]$' -or
            $base -match '^(CON|PRN|AUX|NUL|CONIN\$|CONOUT\$|COM[1-9¹²³]|LPT[1-9¹²³])$') {
            throw "扩展包路径 '$Entry' 不符合要求。"
        }
        $path = Join-Path $path $part
        $item = Get-Item -LiteralPath $path -Force
        if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
            throw "扩展包路径不能使用链接：'$Entry'。"
        }
    }
    if ($item -isnot [IO.FileInfo]) {
        throw "路径必须指向文件：'$Entry'。"
    }
    if ($files.ContainsKey($Entry)) {
        if ($Entry -cnotin $files.Keys) { throw "ZIP 条目名称冲突：'$Entry'。" }
        return $files[$Entry]
    }
    $files.Add($Entry, $item.FullName)
    return $item.FullName
}

function Assert-WindowsX64([string] $Path) {
    $reader = [IO.BinaryReader]::new([IO.File]::OpenRead($Path))
    try {
        if ($reader.BaseStream.Length -lt 64 -or $reader.ReadUInt16() -ne 0x5A4D) {
            throw "文件必须是 Windows 可执行程序：$Path"
        }
        $reader.BaseStream.Position = 0x3C
        $peOffset = $reader.ReadInt32()
        if ($peOffset -lt 64 -or $peOffset -gt ($reader.BaseStream.Length - 24)) {
            throw "可执行文件头无效：$Path"
        }
        $reader.BaseStream.Position = $peOffset
        if ($reader.ReadUInt32() -ne 0x4550 -or $reader.ReadUInt16() -ne 0x8664) {
            throw "文件必须是 Windows x64 可执行程序：$Path"
        }
        $reader.BaseStream.Position = $peOffset + 22
        $characteristics = $reader.ReadUInt16()
        if (-not ($characteristics -band 0x0002) -or ($characteristics -band 0x2000)) {
            throw "文件必须是可运行的程序：$Path"
        }
    } finally {
        $reader.Dispose()
    }
}

function Assert-SqliteDatabase([string] $Path) {
    $reader = [IO.BinaryReader]::new([IO.File]::OpenRead($Path))
    try {
        if ([Text.Encoding]::ASCII.GetString($reader.ReadBytes(16)) -cne "SQLite format 3`0") {
            throw "文件必须是 SQLite 数据库：$Path"
        }
    } finally {
        $reader.Dispose()
    }
}

$packagePath = Add-ReleaseFile 'bmc-package.json'
$package = Get-Content -LiteralPath $packagePath -Raw | ConvertFrom-Json -AsHashtable
if ($package -isnot [System.Collections.IDictionary] -or $package.schema -ne 1 -or $package.coreApi -ne 1 -or
    $package.id -cnotmatch '\A[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}\z' -or
    $package.version -cnotmatch '\A(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z' -or
    $package.type -cnotin 'toolplugin', 'book', 'library', 'theme', 'bundle') {
    throw '扩展包的 id、version、type、schema 或 coreApi 无效。'
}
Assert-PackageIdentity $package $repo
$null = Add-ReleaseFile 'README.md'
$null = Add-ReleaseFile $package.help '.json'
$null = Add-ReleaseFile $package.changelog '.json'
if ($package.envRegistry) {
    $null = Add-ReleaseFile $package.envRegistry '.csv'
}
if (Test-Path -LiteralPath (Join-Path $repo 'LICENSE')) {
    $null = Add-ReleaseFile 'LICENSE'
}
if (Test-Path -LiteralPath (Join-Path $repo 'THIRD_PARTY_NOTICES.txt')) {
    $null = Add-ReleaseFile 'THIRD_PARTY_NOTICES.txt'
}
foreach ($asset in $package.assets) {
    $null = Add-ReleaseFile $asset
}
foreach ($migration in $package.libraryMigrations.Values) {
    $null = Add-ReleaseFile $migration '.sql'
}

switch ($package.type) {
    'book' { Assert-SqliteDatabase (Add-ReleaseFile $package.data '.book.db') }
    'library' {
        if ($package.library) {
            Assert-SqliteDatabase (Add-ReleaseFile $package.data '.library.db')
        } else {
            $null = Add-ReleaseFile $package.data '.json'
        }
    }
    'theme' { $null = Add-ReleaseFile $package.data '.json' }
    'toolplugin' {
        foreach ($entry in $package.provides.tools.Values) {
            switch -CaseSensitive ($entry.runtime ?? '') {
                { $_ -cin '', 'native' } { Assert-WindowsX64 (Add-ReleaseFile $entry.entry '.exe') }
                'bash' { $null = Add-ReleaseFile $entry.entry '.sh' }
                default { throw "不支持工具运行时 '$($entry.runtime)'。" }
            }
        }
        foreach ($entry in $package.provides.resolvers.Values) {
            if ($null -ne $entry) { Assert-WindowsX64 (Add-ReleaseFile $entry.entry '.exe') }
        }
    }
}

$output = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputDirectory)
$null = [IO.Directory]::CreateDirectory($output)
$baseName = if ($package.Contains('shortName')) {
    "$($package.name)@$($package.version)"
} else {
    "BMCShell-$($package.type)-$($package.id)-$($package.version)"
}
$archivePath = Join-Path $output "$baseName.zip"
$manifestPath = Join-Path $output "$baseName.manifest.json"
if ((Test-Path -LiteralPath $archivePath) -or (Test-Path -LiteralPath $manifestPath)) {
    throw '该版本的扩展包已存在。请使用新版本号或其他输出目录。'
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
            foreach ($entry in ($files.Keys | Sort-Object -CaseSensitive)) {
                $null = [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $files[$entry], $entry)
            }
        } finally {
            $zip.Dispose()
        }
    } finally {
        $stream.Dispose()
    }

    $manifest = [ordered]@{
        schema = 1
        id = $package.id
        type = $package.type
        name = $package.name
        version = $package.version
        coreApi = $package.coreApi
        platforms = @($package.platforms)
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
