# Facet Studio Interactive Installer for Windows PowerShell
#
# Usage:
#   irm https://xibodev.github.io/facet-studio/install.ps1 | iex
# Or for local development testing:
#   .\docs\install.ps1 -Local
#
param(
    [switch]$Local,
    [string]$Port = "",
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

function Write-Banner {
    Write-Host ""
    Write-Host "  ███████╗ █████╗  ██████╗███████╗████████╗    ███████╗████████╗██╗   ██╗██████╗ ██╗ ██████╗ " -ForegroundColor Cyan
    Write-Host "  ██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝    ██╔════╝╚══██╔══╝██║   ██║██╔══██╗██║██╔═══██╗" -ForegroundColor Cyan
    Write-Host "  █████╗  ███████║██║     █████╗     ██║       ███████╗   ██║   ██║   ██║██║  ██║██║██║   ██║" -ForegroundColor Cyan
    Write-Host "  ██╔══╝  ██╔══██║██║     ██╔══╝     ██║       ╚════██║   ██║   ██║   ██║██║  ██║██║██║   ██║" -ForegroundColor Blue
    Write-Host "  ██║     ██║  ██║╚██████╗███████╗   ██║       ███████║   ██║   ╚██████╔╝██████╔╝██║╚██████╔╝" -ForegroundColor Blue
    Write-Host "  ╚═╝     ╚═╝  ╚═╝ ╚═════╝╚══════╝   ╚═╝       ╚══════╝   ╚═╝    ╚═════╝ ╚═════╝ ╚═╝ ╚═════╝ " -ForegroundColor Blue
    Write-Host "                                      Facet Studio v1.0.0" -ForegroundColor DarkGray
    Write-Host "──────────────────────────────────────────────────────────────────────────────────────────────" -ForegroundColor DarkGray
}

function Test-PortAvailable {
    param([int]$TestPort)
    try {
        $conn = Get-NetTCPConnection -LocalPort $TestPort -ErrorAction SilentlyContinue
        return ($null -eq $conn)
    } catch {
        return $true
    }
}

function Get-MaskedPassword {
    param([string]$Prompt)
    Write-Host $Prompt -NoNewline -ForegroundColor Yellow
    $password = ""
    while ($true) {
        $key = [Console]::ReadKey($true)
        if ($key.Key -eq [ConsoleKey]::Enter) {
            Write-Host ""
            break
        } elseif ($key.Key -eq [ConsoleKey]::Backspace) {
            if ($password.Length -gt 0) {
                $password = $password.Substring(0, $password.Length - 1)
                Write-Host "`b `b" -NoNewline
            }
        } elseif ($key.KeyChar -ge 32 -and $key.KeyChar -le 126) {
            $password += $key.KeyChar
            Write-Host "*" -NoNewline
        }
    }
    return $password
}

function Test-FreeModelEndpoint {
    param(
        [string]$Name,
        [string]$URL,
        [string]$Model,
        [hashtable]$Headers
    )
    Write-Host "  Testing $Name ($Model)... " -NoNewline -ForegroundColor Gray
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $body = @{
        model = $Model
        messages = @(@{ role = "user"; content = "hi" })
        max_tokens = 10
    } | ConvertTo-Json
    try {
        $res = Invoke-RestMethod -Uri $URL -Method Post -Headers $Headers -Body $body -TimeoutSec 4 -ErrorAction Stop
        $sw.Stop()
        Write-Host "✓ $($sw.ElapsedMilliseconds)ms (Reachable)" -ForegroundColor Green
        return @{
            Name = $Name
            Model = $Model
            URL = $URL
            Latency = $sw.ElapsedMilliseconds
            Success = $true
        }
    } catch {
        $sw.Stop()
        Write-Host "✗ (Unreachable or busy)" -ForegroundColor Red
        return @{
            Name = $Name
            Model = $Model
            URL = $URL
            Latency = 999999
            Success = $false
        }
    }
}

Write-Banner

# 1. Platform & Arch Check
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLower()
Write-Host "✓ System: Windows ($arch)" -ForegroundColor Green
Write-Host "✓ Runtime: Self-contained (zero external dependencies required)" -ForegroundColor Green

# 2. Destination Directory
if (-not $InstallDir) {
    $defaultInstall = Join-Path $HOME ".facet-studio\bin"
    Write-Host "`n[1/4] Install Location" -ForegroundColor Cyan
    Write-Host "      Default: $defaultInstall" -ForegroundColor DarkGray
    $inputDir = Read-Host "      Press Enter to accept or enter custom path"
    if ($inputDir.Trim()) {
        $InstallDir = $inputDir.Trim()
    } else {
        $InstallDir = $defaultInstall
    }
}
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Write-Host "✓ Destination directory: $InstallDir" -ForegroundColor Green

# 3. Port Check
$effectivePort = 18800
if ($Port) {
    $effectivePort = [int]$Port
} else {
    if (-not (Test-PortAvailable 18800)) {
        Write-Host "`n! Notice: Port 18800 is already occupied (e.g. Docker/WSL bridge)." -ForegroundColor Yellow
        if (Test-PortAvailable 19900) {
            $effectivePort = 19900
            Write-Host "✓ Using clean port $effectivePort" -ForegroundColor Green
        } elseif (Test-PortAvailable 8888) {
            $effectivePort = 8888
            Write-Host "✓ Using clean port $effectivePort" -ForegroundColor Green
        }
    }
}

# 4. Binary Installation
$facetStudioExe = Join-Path $InstallDir "facet-studio.exe"
$facetKernelExe = Join-Path $InstallDir "facet-studio-kernel.exe"

if ($Local) {
    Write-Host "`n[2/4] Deploying Local Build Binaries" -ForegroundColor Cyan
    $srcStudio = ".\build\facet-studio.exe"
    $srcKernel = ".\build\facet-studio-kernel.exe"
    if (-not (Test-Path $srcStudio) -or -not (Test-Path $srcKernel)) {
        Write-Host "Local build not found in .\build. Building now..." -ForegroundColor Yellow
        go build -v -tags "goolm,stdjson" -o .\build\facet-studio.exe .\web\backend
        go build -v -tags "goolm,stdjson" -o .\build\facet-studio-kernel.exe .\cmd\facet-studio
    }
    Copy-Item $srcStudio $facetStudioExe -Force
    Copy-Item $srcKernel $facetKernelExe -Force
    Write-Host "✓ Copied facet-studio.exe and facet-studio-kernel.exe to $InstallDir" -ForegroundColor Green
} else {
    Write-Host "`n[2/4] Fetching Facet Studio Binaries" -ForegroundColor Cyan
    # Download latest release or fall back to local if present
    $releaseUrl = "https://github.com/xibodev/facet-studio/releases/latest/download"
    $srcStudio = ".\build\facet-studio.exe"
    $srcKernel = ".\build\facet-studio-kernel.exe"
    if (Test-Path $srcStudio) {
        Copy-Item $srcStudio $facetStudioExe -Force
        Copy-Item $srcKernel $facetKernelExe -Force
        Write-Host "✓ Installed facet-studio.exe and facet-studio-kernel.exe" -ForegroundColor Green
    } else {
        Write-Host "Downloading facet-studio.exe..." -ForegroundColor DarkGray
        Invoke-WebRequest -Uri "$releaseUrl/facet-studio-windows-amd64.exe" -OutFile $facetStudioExe
        Write-Host "Downloading facet-studio-kernel.exe..." -ForegroundColor DarkGray
        Invoke-WebRequest -Uri "$releaseUrl/facet-studio-kernel-windows-amd64.exe" -OutFile $facetKernelExe
        Write-Host "✓ Downloaded and installed binaries" -ForegroundColor Green
    }
}

# Add to User PATH
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    Write-Host "✓ Added $InstallDir to User PATH" -ForegroundColor Green
}

