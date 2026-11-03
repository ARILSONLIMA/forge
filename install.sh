#!/usr/bin/env bash
#
# 🔨 Forge Installer & Ecosystem Setup Script
# Usage:
#   ./install.sh [options]
#   curl -fsSL https://raw.githubusercontent.com/ARILSONLIMA/forge/main/install.sh | bash
#

set -e

# --- Terminal Styling ---
BOLD='\033[1m'
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
RESET='\033[0m'

log_info() {
    printf "${BLUE}==>${RESET} %b\n" "$1"
}

log_success() {
    printf "${GREEN} ✓ ${RESET} %b\n" "$1"
}

log_warn() {
    printf "${YELLOW} ! ${RESET} %b\n" "$1"
}

log_error() {
    printf "${RED} ✗ ${RESET} %b\n" "$1" >&2
}

print_banner() {
    printf "${CYAN}${BOLD}"
    cat << 'EOF'
  ███████╗ ██████╗ ██████╗  ██████╗ ███████╗
  ██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝
  █████╗  ██║   ██║██████╔╝██║  ███╗█████╗  
  ██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝  
  ██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗
  ╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
EOF
    printf "${RESET}"
    printf "${BOLD}Deterministic AI Engineering Harness - Installer & Setup${RESET}\n"
    printf "========================================================\n\n"
}

# --- Default Options ---
AUTO_APPROVE=0
ONLY_FORGE=0
INIT_PROJECT=0
BIN_DIR="${HOME}/.local/bin"

if [ "$(id -u)" -eq 0 ]; then
    BIN_DIR="/usr/local/bin"
fi

# Parse CLI arguments
while [ "$#" -gt 0 ]; do
    case "$1" in
        -y|--yes|--all)
            AUTO_APPROVE=1
            shift
            ;;
        --no-deps)
            ONLY_FORGE=1
            shift
            ;;
        --init)
            INIT_PROJECT=1
            shift
            ;;
        --bin-dir=*)
            BIN_DIR="${1#*=}"
            shift
            ;;
        --bin-dir)
            BIN_DIR="$2"
            shift 2
            ;;
        -h|--help)
            print_banner
            printf "Usage: ./install.sh [options]\n\n"
            printf "Options:\n"
            printf "  -y, --yes, --all       Install all tools and dependencies automatically without prompts\n"
            printf "  --bin-dir <DIR>        Directory to install binaries (default: ~/.local/bin)\n"
            printf "  --no-deps              Install only the Forge binary, skip external tools\n"
            printf "  --init                 Initialize Forge ('forge init') in current project after install\n"
            printf "  -h, --help             Show this help message\n\n"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

print_banner

# --- Detect OS & Arch ---
OS_TYPE="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_TYPE="$(uname -m)"

case "$ARCH_TYPE" in
    x86_64|amd64)
        ARCH_NORM="x86_64"
        GO_ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH_NORM="aarch64"
        GO_ARCH="arm64"
        ;;
    *)
        log_error "Unsupported architecture: $ARCH_TYPE"
        exit 1
        ;;
esac

log_info "Detected platform: ${BOLD}${OS_TYPE} (${ARCH_NORM})${RESET}"
log_info "Target binary directory: ${BOLD}${BIN_DIR}${RESET}"
mkdir -p "${BIN_DIR}"

# --- Helper for Prompts ---
prompt_user() {
    local prompt_msg="$1"
    local default_ans="${2:-y}"

    if [ "$AUTO_APPROVE" -eq 1 ]; then
        return 0
    fi

    # Read from /dev/tty if stdin is piped (e.g. curl ... | bash)
    local response
    if [ -t 0 ]; then
        read -r -p "$prompt_msg " response
    elif [ -e /dev/tty ]; then
        read -r -p "$prompt_msg " response < /dev/tty
    else
        return 0
    fi

    response="${response:-$default_ans}"
    case "$response" in
        [yY][eE][sS]|[yY]|[sS]|[sS][iI][mM]|"")
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# --- Step 1: PATH Setup ---
ensure_path() {
    log_info "Verifying PATH environment..."
    case ":$PATH:" in
        *":$BIN_DIR:"*)
            log_success "${BIN_DIR} is already in your PATH"
            ;;
        *)
            log_warn "${BIN_DIR} is not in your current PATH"
            local export_line="export PATH=\"${BIN_DIR}:\$HOME/go/bin:\$PATH\""
            local updated=0

            for rc in "${HOME}/.bashrc" "${HOME}/.zshrc" "${HOME}/.profile"; do
                if [ -f "$rc" ]; then
                    if ! grep -q "${BIN_DIR}" "$rc"; then
                        printf "\n# Forge CLI and ecosystem tools\n%s\n" "$export_line" >> "$rc"
                        log_success "Appended PATH to $rc"
                        updated=1
                    fi
                fi
            done

            export PATH="${BIN_DIR}:${HOME}/go/bin:$PATH"
            if [ "$updated" -eq 1 ]; then
                log_info "Environment updated! You can run ${BOLD}source ~/.bashrc${RESET} or open a new terminal."
            fi
            ;;
    esac
}

