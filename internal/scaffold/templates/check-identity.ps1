function Assert-PackageIdentity([System.Collections.IDictionary] $Package, [string] $Repository) {
    if (-not $Package.Contains('shortName')) {
        return
    }
    $typePrefix = if ($Package.type -ceq 'toolplugin') { 'tool' } else { $Package.type }
    if ($Package.publisher -isnot [string] -or $Package.publisher -cnotmatch '\A[a-z][a-z0-9]*\z' -or
        $typePrefix -cnotin 'book', 'library', 'theme', 'tool', 'bundle' -or
        $Package.shortName -isnot [string] -or $Package.shortName.Length -gt 48 -or
        $Package.shortName -cnotmatch '\A[a-z][a-z0-9]*(?:-[a-z0-9]+)*\z') {
        throw 'publisher、type 或 shortName 无效。'
    }
    $expected = "$($Package.publisher)-$typePrefix-$($Package.shortName)"
    if ($Package.name -cne $expected) {
        throw "扩展包名称必须是 '$expected'。"
    }
    $directory = Split-Path -Leaf $Repository
    if ($directory -cne $expected) {
        throw "源目录名 '$directory' 必须与扩展包名称 '$expected' 一致。"
    }
}
