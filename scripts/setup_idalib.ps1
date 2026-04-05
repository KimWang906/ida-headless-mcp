#Requires -Version 5.1
<#
.SYNOPSIS
    Windows IDA idalib setup (PowerShell version of setup_idalib.sh)
.PARAMETER IdaPath
    IDA installation path (auto-detected if not specified)
#>
param(
    [string]$IdaPath = $env:IDA_PATH
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Info($msg)  { Write-Host "[*] $msg" -ForegroundColor Cyan }
function Write-Ok($msg)    { Write-Host "[+] $msg" -ForegroundColor Green }
function Write-Warn($msg)  { Write-Host "[!] $msg" -ForegroundColor Yellow }
function Fail($msg)        { Write-Host "[-] $msg" -ForegroundColor Red; exit 1 }

# ---------------------------------------------------------------------------
# 1. Locate IDA installation
# ---------------------------------------------------------------------------
if ($IdaPath -and (Test-Path $IdaPath)) {
    Write-Ok "Using IDA_PATH from environment: $IdaPath"
} else {
    $IdaPath = $null

    $candidates = @(
        "C:\Program Files\IDA Professional 9.2",
        "C:\Program Files\IDA Essential 9.2",
        "C:\Program Files\IDA Professional 9.1",
        "C:\Program Files\IDA Essential 9.1",
        "C:\Program Files\IDA Professional 9.0",
        "C:\Program Files\IDA Pro 9.0",
        "$env:USERPROFILE\idapro-9.2",
        "$env:USERPROFILE\idapro-9.1",
        "$env:USERPROFILE\idapro-9.0",
        "$env:USERPROFILE\ida-pro-9.2",
        "$env:USERPROFILE\ida-pro-9.1",
        "$env:USERPROFILE\ida-pro-9.0",
        "C:\idapro-9.2",
        "C:\idapro-9.1",
        "C:\idapro-9.0"
    )

    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            $IdaPath = $candidate
            break
        }
    }

    if (-not $IdaPath) {
        Write-Host ""
        Write-Host "[-] IDA installation not found." -ForegroundColor Red
        Write-Host "    Set the IDA_PATH environment variable or use -IdaPath:" -ForegroundColor Yellow
        Write-Host "    Example: .\scripts\setup_idalib.ps1 -IdaPath 'C:\Program Files\IDA Pro 9.2'" -ForegroundColor Yellow
        exit 1
    }

    Write-Ok "Found IDA at: $IdaPath"
}

# ---------------------------------------------------------------------------
# 2. Locate idalib directory
# ---------------------------------------------------------------------------
$IdalibDir = Join-Path $IdaPath "idalib"

if (-not (Test-Path $IdalibDir)) {
    $dllCandidates = @("ida.dll","ida64.dll","libida64.dll") |
        ForEach-Object { Join-Path $IdaPath $_ } |
        Where-Object { Test-Path $_ }
    if ($dllCandidates -and (Test-Path (Join-Path $IdaPath "python"))) {
        Write-Warn "idalib directory not found, but IDA files detected at root: $IdaPath"
        $IdalibDir = $IdaPath
    } else {
        Fail "idalib directory not found at: $IdalibDir`n  Requires IDA Pro 9.0+ or IDA Essential 9.2+"
    }
}

Write-Ok "Found idalib at: $IdalibDir"

# ---------------------------------------------------------------------------
# 3. Install idalib Python package
# ---------------------------------------------------------------------------
Write-Host ""
Write-Info "Installing idalib Python package..."

$IdalibPython = Join-Path $IdalibDir "python"
if (-not (Test-Path $IdalibPython)) {
    Fail "idalib Python directory not found at: $IdalibPython"
}

try {
    & pip install $IdalibPython
    if ($LASTEXITCODE -eq 0) {
        Write-Ok "idalib Python package installed"
    } else {
        Write-Warn "pip install returned non-zero - idalib might already be installed"
    }
} catch {
    Write-Warn "pip install error: $_"
}

# ---------------------------------------------------------------------------
# 4. Activate idalib
# ---------------------------------------------------------------------------
Write-Host ""
Write-Info "Activating idalib..."

$ActivateScript = Join-Path $IdalibPython "py-activate-idalib.py"
if (-not (Test-Path $ActivateScript)) {
    Write-Warn "Activation script not found at $ActivateScript - skipping"
} else {
    try {
        & python $ActivateScript -d $IdaPath
        if ($LASTEXITCODE -eq 0) {
            Write-Ok "idalib activated"
        } else {
            Fail "Failed to activate idalib"
        }
    } catch {
        Fail "Failed to activate idalib: $_"
    }
}

# ---------------------------------------------------------------------------
# 5. Verify
# ---------------------------------------------------------------------------
Write-Host ""
Write-Info "Testing idalib import..."

$result = & python -c "import idapro; print('idalib ready')" 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Ok "idalib is importable"
} else {
    Write-Host "[-] Failed to import idalib." -ForegroundColor Red
    Write-Host "    IDA DLLs must be on PATH. Add to your PowerShell profile:" -ForegroundColor Yellow
    Write-Host "      `$env:PATH = `"$IdaPath;`$env:PATH`"" -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Ok "Setup complete!"
Write-Host ""
Write-Info "If you encounter DLL errors at runtime, add IDA to PATH:"
Write-Host "  In PowerShell: " -NoNewline
Write-Host "`$env:PATH = `"$IdaPath;`$env:PATH`"" -ForegroundColor Yellow
