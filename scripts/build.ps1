# Local build: Windows/Linux/macOS (amd64+arm64) executables. Syncs into skill bins.
# Multi-platform release binaries: push a v* tag → GitHub Actions (.github/workflows/release.yml).
$ErrorActionPreference = "Stop"
$Version = if ($env:VERSION) { $env:VERSION } else { "0.4.0" }
$ld = "-s -w -X github.com/Fracizz/sshctl/cmd.Version=$Version"
$go = "go"
if (Test-Path "C:\Program Files\Go\bin\go.exe") {
    $env:GOROOT = "C:\Program Files\Go"
    $go = "C:\Program Files\Go\bin\go.exe"
}

New-Item -ItemType Directory -Force -Path bin | Out-Null

$platforms = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Name = "sshctl-windows-amd64.exe" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Name = "sshctl-windows-arm64.exe" },
    @{ GOOS = "linux";   GOARCH = "amd64"; Name = "sshctl-linux-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Name = "sshctl-linux-arm64" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; Name = "sshctl-darwin-amd64" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; Name = "sshctl-darwin-arm64" }
)

$skillBins = @(
    (Join-Path "skills" (Join-Path "sshctl" "bin")),
    (Join-Path $env:USERPROFILE ".claude\skills\sshctl\bin"),
    (Join-Path $env:USERPROFILE ".cursor\skills\sshctl\bin"),
    (Join-Path $env:USERPROFILE ".codex\skills\sshctl\bin")
)

foreach ($p in $platforms) {
    $env:GOOS = $p.GOOS
    $env:GOARCH = $p.GOARCH
    $env:CGO_ENABLED = "0"
    $out = Join-Path bin $p.Name
    Write-Host "building $out"
    & $go build -ldflags $ld -o $out .
    if ($LASTEXITCODE -ne 0) { throw "go build failed for $($p.Name)" }

    foreach ($binDir in $skillBins) {
        $skillRoot = Split-Path $binDir -Parent
        if (-not (Test-Path $skillRoot)) {
            continue
        }
        New-Item -ItemType Directory -Force -Path $binDir | Out-Null
        $dest = Join-Path $binDir $p.Name
        Copy-Item $out $dest -Force
        Write-Host "copied $dest"
    }
}

# Agent skill on Windows uses sshctl.exe (alias of windows-amd64)
$winAmd64 = Join-Path bin "sshctl-windows-amd64.exe"
$localExe = Join-Path bin "sshctl.exe"
Copy-Item $winAmd64 $localExe -Force
Write-Host "copied $localExe"
foreach ($binDir in $skillBins) {
    $skillRoot = Split-Path $binDir -Parent
    if (-not (Test-Path $skillRoot)) {
        continue
    }
    $dest = Join-Path $binDir "sshctl.exe"
    Copy-Item $winAmd64 $dest -Force
    Write-Host "copied $dest"
}

Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
Write-Host "done sshctl $Version"
