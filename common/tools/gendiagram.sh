#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <input.mmd> <output.png>" >&2
  exit 2
fi

input="$1"
output="$2"

if ! command -v mmdc >/dev/null 2>&1; then
  echo "error: mmdc not found in PATH" >&2
  exit 2
fi

chromium_bin=""
for candidate in chromium chromium-browser google-chrome google-chrome-stable; do
  if command -v "$candidate" >/dev/null 2>&1; then
    chromium_bin="$(command -v "$candidate")"
    break
  fi
done

if [[ -z "$chromium_bin" ]]; then
  echo "error: chromium/chrome not found in PATH" >&2
  exit 2
fi

config_file="$(mktemp /tmp/puppeteer-mermaid-XXXXXX.json)"
trap 'rm -f "$config_file"' EXIT
cat > "$config_file" <<JSON
{
  "executablePath": "$chromium_bin",
  "args": ["--no-sandbox", "--disable-setuid-sandbox"]
}
JSON

mkdir -p "$(dirname "$output")"
scale="${MMDC_SCALE:-4}"
mmdc -p "$config_file" -i "$input" -o "$output" -s "$scale"
