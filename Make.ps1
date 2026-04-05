#Requires -Version 5.1
<#
.SYNOPSIS
    Windows PowerShell equivalent of the project Makefile.
.DESCRIPTION
    Supports the same targets as the Makefile: build, clean, test, run,
    install-tools, install-python, setup-idalib, setup, proto, inspector.
.PARAMETER Target
    Build target to execute (default: setup)
.EXAMPLE
    .\Make.ps1 build
    .\Make.ps1 setup
    .\Make.ps1 clean
#>
param(
    [Parameter(Position=0)]
    [ValidateSet('build','clean','test','test-all','integration-test','run',
                 'proto','proto-check','install-tools','install-python',
                 'setup-idalib','setup','inspector',
                 'service-install','service-uninstall','service-start',
                 'service-stop','service-restart','service-status','help')]
    [string]$Target = 'help',

    # service-* 타겟에 전달할 선택적 인수
    [string]$ServiceName = 'IDAHeadlessMCP',
    [int]$ServicePort = 17300,
    [string]$IdaPath = $env:IDA_PATH
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ProjectRoot = $PSScriptRoot

function Write-Step($msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "[+] $msg" -ForegroundColor Green }
function Write-Fail($msg) { Write-Host "[-] $msg" -ForegroundColor Red; exit 1 }

function Require-Command($name) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) {
        Write-Fail "'$name' is not installed or not on PATH. Please install it first."
    }
}

# ---------------------------------------------------------------------------
function Target-Build {
    Write-Step "Building Go server..."
    Require-Command "go"

    $binDir = Join-Path $ProjectRoot "bin"
    if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }

    $outPath = Join-Path $binDir "ida-mcp-server.exe"
    & go build -o $outPath .\cmd\ida-mcp-server
    if ($LASTEXITCODE -ne 0) { Write-Fail "go build failed" }
    Write-Ok "Built: $outPath"
}

# ---------------------------------------------------------------------------
function Target-Clean {
    Write-Step "Cleaning build artifacts..."

    $binDir = Join-Path $ProjectRoot "bin"
    if (Test-Path $binDir) {
        Remove-Item -Recurse -Force $binDir
        Write-Ok "Removed: bin\"
    }

    Get-ChildItem (Join-Path $ProjectRoot "ida\worker\v1") -Filter "*.pb.go" -ErrorAction SilentlyContinue |
        Remove-Item -Force
    Write-Ok "Removed .pb.go files"

    Get-ChildItem (Join-Path $ProjectRoot "python\worker") -Filter "*.pyc" -Recurse -ErrorAction SilentlyContinue |
        Remove-Item -Force
    Get-ChildItem (Join-Path $ProjectRoot "python\worker") -Filter "__pycache__" -Recurse -ErrorAction SilentlyContinue |
        Remove-Item -Recurse -Force
    Write-Ok "Removed Python cache files"
}

# ---------------------------------------------------------------------------
function Target-Test {
    Write-Step "Running unit tests..."
    Require-Command "go"

    & go test -v .\internal\... .\ida\...
    if ($LASTEXITCODE -ne 0) { Write-Fail "Go tests failed" }

    Write-Step "Running consistency checks..."
    $consistencyScript = Join-Path $ProjectRoot "scripts\consistency.ps1"
    & powershell -ExecutionPolicy Bypass -File $consistencyScript `
        -RulesFile (Join-Path $ProjectRoot "consistency.yaml")
    if ($LASTEXITCODE -ne 0) { Write-Fail "Consistency check failed" }
}

# ---------------------------------------------------------------------------
function Target-TestAll {
    Write-Step "Running all tests (including integration)..."
    Require-Command "go"

    & go test -v -tags=integration .\...
    if ($LASTEXITCODE -ne 0) { Write-Fail "Tests failed" }

    $consistencyScript = Join-Path $ProjectRoot "scripts\consistency.ps1"
    & powershell -ExecutionPolicy Bypass -File $consistencyScript `
        -RulesFile (Join-Path $ProjectRoot "consistency.yaml")
}

# ---------------------------------------------------------------------------
function Target-IntegrationTest {
    Write-Step "Running integration tests (MCP transports)..."
    Require-Command "go"

    & go test .\internal\server -run TestStreamableHTTPTransportLifecycle -v
    if ($LASTEXITCODE -ne 0) { Write-Fail "Integration test failed" }
    & go test .\internal\server -run TestSSETransportLifecycle -v
    if ($LASTEXITCODE -ne 0) { Write-Fail "Integration test failed" }
}

# ---------------------------------------------------------------------------
function Target-Run {
    Target-Build
    Write-Step "Starting server..."
    $exe = Join-Path $ProjectRoot "bin\ida-mcp-server.exe"
    & $exe
}

# ---------------------------------------------------------------------------
function Target-Proto {
    Write-Step "Regenerating protobuf files..."
    Require-Command "protoc"
    Require-Command "go"

    $gopath = & go env GOPATH
    $env:PATH = "$gopath\bin;$env:PATH"
    & go generate .\proto\ida\worker\v1
    if ($LASTEXITCODE -ne 0) { Write-Fail "proto generation failed" }
    Write-Ok "Protobuf generation complete"
}

