#!/usr/bin/env bash
# Screenshot baseline regression for the local React + Hydro shell.
# This script intentionally refuses non-local URLs.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

BASE_URL=${BASE_URL:-http://localhost:5173}
BASELINE_DIR=${BASELINE_DIR:-web/visual-baselines}
CURRENT_DIR=${CURRENT_DIR:-.temp/visual-current}
UPDATE_BASELINES=${UPDATE_BASELINES:-0}
MAX_DIFF_PIXELS=${MAX_DIFF_PIXELS:-5000}
PWCLI=${PWCLI:-"$HOME/.codex/skills/playwright/scripts/playwright_cli.sh"}
VISUAL_AUTH_USERNAME=${VISUAL_AUTH_USERNAME:-}
VISUAL_AUTH_PASSWORD=${VISUAL_AUTH_PASSWORD:-}

parse_ae_pixels() {
  # ImageMagick prints e.g. "1234.5 (0.02)"; only the first value is the
  # differing-pixel count. Never concatenate the parenthesised ratio into it.
  local metric=$1 pixels
  pixels=$(printf '%s\n' "$metric" | awk 'NR == 1 { printf "%d\n", int($1 + 0) }')
  if [[ ! "$pixels" =~ ^[0-9]+$ ]]; then
    echo "visual-regression: could not parse ImageMagick metric: $metric" >&2
    return 2
  fi
  printf '%s\n' "$pixels"
}

image_dimensions() {
  identify -format '%wx%h' "$1"
}

compare_images() {
  local baseline=$1 current=$2 diff=$3
  local baseline_size current_size metric status pixels

  baseline_size=$(image_dimensions "$baseline")
  current_size=$(image_dimensions "$current")
  if [[ "$baseline_size" != "$current_size" ]]; then
    echo "visual-regression: image dimensions differ: $baseline ($baseline_size) vs $current ($current_size)" >&2
    return 2
  fi

  set +e
  metric=$(LC_ALL=C compare -metric AE -fuzz 2% "$baseline" "$current" "$diff" 2>&1)
  status=$?
  set -e
  # ImageMagick returns 1 when images differ and 2 on a real comparison error.
  if [[ $status -gt 1 ]]; then
    echo "visual-regression: ImageMagick comparison failed: $metric" >&2
    return 2
  fi
  pixels=$(parse_ae_pixels "$metric")
  printf '%s\n' "$pixels"
}

require_image_tools() {
  if ! command -v compare >/dev/null 2>&1 || ! command -v identify >/dev/null 2>&1; then
    echo "visual-regression: ImageMagick 'compare' and 'identify' are required" >&2
    exit 2
  fi
}

run_self_test() {
  require_image_tools
  if ! command -v convert >/dev/null 2>&1; then
    echo "visual-regression self-test: ImageMagick 'convert' is required" >&2
    exit 2
  fi

  local tmp pixels
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' RETURN

  [[ $(parse_ae_pixels '1234.5 (0.02)') == 1234 ]]
  [[ $(parse_ae_pixels '0 (0)') == 0 ]]

  convert -size 8x8 xc:white "$tmp/white.png"
  convert -size 8x8 xc:black "$tmp/black.png"
  convert -size 9x8 xc:white "$tmp/wrong-size.png"

  pixels=$(compare_images "$tmp/white.png" "$tmp/white.png" "$tmp/same.diff.png")
  [[ "$pixels" == 0 ]]

  pixels=$(compare_images "$tmp/white.png" "$tmp/black.png" "$tmp/change.diff.png")
  [[ "$pixels" == 64 ]]

  if compare_images "$tmp/white.png" "$tmp/wrong-size.png" "$tmp/size.diff.png" >/dev/null 2>&1; then
    echo "visual-regression self-test: dimension mismatch was accepted" >&2
    exit 1
  fi

  echo "visual-regression self-test: passed"
}

wait_for_ready() {
  local session=$1
  bash "$PWCLI" -s="$session" run-code 'async (page) => {
    await page.waitForLoadState("domcontentloaded");
    await page.waitForFunction(() => {
      const main = document.querySelector(".main");
      if (!main) return false;
      const placeholder = main.querySelector(".nothing-placeholder");
      const loading = Boolean(placeholder && /加载中|loading/i.test(placeholder.textContent ?? ""));
      const loader = document.querySelector(".page-loader");
      const loaderHidden = !loader
        || getComputedStyle(loader).display === "none"
        || Number(getComputedStyle(loader).opacity) === 0;
      const sectionsReady = [...main.querySelectorAll(".section")].every((section) => (
        section.classList.contains("visible")
        || Number(getComputedStyle(section).opacity) >= 0.99
      ));
      return document.readyState === "complete" && !loading && loaderHidden && sectionsReady;
    }, null, { timeout: 15000 });
    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.all([...document.images].map((image) => {
        if (image.complete) return Promise.resolve();
        return new Promise((resolve) => {
          image.addEventListener("load", resolve, { once: true });
          image.addEventListener("error", resolve, { once: true });
        });
      }));
    });
  }' >/dev/null
}