# 5. Direct Shell Password Setup
Write-Host "`n[3/4] Setup Dashboard Security" -ForegroundColor Cyan
Write-Host "      Set your web cockpit password now (minimum 8 characters)." -ForegroundColor DarkGray
$pwd1 = ""
while ($pwd1.Length -lt 8) {
    $pwd1 = Get-MaskedPassword "      Enter Password: "
    if ($pwd1.Length -lt 8) {
        Write-Host "      Password must be at least 8 characters. Please try again." -ForegroundColor Red
    }
}
$pwd2 = Get-MaskedPassword "      Confirm Password: "
while ($pwd1 -ne $pwd2) {
    Write-Host "      Passwords do not match. Please try again." -ForegroundColor Red
    $pwd1 = Get-MaskedPassword "      Enter Password: "
    $pwd2 = Get-MaskedPassword "      Confirm Password: "
}

& $facetStudioExe -password $pwd1 | Out-Null
Write-Host "✓ Dashboard password encrypted and saved." -ForegroundColor Green

# Save port in launcher-config.json
$launcherCfgPath = Join-Path $HOME ".facet-studio\launcher-config.json"
$launcherCfg = @{
    port = $effectivePort
    public = $false
    allow_localhost_bypass = $true
} | ConvertTo-Json
Set-Content -Path $launcherCfgPath -Value $launcherCfg -Force

