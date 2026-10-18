param(
    [Parameter(Mandatory=$true)][string]$Executable,
    [Parameter(Mandatory=$true)][string]$FixtureExecutable
)
$ErrorActionPreference = 'Stop'
$taskRoot = (Resolve-Path -LiteralPath '.').Path
$taskSource = (Resolve-Path -LiteralPath $Executable).Path
$taskFixture = (Resolve-Path -LiteralPath $FixtureExecutable).Path
$taskVersion = (Get-Content -LiteralPath (Join-Path $taskRoot 'mygo.json') -Raw | ConvertFrom-Json).version
$taskSandbox = Join-Path $taskRoot ('validation/update-package-smoke-' + [guid]::NewGuid().ToString('N'))
$taskApp = Join-Path $taskSandbox '中文 便携程序'
New-Item -ItemType Directory -Path $taskApp | Out-Null
$taskExe = Join-Path $taskApp 'OShell Desktop.exe'
Copy-Item -LiteralPath $taskSource -Destination $taskExe
& $taskFixture --create-state $taskApp
if ($LASTEXITCODE -ne 0) { throw 'Unable to create synthetic configuration' }
$env:LOCALAPPDATA = Join-Path $taskSandbox 'cache'
$taskOwned = [System.Collections.Generic.List[System.Diagnostics.Process]]::new()
try {
    # Create synthetic portable data using the real application, then close it.
    $taskInitial = Start-Process -FilePath $taskExe -WorkingDirectory $taskApp -WindowStyle Hidden -PassThru
    $taskOwned.Add($taskInitial)
    $taskDeadline = [DateTime]::UtcNow.AddSeconds(30)
    while ([DateTime]::UtcNow -lt $taskDeadline -and -not (Test-Path -LiteralPath (Join-Path $taskApp 'config.json'))) {
        if ($taskInitial.HasExited) { throw 'Initial application failed to start' }
        Start-Sleep -Milliseconds 100
    }
    if (-not $taskInitial.WaitForInputIdle(10000)) { throw 'Initial GUI not ready' }
    $null = $taskInitial.CloseMainWindow()
    if (-not $taskInitial.WaitForExit(5000)) { $taskInitial.Kill(); $taskInitial.WaitForExit() }
    # Extra sentinel data must survive byte-for-byte, even if unknown to the app.
    [IO.File]::WriteAllText((Join-Path $taskApp 'known_hosts'), '# synthetic test sentinel')
    [IO.File]::WriteAllText((Join-Path $taskApp 'desktop-preferences.json'), '{"sidebarVisible":false,"wordSeparators":"/: ","shiftWhitespace":true}', [Text.UTF8Encoding]::new($false))
    $taskBefore = @{}
    Get-ChildItem -LiteralPath $taskApp -File -Recurse -Force | Where-Object { $_.Name -ne 'OShell Desktop.exe' -and $_.Name -ne '.oshell-state.lock' } | ForEach-Object {
        $taskBefore[[IO.Path]::GetRelativePath($taskApp,$_.FullName)] = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash
    }
    if (-not $taskBefore.ContainsKey('config.json')) { throw 'Synthetic configuration missing' }
    $taskStageName = '.oshell-update-package-smoke'
    $taskStage = Join-Path $taskApp $taskStageName
    New-Item -ItemType Directory -Path (Join-Path $taskStage 'payload') | Out-Null
    $taskPayload = Join-Path $taskStage 'payload/OShell Desktop.exe'
    Copy-Item -LiteralPath $taskExe -Destination $taskPayload
    # PE permits an overlay. Change the file hash without changing its version.
    $taskStream = [IO.File]::Open($taskPayload,[IO.FileMode]::Append)
    try { $taskBytes = [Text.Encoding]::UTF8.GetBytes(' update smoke marker'); $taskStream.Write($taskBytes,0,$taskBytes.Length) } finally { $taskStream.Dispose() }
    $taskHelper = Join-Path $taskStage 'updater.exe'
    Copy-Item -LiteralPath $taskExe -Destination $taskHelper
    $taskHolder = Start-Process -FilePath (Join-Path $PSHOME 'pwsh.exe') -ArgumentList '-NoProfile','-NonInteractive','-Command','Start-Sleep -Seconds 90' -WindowStyle Hidden -PassThru
    $taskOwned.Add($taskHolder)
    $taskToken = ([guid]::NewGuid().ToString('N') + [guid]::NewGuid().ToString('N'))
    $taskDigest = (Get-FileHash -LiteralPath $taskPayload -Algorithm SHA256).Hash.ToLowerInvariant()
    $taskPlan = @{
        Executable=$taskExe; Stage=$taskStageName; Token=$taskToken; Version=$taskVersion; ParentPID=$taskHolder.Id
        Files=@(@{Name='OShell Desktop.exe';SHA256=$taskDigest;Existed=$true})
    }
    $taskJournal = Join-Path $taskApp '.oshell-update.json'
    [IO.File]::WriteAllText($taskJournal,($taskPlan | ConvertTo-Json -Depth 5),[Text.UTF8Encoding]::new($false))
    $taskUpdate = Start-Process -FilePath $taskHelper -ArgumentList '--oshell-apply-update',('"'+$taskJournal+'"') -WorkingDirectory $taskApp -WindowStyle Hidden -PassThru
    $taskOwned.Add($taskUpdate)
    $taskDeadline = [DateTime]::UtcNow.AddSeconds(15)
    while (-not (Test-Path -LiteralPath (Join-Path $taskStage 'helper-ready')) -and [DateTime]::UtcNow -lt $taskDeadline) {
        if ($taskUpdate.HasExited) { throw "Updater exited before handshake: $($taskUpdate.ExitCode)" }
        Start-Sleep -Milliseconds 100
    }
    if (-not (Test-Path -LiteralPath (Join-Path $taskStage 'helper-ready'))) { throw 'Helper handshake missing' }
    $taskHolder.Kill(); $taskHolder.WaitForExit()
    if (-not $taskUpdate.WaitForExit(45000)) { throw 'Packaged application update timed out' }
    if ($taskUpdate.ExitCode -ne 0) { throw "Updater failed: $($taskUpdate.ExitCode)" }
    $taskReceipt = [IO.File]::ReadAllText((Join-Path $taskStage 'receipt'))
    $taskChildPID = [int]($taskReceipt.Split(':')[-1])
    $taskChild = Get-Process -Id $taskChildPID
    if ($taskChild.Path -ne $taskExe) { throw 'Unexpected restarted executable' }
    $taskOwned.Add($taskChild)
    if (-not $taskChild.WaitForInputIdle(10000)) { throw 'Updated GUI not ready' }
    if ((Get-FileHash -LiteralPath $taskExe -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskDigest) { throw 'Updated executable hash mismatch' }
    if (Test-Path -LiteralPath $taskJournal) { throw 'Update journal not finalized' }
    foreach ($taskName in $taskBefore.Keys) {
        if ((Get-FileHash -LiteralPath (Join-Path $taskApp $taskName) -Algorithm SHA256).Hash -ne $taskBefore[$taskName]) { throw "Portable data changed: $taskName" }
    }
    & $taskFixture --verify-state $taskApp
    if ($LASTEXITCODE -ne 0) { throw 'Synthetic session or saved password lost after update' }
    [pscustomobject]@{Version=$taskVersion;SingleFileUpdate=$true;RestartReceipt=$true;GuiReady=$true;DataPreserved=@($taskBefore.Keys);Result=[IO.File]::ReadAllText((Join-Path $taskApp '.oshell-update-result.txt'));TestDirectory=$taskSandbox} |
        ConvertTo-Json | Set-Content -LiteralPath (Join-Path $taskRoot 'validation/update-package-smoke.json')
    Get-Content -LiteralPath (Join-Path $taskRoot 'validation/update-package-smoke.json')
} finally {
    foreach ($taskProcess in $taskOwned) {
        try { if (-not $taskProcess.HasExited) { $null=$taskProcess.CloseMainWindow();if (-not $taskProcess.WaitForExit(3000)) { $taskProcess.Kill();$taskProcess.WaitForExit() } } } catch { }
        $taskProcess.Dispose()
    }
}