# ---------------------------------------------------------------------------
function Target-ProtoCheck {
    Target-Proto
    Write-Step "Checking for proto drift..."
    $diff = & git diff --exit-code proto ida python/worker/gen 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host $diff
        Write-Fail "Proto files are out of date. Run: .\Make.ps1 proto"
    }
    Write-Ok "Proto files are up to date"
}

# ---------------------------------------------------------------------------
function Target-InstallTools {
    Write-Step "Installing Go protoc plugins..."
    Require-Command "go"
    & go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    & go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
    Write-Ok "Tools installed"
}

# ---------------------------------------------------------------------------
function Target-InstallPython {
    Write-Step "Installing Python dependencies..."
    & pip install -r (Join-Path $ProjectRoot "python\requirements.txt")
    if ($LASTEXITCODE -ne 0) { Write-Fail "pip install failed" }
    Write-Ok "Python dependencies installed"
}

# ---------------------------------------------------------------------------
function Target-SetupIdalib {
    Write-Step "Setting up idalib..."
    $script = Join-Path $ProjectRoot "scripts\setup_idalib.ps1"
    & powershell -ExecutionPolicy Bypass -File $script
    if ($LASTEXITCODE -ne 0) { Write-Fail "idalib setup failed" }
}

# ---------------------------------------------------------------------------
function Target-Setup {
    Target-SetupIdalib
    Target-InstallPython
    Target-Build
    Write-Host ""
    Write-Ok "Setup complete. Run: .\bin\ida-mcp-server.exe"
}

# ---------------------------------------------------------------------------
function Target-Inspector {
    Write-Step "Launching MCP Inspector..."
    Require-Command "npx"
    & npx @modelcontextprotocol/inspector
}

# ---------------------------------------------------------------------------
function Invoke-ServiceScript($action) {
    $script = Join-Path $ProjectRoot "scripts\service.ps1"
    $args = @($action, "-ServiceName", $ServiceName, "-Port", $ServicePort)
    if ($IdaPath) { $args += @("-IdaPath", $IdaPath) }

    $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
        [Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isAdmin) {
        Write-Host "[-] Service management requires administrator privileges." -ForegroundColor Red
        Write-Host "    Run PowerShell as Administrator and try again." -ForegroundColor Yellow
        exit 1
    }
    & powershell -ExecutionPolicy Bypass -File $script @args
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

# ---------------------------------------------------------------------------
function Target-Help {
    Write-Host @"

IDA Headless MCP - Windows Build Script
Usage: .\Make.ps1 <target> [-ServiceName <name>] [-ServicePort <port>] [-IdaPath <path>]

Build Targets:
  build             Build Go server binary -> bin\ida-mcp-server.exe
  clean             Clean build artifacts
  test              Unit tests + consistency checks
  test-all          Full test including integration tests
  integration-test  MCP transport integration tests
  run               Run server after build
  proto             Regenerate protobuf files (protoc required)
  proto-check       Check for proto file changes
  install-tools     Install Go protoc plugins
  install-python    Python dependencies installation
  setup-idalib      Setup idalib (Windows IDA path auto-detection)
  setup             Full setup: idalib + python + build
  inspector         Run MCP Inspector (npx required)

Service Targets (administrator privileges required):
  service-install   Register and start as Windows service
  service-uninstall Service stop and remove
  service-start     Service start
  service-stop      Service stop
  service-restart   Service restart
  service-status    Check service status

Service Options:
  -ServiceName      Service name (default: IDAHeadlessMCP)
  -ServicePort      Port number (default: 17300)
  -IdaPath          IDA installation path (can also be set via IDA_PATH environment variable)

Examples:
  .\Make.ps1 build
  .\Make.ps1 service-install
  .\Make.ps1 service-install -IdaPath "C:\Program Files\IDA Pro 9.2" -ServicePort 17300
  .\Make.ps1 service-status
  .\Make.ps1 service-uninstall
  help              Display this help message

"@
}

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------
Set-Location $ProjectRoot

switch ($Target) {
    'build'            { Target-Build }
    'clean'            { Target-Clean }
    'test'             { Target-Test }
    'test-all'         { Target-TestAll }
    'integration-test' { Target-IntegrationTest }
    'run'              { Target-Run }
    'proto'            { Target-Proto }
    'proto-check'      { Target-ProtoCheck }
    'install-tools'    { Target-InstallTools }
    'install-python'   { Target-InstallPython }
    'setup-idalib'     { Target-SetupIdalib }
    'setup'            { Target-Setup }
    'inspector'        { Target-Inspector }
    'service-install'   { Invoke-ServiceScript 'install' }
    'service-uninstall' { Invoke-ServiceScript 'uninstall' }
    'service-start'     { Invoke-ServiceScript 'start' }
    'service-stop'      { Invoke-ServiceScript 'stop' }
    'service-restart'   { Invoke-ServiceScript 'restart' }
    'service-status'    { Invoke-ServiceScript 'status' }
    'help'             { Target-Help }
}





