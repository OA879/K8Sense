#!/usr/bin/env bash
#
# Build the K8sense container image and push it to a registry (Docker Hub by
# default), tagged so every push is uniquely versioned and traceable to code.
#
# Each push produces three tags on the same image:
#   <repo>:<git-short-sha>     immutable, pins the exact commit
#   <repo>:<UTC build stamp>   unique per push (YYYYMMDD-HHMMSS), even for the
#                              same commit — this is the "versioned per push" tag
#   <repo>:latest              always the most recent push
#
# Usage:
#   docker login                       # once; uses YOUR Docker Hub credentials
#   ./scripts/docker-publish.sh        # pushes to $DOCKER_REPO (default below)
#   DOCKER_REPO=you/k8sense ./scripts/docker-publish.sh
#   PLATFORMS=linux/amd64,linux/arm64 ./scripts/docker-publish.sh   # multi-arch
#
# Env:
#   DOCKER_REPO   registry repo, e.g. "oa879/k8sense" (default below)
#   PLATFORMS     buildx platforms (default linux/amd64 — the common cluster arch)
#   PUSH          "false" to build+tag locally without pushing (default "true")
set -euo pipefail

REPO="${DOCKER_REPO:-oa879/k8sense}"
PLATFORMS="${PLATFORMS:-linux/amd64}"
PUSH="${PUSH:-true}"

cd "$(dirname "$0")/.."

# Point docker at the colima VM if that's the active runtime and DOCKER_HOST
# isn't already set.
if [[ -z "${DOCKER_HOST:-}" && -S "$HOME/.colima/default/docker.sock" ]]; then
  export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
fi

# --- version tags ---
SHA="$(git rev-parse --short HEAD)"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
DIRTY=""
git diff --quiet && git diff --cached --quiet || DIRTY="-dirty"
SHA_TAG="${SHA}${DIRTY}"

echo "==> Publishing K8sense image"
echo "    repo:      ${REPO}"
echo "    platforms: ${PLATFORMS}"
echo "    tags:      ${SHA_TAG}, ${STAMP}, latest"
[[ -n "$DIRTY" ]] && echo "    WARNING: working tree has uncommitted changes (tag marked -dirty)"

# --- preflight: registry auth (only needed when pushing) ---
if [[ "$PUSH" == "true" ]]; then
  registry="${REPO%%/*}"                     # e.g. "you" -> assume Docker Hub
  if ! docker system info 2>/dev/null | grep -qi 'Username:'; then
    if [[ ! -s "$HOME/.docker/config.json" ]] || ! grep -q '"auths"[^}]*[^{}]' "$HOME/.docker/config.json" 2>/dev/null; then
      echo "ERROR: not logged in to a registry. Run 'docker login' first." >&2
      exit 1
    fi
  fi
  echo "    (pushing to registry)"
fi

# buildx builder (create once, reuse)
docker buildx inspect k8sense-builder >/dev/null 2>&1 || \
  docker buildx create --name k8sense-builder --use >/dev/null
docker buildx use k8sense-builder

BUILD_ARGS=(
  --platform "$PLATFORMS"
  -t "${REPO}:${SHA_TAG}"
  -t "${REPO}:${STAMP}"
  -t "${REPO}:latest"
  -f Dockerfile
  .
)

if [[ "$PUSH" == "true" ]]; then
  BUILD_ARGS+=(--push)
else
  # --load only supports a single platform.
  BUILD_ARGS+=(--load)
fi

docker buildx build "${BUILD_ARGS[@]}"

echo "==> Done."
echo "    ${REPO}:${SHA_TAG}"
echo "    ${REPO}:${STAMP}"
echo "    ${REPO}:latest"
[[ "$PUSH" == "true" ]] && echo "    Pushed to ${REPO}."
