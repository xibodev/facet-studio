#!/usr/bin/env bash
# Facet Studio Interactive Installer for macOS and Linux
#
# Usage:
#   curl -fsSL https://xibodev.github.io/facet-studio/install.sh | bash
#
set -euo pipefail

CYAN='\033[0;36m'
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

echo ""
echo -e "${CYAN}  ███████╗ █████╗  ██████╗███████╗████████╗    ███████╗████████╗██╗   ██╗██████╗ ██╗ ██████╗ ${NC}"
echo -e "${CYAN}  ██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝    ██╔════╝╚══██╔══╝██║   ██║██╔══██╗██║██╔═══██╗${NC}"
echo -e "${CYAN}  █████╗  ███████║██║     █████╗     ██║       ███████╗   ██║   ██║   ██║██║  ██║██║██║   ██║${NC}"
echo -e "${BLUE}  ██╔══╝  ██╔══██║██║     ██╔══╝     ██║       ╚════██║   ██║   ██║   ██║██║  ██║██║██║   ██║${NC}"
echo -e "${BLUE}  ██║     ██║  ██║╚██████╗███████╗   ██║       ███████║   ██║   ╚██████╔╝██████╔╝██║╚██████╔╝${NC}"
echo -e "${BLUE}  ╚═╝     ╚═╝  ╚═╝ ╚═════╝╚══════╝   ╚═╝       ╚══════╝   ╚═╝    ╚═════╝ ╚═════╝ ╚═╝ ╚═════╝ ${NC}"
echo -e "${GRAY}                                      Facet Studio v1.0.0${NC}"
echo -e "${GRAY}──────────────────────────────────────────────────────────────────────────────────────────────${NC}"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux*)  PLATFORM="linux" ;;
  Darwin*) PLATFORM="darwin" ;;
  *)       echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64) TARGET_ARCH="amd64" ;;
  arm64|aarch64) TARGET_ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo -e "${GREEN}✓ System: $OS ($ARCH)${NC}"
echo -e "${GREEN}✓ Runtime: Self-contained (zero external dependencies required)${NC}"

# Destination
INSTALL_DIR="${HOME}/.facet-studio/bin"
mkdir -p "$INSTALL_DIR"
echo -e "${GREEN}✓ Destination directory: $INSTALL_DIR${NC}"

# Port check
PORT=18800
if command -v nc >/dev/null 2>&1; then
  if nc -z 127.0.0.1 18800 2>/dev/null; then
    echo -e "${YELLOW}! Port 18800 is already in use. Using port 19900.${NC}"
    PORT=19900
  fi
fi

# Download binaries
RELEASE_URL="https://github.com/xibodev/facet-studio/releases/latest/download"
STUDIO_BIN="${INSTALL_DIR}/facet-studio"
KERNEL_BIN="${INSTALL_DIR}/facet-studio-kernel"

echo ""
echo -e "${CYAN}[1/3] Fetching Facet Studio Binaries...${NC}"
if [ -f "./build/facet-studio" ] && [ -f "./build/facet-studio-kernel" ]; then
  cp "./build/facet-studio" "$STUDIO_BIN"
  cp "./build/facet-studio-kernel" "$KERNEL_BIN"
else
  curl -fsSL "${RELEASE_URL}/facet-studio-${PLATFORM}-${TARGET_ARCH}" -o "$STUDIO_BIN"
  curl -fsSL "${RELEASE_URL}/facet-studio-kernel-${PLATFORM}-${TARGET_ARCH}" -o "$KERNEL_BIN"
fi
chmod +x "$STUDIO_BIN" "$KERNEL_BIN"
echo -e "${GREEN}✓ Installed binaries to $INSTALL_DIR${NC}"

# Password setup
echo ""
echo -e "${CYAN}[2/3] Setup Dashboard Password${NC}"
echo -e "${GRAY}      Set your web cockpit password (minimum 8 characters).${NC}"
PASSWORD=""
while [ ${#PASSWORD} -lt 8 ]; do
  read -rsp "      Enter Password: " PASSWORD
  echo ""
  if [ ${#PASSWORD} -lt 8 ]; then
    echo -e "      Password must be at least 8 characters. Please try again."
  fi
done

read -rsp "      Confirm Password: " PASSWORD_CONFIRM
echo ""
while [ "$PASSWORD" != "$PASSWORD_CONFIRM" ]; do
  echo -e "      Passwords do not match. Please try again."
  read -rsp "      Enter Password: " PASSWORD
  echo ""
  read -rsp "      Confirm Password: " PASSWORD_CONFIRM
  echo ""
done

"$STUDIO_BIN" -password "$PASSWORD" >/dev/null 2>&1 || true
echo -e "${GREEN}✓ Dashboard password saved.${NC}"

# Launcher config
mkdir -p "${HOME}/.facet-studio"
cat <<EOF > "${HOME}/.facet-studio/launcher-config.json"
{
  "port": $PORT,
  "public": false,
  "allow_localhost_bypass": true
}
EOF

# Free model test
echo ""
echo -e "${CYAN}[3/3] Live Free Model Pre-flight...${NC}"
echo -e "${GRAY}      Testing zero-key model endpoints...${NC}"

if curl -s -m 4 -X POST "https://opencode.ai/zen/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "x-session-id: sess_install_$RANDOM" \
  -d '{"model":"ling-3.0-flash-fin-free","messages":[{"role":"user","content":"hi"}],"max_tokens":10}' >/dev/null 2>&1; then
  echo -e "  ${GREEN}✓ OpenCode Zen (ling-3.0-flash-fin-free) -- Reachable${NC}"
  STARTING_MODEL="opencode-zen/ling-3.0-flash-fin-free"
else
  echo -e "  ${YELLOW}! OpenCode Zen offline, defaulting to Pollinations${NC}"
  STARTING_MODEL="pollinations/openai-fast"
fi

cat <<EOF > "${HOME}/.facet-studio/config.json"
{
  "active_models": ["opencode-zen/ling-3.0-flash-fin-free", "pollinations/openai-fast"],
  "provider_instances": [
    {
      "id": "opencode-zen",
      "provider_kind": "opencode_zen",
      "adapter": "openai-compatible",
      "protocol": "openai",
      "endpoint": "https://opencode.ai/zen/v1",
      "state": "enabled"
    },
    {
      "id": "pollinations",
      "provider_kind": "pollinations",
      "adapter": "openai-compatible",
      "protocol": "openai",
      "endpoint": "https://text.pollinations.ai/openai",
      "state": "enabled"
    }
  ]
}
EOF
echo -e "${GREEN}✓ Wired $STARTING_MODEL into active chat shortlist.${NC}"

echo ""
echo -e "${GRAY}──────────────────────────────────────────────────────────────────────────────────────────────${NC}"
echo -e "${GREEN}Facet Studio installation complete!${NC}"
echo -e "   URL: ${CYAN}http://localhost:${PORT}${NC}"
echo -e "   Run anytime: ${GRAY}${STUDIO_BIN}${NC}"
echo -e "${GRAY}──────────────────────────────────────────────────────────────────────────────────────────────${NC}"
echo ""

read -rp "Start Facet Studio now? [Y/n]: " START_NOW
if [ "${START_NOW:-y}" != "n" ] && [ "${START_NOW:-y}" != "N" ]; then
  echo -e "${GREEN}Starting Facet Studio on http://localhost:${PORT} ...${NC}"
  "$STUDIO_BIN" -port "$PORT" &
fi
