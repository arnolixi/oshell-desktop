# Versioned, verified tools for trusted self-hosted jobs. No machine-wide installation.
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

function Assert-CacheChild([string]$Root, [string]$Path) {
    $rootPath = [IO.Path]::GetFullPath($Root).TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
    $childPath = [IO.Path]::GetFullPath($Path)
    if (-not $childPath.StartsWith($rootPath, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Tool cache path escaped its root'
    }
    if ((Test-Path -LiteralPath $childPath) -and ((Get-Item -LiteralPath $childPath).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw 'Tool cache must not be a reparse point'
    }
    return $childPath
}

function Get-VerifiedZipTool {
    param([string]$Root, [string]$Name, [string]$Url, [string]$Sha256,
          [string[]]$Executables, [scriptblock]$Download = {
              param($Source, $Target)
              & curl.exe --fail --silent --show-error --location --retry 2 --connect-timeout 15 --max-time 180 --output $Target $Source
              if ($LASTEXITCODE -ne 0) { throw 'Tool archive download failed' }
          })
    if ($Name -notmatch '^[a-zA-Z0-9.-]+$' -or $Sha256 -notmatch '^[a-fA-F0-9]{64}$') { throw 'Invalid tool identity' }
    $null = New-Item -ItemType Directory -Path $Root -Force
    $cache = Assert-CacheChild $Root (Join-Path $Root "$Name-$($Sha256.Substring(0,12))")
    $inventoryPath = Join-Path $cache '.verified-files.json'
    $valid = $false
    if (Test-Path -LiteralPath $inventoryPath) {
        try {
            $inventory = Get-Content -LiteralPath $inventoryPath -Raw | ConvertFrom-Json
            $valid = $inventory.archive -eq $Sha256 -and @($inventory.files).Count -gt 0
            foreach ($file in $inventory.files) {
                $path = Assert-CacheChild $cache (Join-Path $cache $file.path)
                if (-not (Test-Path -LiteralPath $path -PathType Leaf) -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $file.hash) { $valid = $false; break }
            }
            foreach ($exe in $Executables) {
                if (-not (Test-Path -LiteralPath (Assert-CacheChild $cache (Join-Path $cache $exe)) -PathType Leaf)) { $valid = $false }
            }
        } catch { $valid = $false }
    }
    if ($valid) { Write-Host "Tool cache hit: $Name (file hashes verified)"; return $cache }

    Write-Host "Tool cache miss: $Name"
    $stage = Assert-CacheChild $Root (Join-Path $Root "$Name-pending-$([guid]::NewGuid().ToString('N'))")
    $archive = "$stage.zip"
    try {
        & $Download $Url $archive
        if ((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $Sha256) { throw 'Tool archive checksum mismatch' }
        Expand-Archive -LiteralPath $archive -DestinationPath $stage
        foreach ($exe in $Executables) {
            if (-not (Test-Path -LiteralPath (Assert-CacheChild $stage (Join-Path $stage $exe)) -PathType Leaf)) { throw 'Tool archive is missing an executable' }
        }
        $files = @(Get-ChildItem -LiteralPath $stage -File -Recurse | ForEach-Object {
            @{ path = $_.FullName.Substring($stage.Length + 1); hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }
        })
        @{ archive = $Sha256; files = $files } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $stage '.verified-files.json') -Encoding UTF8
        # Only remove the verified absolute child of our dedicated tool cache.
        $cache = Assert-CacheChild $Root $cache
        if (Test-Path -LiteralPath $cache) { Remove-Item -LiteralPath $cache -Recurse -Force }
        $stage = Assert-CacheChild $Root $stage
        Move-Item -LiteralPath $stage -Destination $cache
        return $cache
    } finally {
        $stage = Assert-CacheChild $Root $stage
        if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
        $archive = Assert-CacheChild $Root $archive
        if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
    }
}

if ($MyInvocation.InvocationName -ne '.') {
    if (-not $env:RUNNER_TOOL_CACHE -or -not $env:GITHUB_PATH) { throw 'Requires a GitHub Actions runner' }
    $root = Join-Path $env:RUNNER_TOOL_CACHE 'oshell-desktop-tools'
    $pwsh = Get-VerifiedZipTool -Root $root -Name 'powershell-7.6.7' -Executables @('pwsh.exe') `
        -Url 'https://github.com/PowerShell/PowerShell/releases/download/v7.6.7/PowerShell-7.6.7-win-x64.zip' `
        -Sha256 'D2F952EBDBF24386CB8923553AA58E41A846DB35A6155EA04C58142A4B16DBB7'
    $peerDirectory = 'lrzsz_0.12.21rc_windows_x86_64'
    $peer = Get-VerifiedZipTool -Root $root -Name 'lrzsz-0.12.21rc' -Executables @("$peerDirectory/rz.exe", "$peerDirectory/sz.exe") `
        -Url 'https://github.com/trzsz/lrzsz-win32/releases/download/v0.12.21rc/lrzsz_0.12.21rc_windows_x86_64.zip' `
        -Sha256 'DACE2DB172B1B5A1E5998692760A75355B254E1826718D88BAE7FA3A6DE3C325'
    $pwsh | Out-File $env:GITHUB_PATH -Append -Encoding utf8
    (Join-Path $peer $peerDirectory) | Out-File $env:GITHUB_PATH -Append -Encoding utf8
    'OSHELL_REQUIRE_LRZSZ=1' | Out-File $env:GITHUB_ENV -Append -Encoding utf8
}
