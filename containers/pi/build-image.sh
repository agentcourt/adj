#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
image="${PI_CONTAINER_IMAGE:-agentcourt-pi-sandbox}"

podman build \
  -t "$image" \
  -f "$script_dir/Dockerfile" \
  "$script_dir"