ensure_path

# --- Step 2: Install Forge CLI ---
install_forge() {
    log_info "Compiling / Installing Forge CLI..."

    # Check if we are inside the forge repository
    if [ -f "./cmd/forge/main.go" ] && command -v go >/dev/null 2>&1; then
        log_info "Building forge from local source..."
        go build -ldflags "-s -w" -o "${BIN_DIR}/forge" ./cmd/forge
        log_success "Forge CLI installed to ${BIN_DIR}/forge"
    elif command -v go >/dev/null 2>&1; then
        log_info "Installing forge via 'go install github.com/ARILSONLIMA/forge/cmd/forge@latest'..."
        go install github.com/ARILSONLIMA/forge/cmd/forge@latest
        if [ -f "${HOME}/go/bin/forge" ]; then
            cp "${HOME}/go/bin/forge" "${BIN_DIR}/forge"
            log_success "Forge CLI copied to ${BIN_DIR}/forge"
        fi
    else
        log_warn "Go compiler not found. Searching for local prebuilt 'forge'..."
        if [ -f "./forge" ]; then
            cp "./forge" "${BIN_DIR}/forge"
            chmod +x "${BIN_DIR}/forge"
            log_success "Installed local prebuilt forge to ${BIN_DIR}/forge"
        else
            log_error "Go is required to compile Forge. Please install Go from https://go.dev/dl"
            return 1
        fi
    fi

    chmod +x "${BIN_DIR}/forge"
}

install_forge

if [ "$ONLY_FORGE" -eq 1 ]; then
    log_success "Forge binary installed (--no-deps specified)."
    exit 0
fi

# --- Step 3: Ecosystem Dependencies ---
log_info "Checking ecosystem dependencies..."

# 3.1 Git
if command -v git >/dev/null 2>&1; then
    log_success "git is installed ($(git --version))"
else
    log_warn "git is missing."
    if prompt_user "Install git via package manager now? [Y/n]"; then
        if command -v apt-get >/dev/null 2>&1; then
            sudo apt-get update && sudo apt-get install -y git
        elif command -v dnf >/dev/null 2>&1; then
            sudo dnf install -y git
        elif command -v pacman >/dev/null 2>&1; then
            sudo pacman -S --noconfirm git
        elif command -v brew >/dev/null 2>&1; then
            brew install git
        else
            log_error "Please install git manually with your package manager."
        fi
    fi
fi

# 3.2 uv / uvx (Fast Python package manager for Serena MCP)
if command -v uvx >/dev/null 2>&1 || command -v uv >/dev/null 2>&1; then
    log_success "uv / uvx is installed ($(uv --version 2>/dev/null || echo 'ok'))"
else
    log_warn "uvx / uv (Serena MCP runtime) is missing."
    if prompt_user "Install uv & uvx automatically? [Y/n]"; then
        log_info "Installing uv via official installer (https://astral.sh/uv)..."
        curl -LsSf https://astral.sh/uv/install.sh | sh
        export PATH="${HOME}/.local/bin:$PATH"
        log_success "uv & uvx installed successfully"
    fi
fi

# 3.3 Beads (bd - Distributed DAG Task Tracker)
if command -v bd >/dev/null 2>&1 || command -v beads >/dev/null 2>&1; then
    log_success "bd (Beads DAG) is installed ($(bd version 2>/dev/null | head -n1 || echo 'ok'))"
else
    log_warn "bd (Beads DAG) is missing."
    if prompt_user "Install bd (Beads) automatically? [Y/n]"; then
        log_info "Installing bd via official gastownhall/beads installer..."
        curl -fsSL https://raw.githubusercontent.com/gastownhall/beads/main/scripts/install.sh | bash
        export PATH="${HOME}/.local/bin:$PATH"
        log_success "bd (Beads DAG) installed successfully"
    fi
fi

# 3.4 RTK (Rust Token Killer CLI Proxy)
if command -v rtk >/dev/null 2>&1; then
    log_success "rtk is installed ($(rtk --version 2>/dev/null || echo 'ok'))"
