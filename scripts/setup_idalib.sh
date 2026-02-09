#!/bin/bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[*]${NC} $*"; }
ok()    { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
fail()  { echo -e "${RED}[-]${NC} $*"; exit 1; }

# ---------------------------------------------------------------------------
# 1. Detect OS
# ---------------------------------------------------------------------------
OS="$(uname -s)"
info "Detected OS: $OS"

# ---------------------------------------------------------------------------
# 2. Locate IDA installation
# ---------------------------------------------------------------------------
# Users can override with IDA_PATH environment variable
if [ -n "${IDA_PATH:-}" ] && [ -d "$IDA_PATH" ]; then
    ok "Using IDA_PATH from environment: $IDA_PATH"
else
    IDA_PATH=""

    # Candidate directories to search, ordered by preference (newest first)
    if [ "$OS" = "Darwin" ]; then
        CANDIDATES=(
            "/Applications/IDA Professional 9.2.app/Contents/MacOS"
            "/Applications/IDA Essential 9.2.app/Contents/MacOS"
            "/Applications/IDA Professional 9.1.app/Contents/MacOS"
            "/Applications/IDA Essential 9.1.app/Contents/MacOS"
            "/Applications/IDA Professional 9.0.app/Contents/MacOS"
            "/Applications/IDA Pro 9.0.app/Contents/MacOS"
            "$HOME/Applications/IDA Professional 9.2.app/Contents/MacOS"
            "$HOME/Applications/IDA Essential 9.2.app/Contents/MacOS"
        )
    else
        # Linux: common installation paths
        CANDIDATES=(
            "$HOME/idapro-9.2"
            "$HOME/idapro-9.1"
            "$HOME/idapro-9.0"
            "$HOME/ida-pro-9.2"
            "$HOME/ida-pro-9.1"
            "$HOME/ida-pro-9.0"
            "/opt/idapro-9.2"
            "/opt/idapro-9.1"
            "/opt/idapro-9.0"
            "/opt/ida-pro-9.2"
            "/opt/ida-pro-9.1"
            "/opt/ida-pro-9.0"
            "/usr/local/idapro"
            "$HOME/idapro"
        )
    fi

    for candidate in "${CANDIDATES[@]}"; do
        if [ -d "$candidate" ]; then
            IDA_PATH="$candidate"
            break
        fi
    done

    if [ -z "$IDA_PATH" ]; then
        echo ""
        fail "IDA installation not found. Set IDA_PATH environment variable, e.g.:\n  export IDA_PATH=/path/to/idapro-9.x\n  $0"
    fi

    ok "Found IDA at: $IDA_PATH"
fi

# ---------------------------------------------------------------------------
# 3. Locate idalib
# ---------------------------------------------------------------------------
IDALIB_DIR="$IDA_PATH/idalib"

if [ ! -d "$IDALIB_DIR" ]; then
    # Some installations put idalib directly under the IDA root
    if [ -d "$IDA_PATH/python" ] && [ -f "$IDA_PATH/libida64.so" -o -f "$IDA_PATH/libida64.dylib" ]; then
        warn "idalib directory not found, but IDA Python modules detected at $IDA_PATH"
        IDALIB_DIR="$IDA_PATH"
    else
        fail "idalib directory not found at: $IDALIB_DIR\n  Make sure you have IDA Pro 9.0+ or IDA Essential 9.2+ with idalib support."
    fi
fi

ok "Found idalib at: $IDALIB_DIR"

# ---------------------------------------------------------------------------
# 4. Install idalib Python package
# ---------------------------------------------------------------------------
echo ""
info "Installing idalib Python package..."

IDALIB_PYTHON="$IDALIB_DIR/python"
if [ ! -d "$IDALIB_PYTHON" ]; then
    fail "idalib Python directory not found at: $IDALIB_PYTHON"
fi

if pip3 install "$IDALIB_PYTHON" 2>&1; then
    ok "idalib Python package installed"
else
    warn "pip3 install returned non-zero — idalib might already be installed"
fi

# ---------------------------------------------------------------------------
# 5. Activate idalib
# ---------------------------------------------------------------------------
echo ""
info "Activating idalib..."

ACTIVATE_SCRIPT="$IDALIB_PYTHON/py-activate-idalib.py"
if [ ! -f "$ACTIVATE_SCRIPT" ]; then
    warn "Activation script not found at $ACTIVATE_SCRIPT — skipping activation"
else
    if python3 "$ACTIVATE_SCRIPT" -d "$IDA_PATH"; then
        ok "idalib activated"
    else
        fail "Failed to activate idalib"
    fi
fi

# ---------------------------------------------------------------------------
# 6. Verify
# ---------------------------------------------------------------------------
echo ""
info "Testing idalib import..."

if python3 -c "import idapro; print('idalib ready')" 2>/dev/null; then
    ok "idalib is importable"
else
    fail "Failed to import idalib. Check that IDA libraries are on LD_LIBRARY_PATH (Linux) or DYLD_LIBRARY_PATH (macOS)."
fi

echo ""
ok "Setup complete!"
echo ""
info "If you encounter shared library errors at runtime, add this to your shell profile:"
if [ "$OS" = "Darwin" ]; then
    echo "  export DYLD_LIBRARY_PATH=\"$IDA_PATH:\$DYLD_LIBRARY_PATH\""
else
    echo "  export LD_LIBRARY_PATH=\"$IDA_PATH:\$LD_LIBRARY_PATH\""
fi
