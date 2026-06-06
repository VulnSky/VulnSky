param(
    [Parameter(Mandatory = $true)]
    [string]$ModulePath
)

$ErrorActionPreference = "Stop"

if ($ModulePath -match "\s" -or $ModulePath -notmatch "/") {
    throw "ModulePath should look like github.com/<owner>/vulnsky"
}

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
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

function Invoke-Go {
    & $Go @args
    if ($LASTEXITCODE -ne 0) {
        throw "go $($args -join ' ') failed with exit code $LASTEXITCODE"
    }
}

Invoke-Go mod edit "-module=$ModulePath"

Get-ChildItem -Recurse -Include *.go -Path cmd, internal |
    ForEach-Object {
        $text = [System.IO.File]::ReadAllText($_.FullName, $utf8NoBom)
        $updated = [System.Text.RegularExpressions.Regex]::Replace(
            $text,
            '"vulnsky/([A-Za-z0-9_./-]+)"',
            { param($match) '"' + $ModulePath + '/' + $match.Groups[1].Value + '"' }
        )
        if ($updated -ne $text) {
            [System.IO.File]::WriteAllText($_.FullName, $updated, $utf8NoBom)
        }
    }

Invoke-Go mod tidy
Invoke-Go test ./...

Write-Host "module path updated to $ModulePath"
