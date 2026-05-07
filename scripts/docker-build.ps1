Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$commit = if ($env:COMMIT) { $env:COMMIT } else {
    $gitCommit = git -C $repoRoot rev-parse --short HEAD 2>$null
    if ($LASTEXITCODE -eq 0 -and $gitCommit) { $gitCommit.Trim() } else { "unknown" }
}
$buildDate = if ($env:BUILD_DATE) { $env:BUILD_DATE } else { (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ") }

docker build `
    --build-arg COMMIT=$commit `
    --build-arg BUILD_DATE=$buildDate `
    -t ghcr.io/x-dora/rw-node-go:local `
    $repoRoot
