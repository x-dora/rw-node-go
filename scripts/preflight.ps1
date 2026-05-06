Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot

Push-Location $repoRoot
try {
    mise run fmt
    mise run test
    mise run build
    mise run contract-diff

    if ($env:RUN_PANEL_INTEGRATION -eq "true") {
        bash scripts/panel-integration.sh run
    } else {
        Write-Host "Skipping real Panel live harness. Set RUN_PANEL_INTEGRATION=true to enable it."
    }
} finally {
    Pop-Location
}
