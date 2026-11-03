#!/usr/bin/env bash
# Screenshot baseline regression for the local React + Hydro shell.
# This script intentionally refuses non-local URLs.
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:5173}
BASELINE_DIR=${BASELINE_DIR:-web/visual-baselines}
CURRENT_DIR=${CURRENT_DIR:-.temp/visual-current}
UPDATE_BASELINES=${UPDATE_BASELINES:-0}
MAX_DIFF_PIXELS=${MAX_DIFF_PIXELS:-5000}
PWCLI=${PWCLI:-"$HOME/.codex/skills/playwright/scripts/playwright_cli.sh"}

if [[ ! "$BASE_URL" =~ ^http://(localhost|127\.0\.0\.1)(:[0-9]+)?/?$ ]]; then
  echo "visual-regression: BASE_URL must be a local HTTP URL, got: $BASE_URL" >&2
  exit 2
fi
if ! command -v compare >/dev/null 2>&1; then
  echo "visual-regression: ImageMagick 'compare' is required" >&2
  exit 2
fi
if [[ ! -f "$PWCLI" ]]; then
  echo "visual-regression: Playwright wrapper not executable: $PWCLI" >&2
  exit 2
fi

mkdir -p "$BASELINE_DIR" "$CURRENT_DIR"
session="visual-$(date +%s)"

routes=(
  "home:/"
  "problems:/p"
  "contests:/contest"
  "homework:/homework"
  "training:/training"
  "discuss:/discuss"
  "ranking:/ranking"
  "about:/about"
  "login:/login"
)

cleanup() {
  bash "$PWCLI" -s="$session" close >/dev/null 2>&1 || true
}
trap cleanup EXIT

bash "$PWCLI" -s="$session" open "$BASE_URL/login" >/dev/null
bash "$PWCLI" -s="$session" resize 1280 900 >/dev/null
bash "$PWCLI" -s="$session" set-reduced-motion reduce >/dev/null

fail=0
for entry in "${routes[@]}"; do
  name=${entry%%:*}
  route=${entry#*:}
  current="$CURRENT_DIR/$name.png"
  baseline="$BASELINE_DIR/$name.png"
  diff="$CURRENT_DIR/$name.diff.png"
  bash "$PWCLI" -s="$session" goto "$BASE_URL$route" >/dev/null
  sleep 0.8
  if [[ "$name" == "home" ]]; then
    # Hitokoto is fetched from a third-party service and must not affect the baseline.
    bash "$PWCLI" -s="$session" eval "() => { const node=document.querySelector('[name=hitokoto]'); if (node) node.parentElement.style.visibility='hidden'; }" >/dev/null || true
  fi
  bash "$PWCLI" -s="$session" screenshot --full-page --filename "$current" >/dev/null
  if [[ "$UPDATE_BASELINES" == "1" ]]; then
    cp "$current" "$baseline"
    [[ ! -f "$diff" ]] || rm "$diff"
    echo "updated  $name"
    continue
  fi
  if [[ ! -f "$baseline" ]]; then
    echo "missing  $name ($baseline); run UPDATE_BASELINES=1 once" >&2
    fail=1
    continue
  fi
  metric=$(compare -metric AE -fuzz 2% "$baseline" "$current" "$diff" 2>&1 || true)
  pixels=${metric//[^0-9]/}
  pixels=${pixels:-0}
  if (( pixels > MAX_DIFF_PIXELS )); then
    echo "changed  $name: $pixels differing pixels (limit $MAX_DIFF_PIXELS)" >&2
    echo "         diff: $diff" >&2
    fail=1
  else
    echo "ok       $name ($pixels differing pixels)"
    [[ ! -f "$diff" ]] || rm "$diff"
  fi
done

if (( fail != 0 )); then
  exit 1
fi
echo "visual-regression: passed"