login_for_screenshots() {
  local session=$1
  bash "$PWCLI" -s="$session" run-code "async (page) => {
    const base = '$BASE_URL';
    const response = await page.request.post(base + '/api/v1/auth/login', {
      data: { username: '$VISUAL_AUTH_USERNAME', password: '$VISUAL_AUTH_PASSWORD' },
    });
    if (!response.ok()) {
      throw new Error('visual login failed: HTTP ' + response.status() + ' ' + await response.text());
    }
    const me = await page.request.get(base + '/api/v1/auth/me');
    if (!me.ok()) throw new Error('visual login did not create a session');
    await page.goto(base + '/');
  }" >/dev/null
}

normalize_dynamic_text() {
  local session=$1
  bash "$PWCLI" -s="$session" eval "() => {
    for (const element of document.querySelectorAll('span.time.relative[data-timestamp]')) {
      const fixed = element.getAttribute('data-tooltip') || element.getAttribute('datetime');
      if (fixed) element.textContent = fixed;
    }
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, {
      acceptNode(node) {
        const tag = node.parentElement?.tagName;
        return tag === 'SCRIPT' || tag === 'STYLE' || tag === 'TEXTAREA'
          ? NodeFilter.FILTER_REJECT
          : NodeFilter.FILTER_ACCEPT;
      },
    });
    const datePattern = /\b\d{4}-\d{1,2}-\d{1,2} \d{1,2}:\d{2}:\d{2}\b/g;
    const ipPattern = /\b(?:\d{1,3}\.){3}\d{1,3}\b/g;
    while (walker.nextNode()) {
      const node = walker.currentNode;
      node.nodeValue = node.nodeValue
        .replace(datePattern, '2000-1-1 00:00:00')
        .replace(ipPattern, '127.0.0.1');
    }
  }" >/dev/null
}

assert_no_horizontal_overflow() {
  local session=$1 name=$2
  if ! bash "$PWCLI" -s="$session" run-code 'async (page) => {
    const metrics = await page.evaluate(() => ({
      viewport: window.innerWidth,
      document: document.documentElement.scrollWidth,
    }));
    if (metrics.document > metrics.viewport + 1) {
      throw new Error(`${metrics.document}px document exceeds ${metrics.viewport}px viewport`);
    }
  }' >/dev/null; then
    echo "changed  $name: horizontal overflow detected at the mobile viewport" >&2
    return 1
  fi
}

capture_one() {
  local name=$1 route=$2 width=$3 height=$4 check_overflow=${5:-0}
  local current baseline diff pixels
  current="$CURRENT_DIR/$name.png"
  baseline="$BASELINE_DIR/$name.png"
  diff="$CURRENT_DIR/$name.diff.png"

  bash "$PWCLI" -s="$SESSION" resize "$width" "$height" >/dev/null
  bash "$PWCLI" -s="$SESSION" goto "$BASE_URL$route" >/dev/null
  if ! wait_for_ready "$SESSION"; then
    echo "changed  $name: page did not reach a stable rendered state" >&2
    return 1
  fi
  if [[ "$name" == "home" || "$name" == "home-auth" || "$name" == "mobile-home" ]]; then
    # Hitokoto is fetched from a third-party service and must not affect layout.
    bash "$PWCLI" -s="$SESSION" eval "() => { const node=document.querySelector('[name=hitokoto]'); if (node) node.parentElement.style.display='none'; }" >/dev/null || true
  fi
  normalize_dynamic_text "$SESSION"
  if [[ "$check_overflow" == "1" ]] && ! assert_no_horizontal_overflow "$SESSION" "$name"; then
    return 1
  fi
  bash "$PWCLI" -s="$SESSION" screenshot --full-page --filename "$current" >/dev/null

  if [[ "$UPDATE_BASELINES" == "1" ]]; then
    cp "$current" "$baseline"
    [[ ! -f "$diff" ]] || rm "$diff"
    echo "updated  $name"
    return 0
  fi
  if [[ ! -f "$baseline" ]]; then
    echo "missing  $name ($baseline); run UPDATE_BASELINES=1 once" >&2
    return 1
  fi

  if ! pixels=$(compare_images "$baseline" "$current" "$diff"); then
    echo "changed  $name: comparison could not be completed" >&2
    return 1
  fi
  if (( pixels > MAX_DIFF_PIXELS )); then
    echo "changed  $name: $pixels differing pixels (limit $MAX_DIFF_PIXELS)" >&2
    echo "         diff: $diff" >&2
    return 1
  fi
  echo "ok       $name ($pixels differing pixels)"
  [[ ! -f "$diff" ]] || rm "$diff"
}

