# K8sense — Build Progress Log

> Running record of what has been built, where it lives, and what's next.
> The product vision, architecture decisions, rule catalogue, and roadmap
> live in [`../K8SENSE_CONTEXT.md`](../K8SENSE_CONTEXT.md) — this file tracks
> *implementation* status against that plan.

**Base:** fork of [Headlamp](https://github.com/headlamp-k8s/headlamp) (Apache 2.0).
**Repo location:** `Downloads/K8Sense/k8sense/`
**Last updated:** 2026-07-21

---

## How to run it locally

Prerequisites are already installed on the dev machine (colima, kind, Go 1.26,
Node 25, kubectl). To bring the whole stack up from cold:

```bash
# 1. Container runtime + local test cluster
colima start --cpu 4 --memory 8 --disk 60
kind create cluster --name k8sense-dev          # only if it doesn't exist yet

# 2. Seed demo findings (optional but makes Cluster Doctor show results)
kubectl apply -f <scratchpad>/seed-findings.yaml # see "Test data" below

# 3. Isolated kubeconfig so the app only ever sees the local cluster
kind get kubeconfig --name k8sense-dev > k8sense/.dev/kubeconfig.yaml

# 4. Dev servers (backend :4466 + frontend :3000)
cd k8sense
KUBECONFIG=$PWD/.dev/kubeconfig.yaml npm run start
```

Open <http://localhost:3000>. **Cluster Doctor** is the top sidebar item.

> **Safety:** the dev backend is deliberately scoped to `.dev/kubeconfig.yaml`
> (local kind cluster only) so it can never touch the real amb-nkp-prod /
> wso2-test / finacle-prod clusters during development.

---

## Stage status

| Stage | Status | Notes |
|---|---|---|
| 0. Local dev environment | ✅ Done | colima + kind `k8sense-dev` + metrics-server, seeded demo namespace |
| 1a. Rebrand (identity) | ✅ Done | Names, titles, favicon/icon set, logo, theme, fonts, NOTICE |
| 1b. Cluster Doctor engine (Go) | ✅ Done | Rule loader, scanner, 21 NODE-*/POD-* checks |
| 1c. SQLite persistence | ✅ Done | WAL, migrations, scans/findings CRUD, smoke-tested |
| 1d. Cluster Doctor API (HTTP+SSE) | ✅ Done | scan / status / findings / history / rules |
| 1e. Cluster Doctor UI (React) | ✅ Done | ScanPage, FindingsPage, sidebar entry — MUI |
| 1f. RBAC manifest | ✅ Done | `deploy/k8sense-clusterrole.yaml`, dry-run validated |
| 2a. Full rule library (46 rules) | ✅ Done | Added CP/STOR/NET/RES/CERT/WL — all 8 categories |
| 2b. Scan History UI | ✅ Done | `HistoryPage`, wired to `/history` |
| 2c. Report export (HTML/PDF) | ⬜ Not started | |
| 2d. Guided Fix | ⬜ Not started | |
| 2e. Rule management UI | ⬜ Not started | |
| 3. Dashboard polish | ⬜ Not started | |
| 4. Enterprise | ⬜ Not started | |
| 5. Distribution | ⬜ Not started | |

---

## What's built (Stage 1 detail)

### Rebrand
- Package identity → `k8sense` in root/frontend/app `package.json`.
- Document title / meta / manifest → K8sense; Electron tray tooltip → K8sense.
- **Icon set** regenerated from a single navy-square "8" master SVG:
  `favicon.ico`, `favicon-16/32`, `apple-touch-icon`, `android-chrome-192/512`,
  `mstile`, `safari-pinned-tab`. In-app logo/wordmark SVGs in
  `frontend/src/resources/{icon,logo}-{dark,light}.svg`.
- **Theme:** brand palette (primary `#0F172A`, accent `#3B82F6`, severity
  red/amber/blue, healthy green) applied to the default Dark + Light themes in
  `frontend/src/components/App/defaultAppThemes.ts`. Dark is the default.
- **Fonts:** Inter + JetBrains Mono bundled locally via `@fontsource-variable/*`
  (no CDN) — imported in `index.tsx`, wired into theme + `index.css`.
- **NOTICE** prepended with K8sense's Apache 2.0 attribution for the Headlamp base.

### Cluster Doctor engine — `backend/pkg/clusterdoctor/`
- `finding.go` — `Finding` / `RawFinding` structs.
- `rule.go` — YAML rule loader (`LoadRules`), `Rule` + `GuidedFix` types.
- `registry.go` — `check_fn` name → Go function registry.
- `scanner.go` — category-grouped scan with graceful degradation (a check that
  errors or has no implementation is counted in `SkippedChecks`, never aborts
  the scan) + SSE progress events.
- `checks/nodes.go` — 12 NODE-* checks (NotReady, pressure conditions, taints,
  cordon, CPU/mem over-commit, version skew, pod capacity).
- `checks/pods.go` — 12 POD-* checks (CrashLoop, OOMKilled, Pending, Evicted,
  ImagePullBackOff, init stuck, missing limits/requests, stuck Terminating,
  root, no readiness probe, frequent restarts).
- `checks/control_plane.go` — CP-002 (component not ready), CP-005 (restarted).
- `checks/storage.go` — STOR-001/002/003/005 (PVC pending, PV released/failed,
  StorageClass no provisioner, implicit default StorageClass).
- `checks/network.go` — NET-001/002/004/005 (CoreDNS down, kube-proxy down,
  Service no endpoints, Ingress no address).
- `checks/resources.go` — RES-001/002/004/005 (quota 85%/95%, HPA can't
  compute, HPA at max).
- `checks/certificates.go` — CERT-001/002 (TLS cert expiring/expired) via real
  x509 PEM parsing of `tls.crt` only (never `tls.key`).
- `checks/workloads.go` — WL-001/003/004/005/006/009 (deployment 0-available /
  below-desired, daemonset under-scheduled, statefulset not ready, job failed,
  single-replica no-HA).
- **46 rules total across 8 categories**, metadata in `rules/*.yaml`.

### Persistence — `backend/pkg/clusterdoctor/db/`
- `db.go` — pure-Go `modernc.org/sqlite` (keeps single-binary distribution),
  WAL mode, embedded numbered migrations, OS-appropriate data dir.
- `migrations/001_initial.sql` — full schema from the context doc.
- `scans.go` — `SaveScan` / `GetFindings` / `ListScans`.
- `db_smoke_test.go` — open→migrate→save→read round-trip test (passing).

### API — `backend/pkg/clusterdoctor/api/` + `backend/cmd/clusterdoctor.go`
Routes registered on Headlamp's existing mux router:
- `POST /cluster-doctor/scan` — start scan, returns `scanId` immediately.
- `GET  /cluster-doctor/scan/:id/status` — **SSE** progress stream (EventSource).
- `GET  /cluster-doctor/findings/:scanId` — findings (doubles as JSON export).
- `GET  /cluster-doctor/history?cluster=` — scan history.
- `GET  /cluster-doctor/rules` — rule catalogue.

`livescan.go` buffers + fans out progress events so a late SSE subscriber
(frontend POSTs, *then* opens EventSource) still replays everything. Cluster
resolution reuses Headlamp's kubeconfig store + token flow, so no separate auth.

### UI — React (MUI)
- `frontend/src/lib/cluster-doctor-api.ts` — typed API client.
- `frontend/src/lib/sse-client.ts` — EventSource wrapper.
- `frontend/src/components/cluster-doctor/` — `SeverityBadge`, `FindingsTable`
  (expandable rows w/ remediation), `FindingsFilter`, `ScanProgress`.
- `frontend/src/pages/cluster-doctor/` — `ScanPage`, `FindingsPage`.
- Routes `clusterDoctorScan` / `clusterDoctorFindings` + sidebar entry
  (stethoscope icon).

### RBAC — `deploy/k8sense-clusterrole.yaml`
Read-only `k8sense-scanner` ClusterRole (all tiers) + `k8sense-guided-fix`
(Pro write verbs). Validated with `kubectl apply --dry-run=client`.

---

## Verified end-to-end

Driven through a real headless browser against the kind cluster:
scan → SSE progress (per-category chips) → findings persisted and rendered,
severity-sorted, filterable → scan history. All 8 categories fire correctly
against seeded data (CERT-002 on the expired TLS secret, STOR-001 on the
pending PVC, NET-004 on the orphan Service, WL-001 on zero-available
deployments) plus genuine `kube-system` findings. Latest full scan: **67
findings** (7 CRITICAL / 30 WARNING / 30 INFO). SQLite history survives colima
VM restarts (scans from before a VM reboot still listed).

---

## Decisions log (implementation-level)

These are choices made during the build that deviate from or refine
`K8SENSE_CONTEXT.md`. Each is revisitable.

1. **UI toolkit: MUI, not Tailwind/shadcn (revisitable).**
   The context doc specified shadcn/ui + Tailwind. In practice the Headlamp
   fork is ~200 screens of MUI that can't be removed without rewriting the
   whole app, so adopting Tailwind/shadcn would mean maintaining *two* design
   systems permanently, not replacing one. Cluster Doctor's distinct look is
   achieved via brand theme tokens instead. Kept MUI to ship a working Phase 1
   slice; migrating later is possible but costs a rewrite of these components.

2. **SQLite driver: `modernc.org/sqlite` (pure Go), not `mattn/go-sqlite3`.**
   The context doc named `mattn/go-sqlite3`, which needs cgo. Pure-Go keeps
   K8sense a single static cross-compilable binary — important for the
   PyInstaller-style "copy one file" distribution goal. No functional
   difference for our usage.

3. **Rebrand scope: user-facing surface only.**
   Internal Go/TS identifiers still say "headlamp" (package paths, variable
   names). Renaming them is high-risk churn with zero user benefit; the context
   doc's "zero mention of Headlamp in the product UI" bar is met. Deep rename
   deferred to the year-2 incremental-rewrite plan.

4. **CERT-* reads `tls.crt`, not "annotations only" (Stage 2).**
   `K8SENSE_CONTEXT.md` says K8sense checks TLS expiry "via annotations,
   never reads `.data`". In practice vanilla `kubernetes.io/tls` Secrets carry
   no expiry annotation — you must parse the certificate. `checks/certificates.go`
   reads only `tls.crt` (the **public** certificate, not a secret) and never
   `tls.key`. K8s RBAC can't grant get-metadata-but-not-data on Secrets anyway,
   so the shipped ClusterRole already allows this. Low risk; documented for
   transparency.

5. **CP-* checks target self-hosted control planes (Stage 2).**
   CP checks inspect static control-plane pods in `kube-system` by the
   `component` label / name prefix. Managed control planes (EKS/GKE/AKS) don't
   expose these as pods, so the checks correctly find nothing there rather than
   false-alarming. Deeper managed-control-plane health (via `/healthz`) is a
   later addition.

---

## Next up (remaining Stage 2, in suggested order)

1. **Report export** — HTML first (self-contained, embedded fonts), then PDF
   via bundled Chromium. Backend `/findings/:scanId/export?format=` stub.
2. **Guided Fix** — `GuidedFixModal` + `POST /cluster-doctor/guided-fix` +
   audit-log write. Safe actions only (delete evicted pod, uncordon, scale,
   rollout restart). Rules already carry `guidedFix.action`.
3. **Rule management UI** — list/toggle rules, import custom YAML (`/rules`
   list endpoint already exists; needs toggle + import endpoints).
4. **Finding suppression + comments** — mute a finding with a reason.
5. **Scan diff** — compare two scans (new / resolved / persisted).
