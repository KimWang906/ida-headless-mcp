#Requires -Version 5.1
#Requires -RunAsAdministrator
<#
.SYNOPSIS
    IDA Headless MCP Server - Windows Service Manager (NSSM based)
.DESCRIPTION
    Uses NSSM to register/manage ida-mcp-server.exe as a Windows service.
    Must be run as administrator.
.PARAMETER Action
    install   : Register and start service
    uninstall : Stop and remove service
    start     : Start service
    stop      : Stop service
    restart   : Restart service
    status    : Check service status
.PARAMETER ServiceName
    Service name to register (default: IDAHeadlessMCP)
.PARAMETER Port
    Server port (default: 17300)
.PARAMETER IdaPath
    IDA installation path (used for DLL PATH). If not specified, IDA_PATH environment variable is used.
.EXAMPLE
    .\scripts\service.ps1 install
    .\scripts\service.ps1 install -IdaPath "C:\Program Files\IDA Pro 9.2" -Port 17300
    .\scripts\service.ps1 status
    .\scripts\service.ps1 uninstall
#>
param(
    [Parameter(Position=0, Mandatory)]
    [ValidateSet('install','uninstall','start','stop','restart','status')]
    [string]$Action,

    [string]$ServiceName = 'IDAHeadlessMCP',
    [int]$Port = 17300,
    [string]$IdaPath = $env:IDA_PATH
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ProjectRoot = Split-Path -Parent $PSScriptRoot

function Write-Step($msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "[+] $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "[!] $msg" -ForegroundColor Yellow }
function Write-Fail($msg) { Write-Host "[-] $msg" -ForegroundColor Red; exit 1 }

# ---------------------------------------------------------------------------
# Get NSSM (install with winget if not found)
# ---------------------------------------------------------------------------
function Get-Nssm {
    # 1) PATH에서 먼저 탐색
    $nssm = Get-Command nssm -ErrorAction SilentlyContinue
    if ($nssm) { return $nssm.Source }

    # 2) WinGet 링크 경로 직접 확인 (PATH 등록 전에도 사용 가능)
    $wingetLink = "$env:LOCALAPPDATA\Microsoft\WinGet\Links\nssm.exe"
    if (Test-Path $wingetLink) { return $wingetLink }

    # 3) 없으면 winget으로 설치
    Write-Step "NSSM not found. Installing via winget..."
    winget install NSSM.NSSM --accept-package-agreements --accept-source-agreements | Out-Null
    if ($LASTEXITCODE -ne 0) { Write-Fail "NSSM installation failed" }

    # 4) 설치 후 다시 탐색 (WinGet 링크 우선)
    if (Test-Path $wingetLink) { return $wingetLink }

    $env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" +
                [System.Environment]::GetEnvironmentVariable("PATH","User")

    $nssm = Get-Command nssm -ErrorAction SilentlyContinue
    if ($nssm) { return $nssm.Source }

    Write-Fail "NSSM not found after installation. Please install from https://nssm.cc and add to PATH."
}

# ---------------------------------------------------------------------------
# Check if service exists
# ---------------------------------------------------------------------------
function Service-Exists {
    return [bool](Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)
}

# ---------------------------------------------------------------------------
# install
# ---------------------------------------------------------------------------
function Action-Install {
    Write-Step "Registering IDA Headless MCP service..."

    $exe = Join-Path $ProjectRoot "bin\ida-mcp-server.exe"
    if (-not (Test-Path $exe)) {
        Write-Fail "Binary not found: $exe`n  Please build first: .\Make.ps1 build"
    }

    $workerScript = Join-Path $ProjectRoot "python\worker\server.py"
    if (-not (Test-Path $workerScript)) {
        Write-Fail "Python worker not found: $workerScript"
    }

    # Create log directory
    $logsDir = Join-Path $ProjectRoot "logs"
    if (-not (Test-Path $logsDir)) { New-Item -ItemType Directory -Path $logsDir | Out-Null }

    if (Service-Exists) {
        Write-Warn "Service '$ServiceName' already exists. Removing and reinstalling."
        Action-Uninstall
    }

    $nssm = Get-Nssm

    Write-Step "Configuring service with NSSM..."

    # Basic registration
    & $nssm install $ServiceName $exe
    if ($LASTEXITCODE -ne 0) { Write-Fail "Service registration failed" }

    # Execution arguments
    & $nssm set $ServiceName AppParameters "--worker `"$workerScript`" --port $Port"

    # Working directory (relative to config.json)
    & $nssm set $ServiceName AppDirectory $ProjectRoot

    # Log files
    & $nssm set $ServiceName AppStdout (Join-Path $logsDir "service.log")
    & $nssm set $ServiceName AppStderr (Join-Path $logsDir "service-error.log")
    & $nssm set $ServiceName AppRotateFiles 1
    & $nssm set $ServiceName AppRotateOnline 1
    & $nssm set $ServiceName AppRotateBytes 10485760   # 10 MB

    # Start type: automatic
    & $nssm set $ServiceName Start SERVICE_AUTO_START

    # Restart after 5 seconds if failed
    & $nssm set $ServiceName AppRestartDelay 5000
    & $nssm set $ServiceName AppThrottle 30000

    # Configure environment variables
    $extraEnv = "PYTHONPATH=$ProjectRoot\python\worker\gen;$env:PYTHONPATH"
    if ($IdaPath -and (Test-Path $IdaPath)) {
        # Add IDA DLL to PATH (Windows loads libida64.dll, etc.)
        $extraEnv += "`nPATH=$IdaPath;$env:PATH"
        $extraEnv += "`nIDA_PATH=$IdaPath"
        Write-Ok "IDA path set: $IdaPath"
    } else {
        Write-Warn "IDA_PATH is not set. You need to set IDA_PATH to load idalib in the service."
        $extraEnv += "`nPATH=$env:PATH"
    }
    & $nssm set $ServiceName AppEnvironmentExtra $extraEnv

    # Display name and description
    & $nssm set $ServiceName DisplayName "IDA Headless MCP Server"
    & $nssm set $ServiceName Description "IDA Pro headless binary analysis via Model Context Protocol (port $Port)"

    Write-Ok "Service registration complete"

    # Start service
    Write-Step "Starting service..."
    & $nssm start $ServiceName
    Start-Sleep -Seconds 2
    Action-Status
}

# ---------------------------------------------------------------------------
# uninstall
# ---------------------------------------------------------------------------
function Action-Uninstall {
    Write-Step "Removing service '$ServiceName'..."
    if (-not (Service-Exists)) {
        Write-Warn "Service '$ServiceName' does not exist."
        return
    }

    $nssm = Get-Nssm
    & $nssm stop $ServiceName confirm 2>&1 | Out-Null
    & $nssm remove $ServiceName confirm
    if ($LASTEXITCODE -eq 0) { Write-Ok "Service removal complete" }
    else { Write-Fail "Service removal failed" }
}

# ---------------------------------------------------------------------------
# start / stop / restart
# ---------------------------------------------------------------------------
function Action-Start {
    Write-Step "Starting service '$ServiceName'..."
    if (-not (Service-Exists)) { Write-Fail "Service is not registered. Please run install first." }
    Start-Service -Name $ServiceName
    Start-Sleep -Seconds 2
    Action-Status
}

function Action-Stop {
    Write-Step "Stopping service '$ServiceName'..."
    if (-not (Service-Exists)) { Write-Fail "Service is not registered." }
    Stop-Service -Name $ServiceName -Force
    Start-Sleep -Seconds 1
    Action-Status
}

function Action-Restart {
    Write-Step "Restarting service '$ServiceName'..."
    if (-not (Service-Exists)) { Write-Fail "Service is not registered." }
    Restart-Service -Name $ServiceName -Force
    Start-Sleep -Seconds 2
    Action-Status
}

# ---------------------------------------------------------------------------
# status
# ---------------------------------------------------------------------------
function Action-Status {
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if (-not $svc) {
        Write-Host "   Service '$ServiceName': " -NoNewline
        Write-Host "Not registered" -ForegroundColor DarkGray
        return
    }

    $color = switch ($svc.Status) {
        'Running' { 'Green' }
        'Stopped' { 'Yellow' }
        default   { 'Red' }
    }

    Write-Host "`n   Service name : $($svc.Name)"
    Write-Host "   Display name : $($svc.DisplayName)"
    Write-Host "   Status       : " -NoNewline
    Write-Host $svc.Status -ForegroundColor $color
    Write-Host "   Start type : $($svc.StartType)"

    if ($svc.Status -eq 'Running') {
        $logsDir = Join-Path $ProjectRoot "logs"
        Write-Host "   Log path : $logsDir"
        Write-Host "   Server URL : http://localhost:$Port/"
        Write-Host ""
        Write-Ok "Service is running: http://localhost:$Port/"
    }
}

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------
switch ($Action) {
    'install'   { Action-Install }
    'uninstall' { Action-Uninstall }
    'start'     { Action-Start }
    'stop'      { Action-Stop }
    'restart'   { Action-Restart }
    'status'    { Action-Status }
}

