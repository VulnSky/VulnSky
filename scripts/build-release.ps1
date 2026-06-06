param(
    [string]$OutputDir = "dist"
)

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

function Get-SHA256Hex {
    param([string]$Path)

    $cmd = Get-Command Get-FileHash -ErrorAction SilentlyContinue
    if ($cmd) {
        return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
    }

    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $sha = [System.Security.Cryptography.SHA256]::Create()
        try {
            return ([System.BitConverter]::ToString($sha.ComputeHash($stream)) -replace "-", "").ToLowerInvariant()
        }
        finally {
            $sha.Dispose()
        }
    }
    finally {
        $stream.Dispose()
    }
}

function New-ZipPackage {
    param(
        [string]$SourcePath,
        [string]$ArchivePath
    )

    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    if (Test-Path $ArchivePath) {
        Remove-Item -Force -LiteralPath $ArchivePath
    }
    $archive = [System.IO.Compression.ZipFile]::Open($ArchivePath, [System.IO.Compression.ZipArchiveMode]::Create)
    try {
        [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, $SourcePath, (Split-Path -Leaf $SourcePath)) | Out-Null
    }
    finally {
        $archive.Dispose()
    }
}

function New-ReleasePackage {
    param(
        [string]$GOOS,
        [string]$BinaryPath
    )

    if ($GOOS -eq "windows") {
        $archivePath = [System.IO.Path]::ChangeExtension($BinaryPath, ".zip")
        New-ZipPackage -SourcePath $BinaryPath -ArchivePath $archivePath
        return $archivePath
    }

    $tar = Get-Command tar -ErrorAction SilentlyContinue
    if ($tar) {
        $archivePath = "$BinaryPath.tar.gz"
        & $tar.Source -czf $archivePath -C (Split-Path -Parent $BinaryPath) (Split-Path -Leaf $BinaryPath)
        if ($LASTEXITCODE -ne 0) {
            throw "tar failed with exit code $LASTEXITCODE"
        }
        return $archivePath
    }

    $fallbackArchivePath = "$BinaryPath.zip"
    New-ZipPackage -SourcePath $BinaryPath -ArchivePath $fallbackArchivePath
    return $fallbackArchivePath
}

$Go = Resolve-Go

function Invoke-Go {
    & $Go @args
    if ($LASTEXITCODE -ne 0) {
        throw "go $($args -join ' ') failed with exit code $LASTEXITCODE"
    }
}

$modulePath = (& $Go list -m).Trim()
if ($LASTEXITCODE -ne 0 -or -not $modulePath) {
    throw "go list -m failed"
}

$version = "dev"
$commit = "none"
if (Get-Command git -ErrorAction SilentlyContinue) {
    $tag = git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -eq 0 -and $tag) {
        $version = $tag.Trim()
    }
    $sha = git rev-parse HEAD 2>$null
    if ($LASTEXITCODE -eq 0 -and $sha) {
        $commit = $sha.Trim()
    }
}
$buildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -X $modulePath/internal/version.Version=$version -X $modulePath/internal/version.Commit=$commit -X $modulePath/internal/version.BuildDate=$buildDate"

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Output = "vulnsky-windows-amd64.exe" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Output = "vulnsky-windows-arm64.exe" },
    @{ GOOS = "linux"; GOARCH = "amd64"; Output = "vulnsky-linux-amd64" },
    @{ GOOS = "linux"; GOARCH = "arm64"; Output = "vulnsky-linux-arm64" },
    @{ GOOS = "darwin"; GOARCH = "amd64"; Output = "vulnsky-darwin-amd64" },
    @{ GOOS = "darwin"; GOARCH = "arm64"; Output = "vulnsky-darwin-arm64" }
)

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $OutputDir "vulnsky-*")
Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $OutputDir "SHA256SUMS")

Invoke-Go test ./...
Invoke-Go vet ./...

foreach ($target in $targets) {
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = "0"
    $out = Join-Path $OutputDir $target.Output
    Write-Host "building $($target.GOOS)/$($target.GOARCH) -> $out"
    Invoke-Go build -trimpath -buildvcs=false "-ldflags=$ldflags" -o $out ./cmd/vulnsky
    $archive = New-ReleasePackage -GOOS $target.GOOS -BinaryPath $out
    Write-Host "packaged $archive"
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

Get-ChildItem -LiteralPath $OutputDir -File |
    Where-Object { $_.Name -like "vulnsky-*.zip" -or $_.Name -like "vulnsky-*.tar.gz" } |
    Sort-Object Name |
    ForEach-Object {
        $hash = Get-SHA256Hex $_.FullName
        "$hash  $($_.Name)"
    } | Set-Content -Encoding ascii -LiteralPath (Join-Path $OutputDir "SHA256SUMS")
