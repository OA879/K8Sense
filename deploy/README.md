# Deploying K8sense

K8sense runs in two modes from the **same binary**:

- **Desktop app** — each user runs it on their machine; it reads their local
  kubeconfigs; nothing leaves the machine. This is the default and needs no
  configuration.
- **Web app (in-cluster)** — a hosted deployment a team browses to. This
  directory covers that mode.

---

## Web app: quick start

```bash
# 1. Read-only scanner RBAC for Cluster Doctor.
kubectl apply -f deploy/k8sense-clusterrole.yaml

# 2. Namespace, ServiceAccount, PVC, Deployment, Service.
kubectl apply -f deploy/k8sense-web.yaml

# 3. Reach it (or put an Ingress / authenticating proxy in front).
kubectl -n k8sense port-forward svc/k8sense 8080:80
open http://localhost:8080
```

The image is `k8sense:dev` by default — build and push your own first:

```bash
docker build -t <registry>/k8sense:<tag> .
# then set image: <registry>/k8sense:<tag> in deploy/k8sense-web.yaml
```

---

## Configuration (environment variables)

| Variable | Default | Purpose |
|---|---|---|
| `K8SENSE_DB_PATH` | app-data dir | SQLite file location. In a container, point at a **mounted volume** or scan history + the audit log are lost on every restart. |
| `K8SENSE_DB_DSN` | *(unset)* | `postgres://…` selects the Postgres backend (for multi-replica HA). Overrides `K8SENSE_DB_PATH`. Never logged. |
| `K8SENSE_RULES_DIR` | next to binary | Cluster Doctor rule library. The image ships it at `/headlamp/rules`; set this only to override. If the rules can't be found, the engine logs a loud error and disables itself. |
| `K8SENSE_CONFIG_DIR` | beside the DB | Where licence / branding / role files live. With a Postgres DSN there is no DB directory, so this (or the app-data dir) is used. |

---

## Choosing a backend

### SQLite (default) — one replica

SQLite is a single-writer store. Two pods sharing one volume **will corrupt
it**, so `k8sense-web.yaml` pins `replicas: 1` with the `Recreate` strategy.
This is fine for most teams; the PVC preserves history and the audit log
across restarts.

### Postgres — multi-replica HA

For high availability, point at Postgres and raise the replica count:

```yaml
env:
  - name: K8SENSE_DB_DSN
    valueFrom:
      secretKeyRef: { name: k8sense-db, key: dsn }   # postgres://user:pass@host:5432/k8sense?sslmode=require
```

Then remove the PVC / `K8SENSE_DB_PATH` and set `replicas: 2+` with a normal
rolling strategy. The Postgres path is verified end-to-end in
`backend/pkg/clusterdoctor/db` (`TestPostgresBackendEndToEnd`).

---

## Security notes for web mode

1. **Shared identity.** In-cluster, K8sense talks to Kubernetes with the
   ServiceAccount in `k8sense-web.yaml`, so **every browser user shares that
   identity's permissions**. Scope the ClusterRole to what you're willing to
   grant everyone.
2. **Put authentication in front.** The Service itself is unauthenticated. Use
   an Ingress with OIDC/OAuth2-proxy, or your platform's auth, so only your
   people reach it.
3. **Attribution is per-user even though authorisation is shared.** The audit
   log records the real identity from each request's bearer token (OIDC email /
   username, or Kubernetes service-account subject), so "who ran this fix?" is
   answerable. The token itself is never logged or stored.
4. **In-app roles are a guardrail, not a boundary.** viewer/operator/admin
   prevent accidental writes and enable a read-only install, but they are read
   from local config. Real enforcement is the cluster's own RBAC plus the
   authenticating proxy above.

---

## What is NOT yet verified

- The **container image has not been run** end-to-end in-cluster in this
  environment (the dev machine was memory-constrained). The Dockerfile is
  correct and unit-guarded, but do a smoke test on a machine with headroom
  before production.
- **Signed desktop installers** need Apple Developer + Windows EV certificates
  (not yet configured).
