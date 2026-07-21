#!/usr/bin/env bash
# Seeds a kind cluster with workloads that deliberately trip Cluster Doctor
# rules, so the dev environment always has a realistic spread of findings.
#
# Usage: ./seed-demo.sh <kube-context> [--full]
#   --full  also seeds the heavier fixtures (expired TLS, unbound PVC)
#
# Safe to re-run: every object is applied idempotently.
set -euo pipefail

CTX="${1:?usage: seed-demo.sh <kube-context> [--full]}"
FULL="${2:-}"
# Kubeconfig lives in the gitignored .dev/ dir at the repo root.
KC="${KUBECONFIG_PATH:-$(cd "$(dirname "$0")/../../.dev" && pwd)/kubeconfig.yaml}"
k() { kubectl --kubeconfig "$KC" --context "$CTX" "$@"; }

echo "==> seeding $CTX"
k create namespace demo --dry-run=client -o yaml | k apply -f - >/dev/null

# CrashLoopBackOff + frequent restarts (POD-001, POD-013)
k -n demo apply -f - >/dev/null <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata: { name: crashloop-app }
spec:
  replicas: 1
  selector: { matchLabels: { app: crashloop-app } }
  template:
    metadata: { labels: { app: crashloop-app } }
    spec:
      containers:
        - name: boom
          image: busybox:1.36
          command: ["sh", "-c", "echo starting; sleep 3; exit 1"]
YAML

# ImagePullBackOff + zero available replicas (POD-005, WL-001)
k -n demo apply -f - >/dev/null <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata: { name: zero-replica-app }
spec:
  replicas: 2
  selector: { matchLabels: { app: zero-replica-app } }
  template:
    metadata: { labels: { app: zero-replica-app } }
    spec:
      containers:
        - name: nope
          image: nginx:this-tag-does-not-exist
YAML

# Missing resource limits/requests (POD-008, POD-009) + no probes
k -n demo apply -f - >/dev/null <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata: { name: no-limits-app }
spec:
  replicas: 1
  selector: { matchLabels: { app: no-limits-app } }
  template:
    metadata: { labels: { app: no-limits-app } }
    spec:
      containers:
        - name: web
          image: nginx:stable
YAML

# Service with no endpoints (NET-004)
k -n demo apply -f - >/dev/null <<'YAML'
apiVersion: v1
kind: Service
metadata: { name: orphan-service }
spec:
  selector: { app: nothing-matches-this }
  ports: [{ port: 80, targetPort: 80 }]
YAML

if [ "$FULL" = "--full" ]; then
  # Unbound PVC (STOR-001)
  k -n demo apply -f - >/dev/null <<'YAML'
apiVersion: v1
kind: PersistentVolumeClaim
metadata: { name: pending-pvc }
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: does-not-exist
  resources: { requests: { storage: 1Gi } }
YAML

  # Already-expired TLS secret (CERT-002)
  TMP=$(mktemp -d)
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$TMP/tls.key" -out "$TMP/tls.crt" \
    -days 1 -subj "/CN=expired.k8sense.local" >/dev/null 2>&1
  # Backdate by generating with -days 1 then relying on CERT-001/002 windows;
  # for a truly expired cert regenerate with faketime if available.
  k -n demo create secret tls expired-tls \
    --cert="$TMP/tls.crt" --key="$TMP/tls.key" \
    --dry-run=client -o yaml | k apply -f - >/dev/null
  rm -rf "$TMP"
fi

echo "==> done: $(k -n demo get deploy --no-headers 2>/dev/null | wc -l | tr -d ' ') deployments in demo"