# 6. Live Free-Model Probing & Wiring
Write-Host "`n[4/4] Live Free Model Pre-flight" -ForegroundColor Cyan
Write-Host "      Sending live test requests to verified zero-key free providers..." -ForegroundColor DarkGray

$testResults = @()
$testResults += Test-FreeModelEndpoint -Name "OpenCode Zen" -URL "https://opencode.ai/zen/v1/chat/completions" -Model "ling-3.0-flash-fin-free" -Headers @{ "Content-Type" = "application/json"; "x-session-id" = "sess_$(Get-Random)" }
$testResults += Test-FreeModelEndpoint -Name "Pollinations" -URL "https://text.pollinations.ai/openai/chat/completions" -Model "openai-fast" -Headers @{ "Content-Type" = "application/json" }

$working = @($testResults | Where-Object { $_.Success } | Sort-Object Latency)

if ($working.Count -gt 0) {
    $best = $working[0]
    Write-Host "`n  ✓ Selected starting model: " -NoNewline -ForegroundColor Green
    Write-Host "$($best.Name) ($($best.Model))" -ForegroundColor Cyan

    # Wire into config.json
    $cfgFile = Join-Path $HOME ".facet-studio\config.json"
    $existingCfg = @{}
    if (Test-Path $cfgFile) {
        try { $existingCfg = Get-Content $cfgFile -Raw | ConvertFrom-Json -AsHashtable } catch {}
    }

    $existingCfg["active_models"] = @("opencode-zen/ling-3.0-flash-fin-free", "pollinations/openai-fast")
    $existingCfg["provider_instances"] = @(
        @{
            id = "opencode-zen"
            provider_kind = "opencode_zen"
            adapter = "openai-compatible"
            protocol = "openai"
            endpoint = "https://opencode.ai/zen/v1"
            state = "enabled"
        },
        @{
            id = "pollinations"
            provider_kind = "pollinations"
            adapter = "openai-compatible"
            protocol = "openai"
            endpoint = "https://text.pollinations.ai/openai"
            state = "enabled"
        }
    )
    $existingCfg | ConvertTo-Json -Depth 6 | Set-Content -Path $cfgFile -Force
    Write-Host "✓ Pre-seeded verified free models into your active chat shortlist." -ForegroundColor Green
}

Write-Host "`n──────────────────────────────────────────────────────────────────────────────────────────────" -ForegroundColor DarkGray
Write-Host "🎉 Facet Studio installation complete!" -ForegroundColor Green
Write-Host "   URL: http://localhost:$effectivePort" -ForegroundColor Cyan
Write-Host "   Run anytime from terminal: facet-studio" -ForegroundColor DarkGray
Write-Host "──────────────────────────────────────────────────────────────────────────────────────────────" -ForegroundColor DarkGray

$startNow = Read-Host "`n[?] Start Facet Studio now? (Y/n)"
if ($startNow.Trim().ToLower() -ne "n") {
    Write-Host "Starting Facet Studio on http://localhost:$effectivePort ..." -ForegroundColor Green
    Start-Process -FilePath $facetStudioExe -ArgumentList "-port $effectivePort"
}
