$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/prepare-runner-tools.ps1"
$scratch = Join-Path ([IO.Path]::GetTempPath()) ('oshell-cache-test-' + [guid]::NewGuid().ToString('N'))
$script:downloads = 0
try {
    $source = Join-Path $scratch 'source'
    $null = New-Item -ItemType Directory -Path $source -Force
    [IO.File]::WriteAllText((Join-Path $source 'tool.exe'), 'synthetic executable fixture')
    [IO.File]::WriteAllText((Join-Path $source 'library.dll'), 'synthetic library fixture')
    $archive = Join-Path $scratch 'fixture.zip'
    Compress-Archive -Path "$source/*" -DestinationPath $archive
    $sha = (Get-FileHash -LiteralPath $archive).Hash
    $cacheRoot = Join-Path $scratch 'cache'
    $parameters = @{
        Root = $cacheRoot; Name = 'test-1'; Url = $archive; Sha256 = $sha; Executables = @('tool.exe')
        Download = { param($url, $target); $script:downloads++; Copy-Item -LiteralPath $url -Destination $target }
    }
    $first = Get-VerifiedZipTool @parameters
    $second = Get-VerifiedZipTool @parameters
    if ($first -ne $second -or $script:downloads -ne 1) { throw 'Verified cache was not reused' }
    [IO.File]::WriteAllText((Join-Path $first 'library.dll'), 'damaged')
    $null = Get-VerifiedZipTool @parameters
    if ($script:downloads -ne 2 -or [IO.File]::ReadAllText((Join-Path $first 'library.dll')) -eq 'damaged') { throw 'Damaged dependency was not repaired' }
    Remove-Item -LiteralPath (Join-Path $first '.verified-files.json')
    $null = Get-VerifiedZipTool @parameters
    if ($script:downloads -ne 3) { throw 'Incomplete cache was accepted' }
    $parameters.Sha256 = '0' * 64
    $rejected = $false
    try { $null = Get-VerifiedZipTool @parameters } catch { $rejected = $_.Exception.Message -match 'checksum mismatch' }
    if (-not $rejected) { throw 'Wrong tool digest was accepted' }
    if (@(Get-ChildItem -LiteralPath $cacheRoot -Filter '*pending*').Count) { throw 'Failed cache setup left temporary files' }
    $rejected = $false
    try { $null = Assert-CacheChild $cacheRoot (Join-Path $cacheRoot '../outside') } catch { $rejected = $true }
    if (-not $rejected) { throw 'Cache cleanup allowed path traversal' }
    Write-Host 'Tool cache tests passed: reuse, dependency repair, incomplete cache, checksum, cleanup boundaries.'
} finally {
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
    $scratchPath = [IO.Path]::GetFullPath($scratch)
    if (-not $scratchPath.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or (Split-Path $scratchPath -Leaf) -notlike 'oshell-cache-test-*') { throw 'Unexpected test cleanup path' }
    if (Test-Path -LiteralPath $scratchPath) { Remove-Item -LiteralPath $scratchPath -Recurse -Force }
}
