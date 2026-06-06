$ErrorActionPreference = "Stop"

function Resolve-Go {
    if ($env:GO_EXE -and (Test-Path $env:GO_EXE)) {
        return $env:GO_EXE
    }
    $fromPath = Get-Command go -ErrorAction SilentlyContinue
    if ($fromPath) {
        return $fromPath.Source
    }
    throw "Go executable not found. Install Go or set GO_EXE to the full go executable path."
}

$Go = Resolve-Go

function Invoke-Go {
    & $Go @args
    if ($LASTEXITCODE -ne 0) {
        throw "go $($args -join ' ') failed with exit code $LASTEXITCODE"
    }
}

Invoke-Go test ./...
Invoke-Go vet ./...
$modulePath = (& $Go list -m).Trim()
if ($LASTEXITCODE -ne 0 -or -not $modulePath) {
    throw "go list -m failed"
}
Invoke-Go build -trimpath -buildvcs=false "-ldflags=-s -w -X $modulePath/internal/version.Version=verify -X $modulePath/internal/version.Commit=local -X $modulePath/internal/version.BuildDate=local" -o dist\vulnsky.exe .\cmd\vulnsky
$versionOutput = & .\dist\vulnsky.exe version
if ($LASTEXITCODE -ne 0) {
    throw "vulnsky version failed with exit code $LASTEXITCODE"
}
if (-not ($versionOutput -match "Version=verify")) {
    throw "version metadata was not injected: $versionOutput"
}

$pathsThatMustBeIgnored = @(
    ".env",
    ".env.local",
    "profiles/default.env",
    "vulnsky.db",
    "dist/vulnsky.exe",
    "backend/README.md",
    "frontend/README.md"
)

if (Test-Path ".git") {
    foreach ($path in $pathsThatMustBeIgnored) {
        git check-ignore --quiet --no-index $path
        if ($LASTEXITCODE -ne 0) {
            throw "Expected path to be ignored: $path"
        }
    }
}

Write-Host "release verification passed"
