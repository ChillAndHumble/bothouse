#!/usr/bin/env bash
# Install the CoNest DSH plugins (conest-collab + Discord UI) into a DSH profile.
# Usage: dsh/install.sh [profile]   (default profile: web; honours $DSH_HOME)
set -euo pipefail
PROFILE="${1:-web}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UI="$HERE/packages/client-ui-discord"

command -v dsh >/dev/null || { echo "dsh not found on PATH; install DeepSeek Harness first" >&2; exit 1; }
if ! command -v pnpm >/dev/null; then
  # npm's global bin may simply be missing from PATH (pnpm already installed there).
  NPM_BIN="$(npm prefix -g)/bin"
  export PATH="$NPM_BIN:$PATH"
  if ! command -v pnpm >/dev/null; then
    echo "== pnpm not found, installing with npm"
    npm i -g pnpm
  fi
fi

# dsh-web-app must match the running dsh version, so take it from the local dsh install.
DSH_BIN="$(command -v dsh)"
DSH_JS="$(node -e 'console.log(require("fs").realpathSync(process.argv[1]))' "$DSH_BIN")"
DSH_DIR="$(cd "$(dirname "$DSH_JS")/.." && pwd)"
WEB_APP="$DSH_DIR/node_modules/@deepseek-ai/dsh-web-app"
[ -d "$WEB_APP" ] || { echo "dsh-web-app not found under $DSH_DIR" >&2; exit 1; }

echo "== build client-ui-discord"
(cd "$UI" && pnpm install --ignore-workspace && pnpm run bundle)

echo "== install into profile $PROFILE"
dsh plugin --profile "$PROFILE" add "link:$WEB_APP" "link:$HERE/collab" "link:$UI"
echo "== done; check with: dsh --profile $PROFILE --dump-config"
