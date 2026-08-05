#!/usr/bin/env bash
#
# seed-registry.sh — copy every image K8sense needs into an internal registry,
# so a fully air-gapped cluster can pull them. Run this ONCE on a machine that
# can reach both the public internet and your internal registry (the classic
# "connected staging host" in an air-gap workflow).
#
# Usage:
#   ./seed-registry.sh <internal-registry> [images-file]
#
# Example:
#   ./seed-registry.sh registry.bank.internal
#   ./seed-registry.sh registry.bank.internal:5000 images.txt
#
# Requires: skopeo (preferred) OR docker. Log in to your registry first, e.g.
#   skopeo login registry.bank.internal
#   # or: docker login registry.bank.internal
#
# The repository path is preserved under the internal registry, matching how
# K8sense rewrites images (registry host is replaced, repo path + tag kept):
#   quay.io/ansible/ansible-runner:latest
#     -> <internal-registry>/ansible/ansible-runner:latest
#
set -euo pipefail

REGISTRY="${1:-}"
IMAGES_FILE="${2:-$(dirname "$0")/images.txt}"

if [[ -z "$REGISTRY" ]]; then
  echo "usage: $0 <internal-registry> [images-file]" >&2
  exit 1
fi

if [[ ! -f "$IMAGES_FILE" ]]; then
  echo "images file not found: $IMAGES_FILE" >&2
  exit 1
fi

# Strip the registry host from a reference, keeping repo path + tag. Mirrors the
# backend's splitRegistry(): first path segment is a host only if it has a '.'
# or ':' or is 'localhost'.
remainder() {
  local ref="$1" first rest
  rest="${ref#*/}"
  first="${ref%%/*}"
  if [[ "$ref" != */* ]]; then
    echo "$ref"; return
  fi
  if [[ "$first" == *.* || "$first" == *:* || "$first" == localhost ]]; then
    echo "$rest"
  else
    echo "$ref"
  fi
}

engine=""
if command -v skopeo >/dev/null 2>&1; then
  engine="skopeo"
elif command -v docker >/dev/null 2>&1; then
  engine="docker"
else
  echo "need either 'skopeo' or 'docker' on PATH" >&2
  exit 1
fi

echo "Seeding $REGISTRY using $engine (from $IMAGES_FILE)"
fail=0

while IFS= read -r line; do
  # skip comments and blanks
  line="${line%%#*}"
  img="$(echo "$line" | xargs || true)"
  [[ -z "$img" ]] && continue

  dest="$REGISTRY/$(remainder "$img")"
  echo "  $img  ->  $dest"

  if [[ "$engine" == "skopeo" ]]; then
    if ! skopeo copy --all "docker://$img" "docker://$dest"; then
      echo "    FAILED: $img" >&2; fail=1
    fi
  else
    if ! (docker pull "$img" && docker tag "$img" "$dest" && docker push "$dest"); then
      echo "    FAILED: $img" >&2; fail=1
    fi
  fi
done < "$IMAGES_FILE"

if [[ "$fail" -ne 0 ]]; then
  echo "Done with errors — some images failed to copy (see FAILED lines above)." >&2
  exit 1
fi

echo "Done. Now set the internal registry in K8sense:"
echo "  Settings → Internal registry: $REGISTRY"
echo "  (or export K8SENSE_INTERNAL_REGISTRY=$REGISTRY on the backend)"