main() {
  if [[ ! "$BASE_URL" =~ ^http://(localhost|127\.0\.0\.1)(:[0-9]+)?/?$ ]]; then
    echo "visual-regression: BASE_URL must be a local HTTP URL, got: $BASE_URL" >&2
    exit 2
  fi
  require_image_tools
  if [[ ! -f "$PWCLI" ]]; then
    echo "visual-regression: Playwright wrapper not executable: $PWCLI" >&2
    exit 2
  fi
  if [[ -n "$VISUAL_AUTH_USERNAME" || -n "$VISUAL_AUTH_PASSWORD" ]]; then
    if [[ -z "$VISUAL_AUTH_USERNAME" || -z "$VISUAL_AUTH_PASSWORD" ]]; then
      echo "visual-regression: both VISUAL_AUTH_USERNAME and VISUAL_AUTH_PASSWORD are required" >&2
      exit 2
    fi
    if [[ ! "$VISUAL_AUTH_USERNAME" =~ ^[A-Za-z0-9_.-]+$ || ! "$VISUAL_AUTH_PASSWORD" =~ ^[A-Za-z0-9_.-]+$ ]]; then
      echo "visual-regression: visual credentials may only contain letters, digits, dot, underscore and hyphen" >&2
      exit 2
    fi
  fi
  mkdir -p "$BASELINE_DIR" "$CURRENT_DIR"
  SESSION="visual-$(date +%s)-$$"

  local public_routes=(
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
  local auth_routes=(
    "home-auth:/"
    "problem-detail:/p/P1001"
    "contest-detail:/contest/6ae994800000000000003001"
    "contest-scoreboard:/contest/6ae994800000000000003001/scoreboard"
    "homework-detail:/homework/6ae994800000000000003002"
    "training-detail:/training/6ae994800000000000004001"
    "discuss-detail:/discuss/6ae994800000000000002101"
    "records:/record"
    "record-detail:/record/6ae994800000000000005001"
    "user-profile:/user/2"
    "admin:/admin"
    "admin-users:/admin/users"
    "settings-account:/home/settings?category=account"
    "security:/home/security"
  )
  local mobile_routes=(
    "mobile-home:/"
    "mobile-problems:/p"
    "mobile-problem-detail:/p/P1001"
    "mobile-contest-detail:/contest/6ae994800000000000003001"
    "mobile-records:/record"
    "mobile-admin:/admin"
  )

  cleanup() {
    bash "$PWCLI" -s="$SESSION" close >/dev/null 2>&1 || true
  }
  trap cleanup EXIT

  export TZ=${TZ:-Asia/Shanghai}
  export LANG=${LANG:-zh_CN.UTF-8}

  bash "$PWCLI" -s="$SESSION" open "$BASE_URL/login" >/dev/null
  bash "$PWCLI" -s="$SESSION" resize 1280 900 >/dev/null
  bash "$PWCLI" -s="$SESSION" set-color-scheme light >/dev/null
  bash "$PWCLI" -s="$SESSION" set-reduced-motion reduce >/dev/null

  local fail=0 entry name route
  for entry in "${public_routes[@]}"; do
    name=${entry%%:*}
    route=${entry#*:}
    if ! capture_one "$name" "$route" 1280 900; then fail=1; fi
  done

  if [[ -n "$VISUAL_AUTH_USERNAME" ]]; then
    if ! login_for_screenshots "$SESSION"; then
      echo "visual-regression: authenticated screenshot setup failed" >&2
      exit 1
    fi
    for entry in "${auth_routes[@]}"; do
      name=${entry%%:*}
      route=${entry#*:}
      if ! capture_one "$name" "$route" 1280 900; then fail=1; fi
    done
    for entry in "${mobile_routes[@]}"; do
      name=${entry%%:*}
      route=${entry#*:}
      if ! capture_one "$name" "$route" 390 844 1; then fail=1; fi
    done
  fi

  if (( fail != 0 )); then
    exit 1
  fi
  echo "visual-regression: passed"
}

if [[ ${1:-} == "--self-test" ]]; then
  run_self_test
else
  main
fi
