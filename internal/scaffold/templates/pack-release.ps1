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
        throw "Invalid package file '$Entry'. Expected extension '$Extension'."
    }
    $path = $repo
    foreach ($part in $Entry.Split('/')) {
        $base = ($part.Split('.')[0]).TrimEnd(' ')
        if ($part -in '', '.', '..', '.git', 'dist' -or $part -match '[<>:"\\|?*~\p{Cc}]' -or $part -match '[. ]$' -or
            $base -match '^(CON|PRN|AUX|NUL|CONIN\$|CONOUT\$|COM[1-9¹²³]|LPT[1-9¹²³])$') {
            throw "Unsafe package path '$Entry'."
        }
        $path = Join-Path $path $part
        $item = Get-Item -LiteralPath $path -Force
        if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
            throw "Package paths must not use links: '$Entry'."
        }
    }
    if ($item -isnot [IO.FileInfo]) {
        throw "Expected a file: '$Entry'."
    }
    if ($files.ContainsKey($Entry)) {
        if ($Entry -cnotin $files.Keys) { throw "Conflicting archive entry '$Entry'." }
        return $files[$Entry]
    }
    $files.Add($Entry, $item.FullName)
    return $item.FullName
}

function Assert-WindowsX64([string] $Path) {
    $reader = [IO.BinaryReader]::new([IO.File]::OpenRead($Path))
    try {
        if ($reader.BaseStream.Length -lt 64 -or $reader.ReadUInt16() -ne 0x5A4D) {
            throw "Expected a Windows executable: $Path"
        }
        $reader.BaseStream.Position = 0x3C
        $peOffset = $reader.ReadInt32()
        if ($peOffset -lt 64 -or $peOffset -gt ($reader.BaseStream.Length - 24)) {
            throw "Invalid executable header: $Path"
        }
        $reader.BaseStream.Position = $peOffset
        if ($reader.ReadUInt32() -ne 0x4550 -or $reader.ReadUInt16() -ne 0x8664) {
            throw "Expected a Windows x64 executable: $Path"
        }
        $reader.BaseStream.Position = $peOffset + 22
        $characteristics = $reader.ReadUInt16()
        if (-not ($characteristics -band 0x0002) -or ($characteristics -band 0x2000)) {
            throw "Expected an executable program: $Path"
        }
    } finally {
        $reader.Dispose()
    }
}

function Assert-SqliteDatabase([string] $Path) {
    $reader = [IO.BinaryReader]::new([IO.File]::OpenRead($Path))
    try {
        if ([Text.Encoding]::ASCII.GetString($reader.ReadBytes(16)) -cne "SQLite format 3`0") {
            throw "Expected a SQLite database: $Path"
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
    throw 'Invalid package ID, version, type, schema, or core API.'
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
                default { throw "Unsupported tool runtime '$($entry.runtime)'." }
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
    throw 'This package version already exists. Choose a new version or output directory.'
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
