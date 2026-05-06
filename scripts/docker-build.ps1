Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$versionFile = Join-Path $repoRoot "VERSION"
$projectVersion = if ($env:PROJECT_VERSION) { $env:PROJECT_VERSION } else { (Get-Content -Raw $versionFile).Trim() }
$nodeVersion = if ($env:NODE_VERSION) { $env:NODE_VERSION } else { "2.7.0" }
$commit = if ($env:COMMIT) { $env:COMMIT } else {
    $gitCommit = git -C $repoRoot rev-parse --short HEAD 2>$null
    if ($LASTEXITCODE -eq 0 -and $gitCommit) { $gitCommit.Trim() } else { "unknown" }
}
$buildDate = if ($env:BUILD_DATE) { $env:BUILD_DATE } else { (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ") }

docker build `
    --build-arg PROJECT_VERSION=$projectVersion `
    --build-arg NODE_VERSION=$nodeVersion `
    --build-arg COMMIT=$commit `
    --build-arg BUILD_DATE=$buildDate `
    -t ghcr.io/x-dora/rw-node-go:local `
    $repoRoot