else
    log_warn "rtk (Rust Token Killer) is missing."
    if prompt_user "Install rtk binary automatically? [Y/n]"; then
        log_info "Downloading RTK binary from rtk-ai/rtk GitHub releases..."
        RTK_TAR=""
        case "${OS_TYPE}_${ARCH_NORM}" in
            linux_x86_64)
                RTK_TAR="rtk-x86_64-unknown-linux-musl.tar.gz"
                ;;
            linux_aarch64)
                RTK_TAR="rtk-aarch64-unknown-linux-gnu.tar.gz"
                ;;
            darwin_x86_64)
                RTK_TAR="rtk-x86_64-apple-darwin.tar.gz"
                ;;
            darwin_aarch64)
                RTK_TAR="rtk-aarch64-apple-darwin.tar.gz"
                ;;
            *)
                log_warn "No precompiled RTK release for ${OS_TYPE}_${ARCH_NORM}. Trying cargo..."
                if command -v cargo >/dev/null 2>&1; then
                    cargo install rtk-cli
                fi
                ;;
        esac

        if [ -n "$RTK_TAR" ]; then
            TMP_DIR="$(mktemp -d)"
            curl -fsSL "https://github.com/rtk-ai/rtk/releases/latest/download/${RTK_TAR}" | tar -xz -C "${TMP_DIR}"
            mv "${TMP_DIR}/rtk" "${BIN_DIR}/rtk"
            chmod +x "${BIN_DIR}/rtk"
            rm -rf "${TMP_DIR}"
            log_success "rtk installed to ${BIN_DIR}/rtk"
        fi
    fi
fi

# 3.5 ast-grep (sg - AST Structural Search & Lint)
if command -v ast-grep >/dev/null 2>&1 || command -v sg >/dev/null 2>&1; then
    log_success "ast-grep / sg is installed ($(ast-grep --version 2>/dev/null || sg --version 2>/dev/null || echo 'ok'))"
else
    log_warn "ast-grep (sg) is missing."
    if prompt_user "Install ast-grep / sg automatically? [Y/n]"; then
        if command -v npm >/dev/null 2>&1; then
            log_info "Installing @ast-grep/cli via npm..."
            npm install -g @ast-grep/cli
            log_success "ast-grep installed via npm"
        else
            log_info "Downloading ast-grep release from GitHub..."
            SG_ZIP=""
            case "${OS_TYPE}_${ARCH_NORM}" in
                linux_x86_64)
                    SG_ZIP="app-x86_64-unknown-linux-gnu.zip"
                    ;;
                linux_aarch64)
                    SG_ZIP="app-aarch64-unknown-linux-gnu.zip"
                    ;;
                darwin_x86_64)
                    SG_ZIP="app-x86_64-apple-darwin.zip"
                    ;;
                darwin_aarch64)
                    SG_ZIP="app-aarch64-apple-darwin.zip"
                    ;;
            esac

            if [ -n "$SG_ZIP" ]; then
                TMP_DIR="$(mktemp -d)"
                curl -fsSL "https://github.com/ast-grep/ast-grep/releases/latest/download/${SG_ZIP}" -o "${TMP_DIR}/sg.zip"
                unzip -q "${TMP_DIR}/sg.zip" -d "${TMP_DIR}"
                mv "${TMP_DIR}/ast-grep" "${BIN_DIR}/ast-grep"
                chmod +x "${BIN_DIR}/ast-grep"
                ln -sf "${BIN_DIR}/ast-grep" "${BIN_DIR}/sg"
                rm -rf "${TMP_DIR}"
                log_success "ast-grep and sg installed to ${BIN_DIR}"
            fi
        fi
    fi
fi

# 3.6 Antigravity CLI (agy)
if command -v agy >/dev/null 2>&1; then
    log_success "agy (Google Antigravity CLI) is installed ($(agy --version 2>/dev/null || echo 'ok'))"
else
    log_warn "agy (Google Antigravity CLI) is not detected in PATH."
    log_info "To install agy, check https://github.com/google/antigravity or your internal distribution."
fi

# --- Step 4: Project Configuration ---
if [ ! -f ".forge/config.yaml" ]; then
    printf "\n"
    if [ "$INIT_PROJECT" -eq 1 ] || prompt_user "Would you like to initialize Forge in the current directory now ('forge init')? [Y/n]"; then
        "${BIN_DIR}/forge" init
    fi
fi

# --- Step 5: Final Health Check ---
printf "\n"
log_info "Running final diagnostic check with 'forge doctor'...\n"
"${BIN_DIR}/forge" doctor || true

printf "\n${GREEN}${BOLD}🎉 Forge and ecosystem setup complete!${RESET}\n"
printf "You are ready to start: run ${CYAN}${BOLD}forge status${RESET} or ${CYAN}${BOLD}forge stage spec${RESET} (or slash command ${CYAN}/spec${RESET} in Antigravity).\n\n"
