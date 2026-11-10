function Assert-PackageIdentity([System.Collections.IDictionary] $Package, [string] $Repository) {
    if (-not $Package.Contains('shortName')) {
        return
    }
    $typePrefix = if ($Package.type -ceq 'toolplugin') { 'tool' } else { $Package.type }
    if ($Package.publisher -isnot [string] -or $Package.publisher -cnotmatch '\A[a-z][a-z0-9]*\z' -or
        $typePrefix -cnotin 'book', 'library', 'theme', 'tool', 'bundle' -or
        $Package.shortName -isnot [string] -or $Package.shortName.Length -gt 48 -or
        $Package.shortName -cnotmatch '\A[a-z][a-z0-9]*(?:-[a-z0-9]+)*\z') {
        throw 'Invalid publisher, type, or shortName.'
    }
    $expected = "$($Package.publisher)-$typePrefix-$($Package.shortName)"
    if ($Package.name -cne $expected) {
        throw "Package name must be '$expected'."
    }
    $directory = Split-Path -Leaf $Repository
    if ($directory -cne $expected) {
        throw "Source directory '$directory' must match package name '$expected'."
    }
}
