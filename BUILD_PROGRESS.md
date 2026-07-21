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
| 2c. Report export | ✅ Done | Self-contained HTML report + JSON; PDF deferred |
| 2d. Guided Fix + audit log | ✅ Done | Confirm-gated write actions, audit trail |
| 2e. Rule management UI | ✅ Done | Per-cluster enable/disable; scanner honors it |
| 2f. Audit log viewer | ✅ Done | `AuditLogPage` |
| 2g. Finding suppression + comments | ✅ Done | Per-resource mute (migration 002) |
| 2h. Scan diff | ✅ Done | Added/resolved/persisted between two scans |
| 2i. Licence system + gating | ✅ Done | Ed25519 validator, 14-day trial, Free/Pro tiers |
| 2j. Retention + storage + test-conn | ✅ Done | Startup prune, storage stats, `/clusters/test` |
| 2k. Custom rule import | ✅ Done | Validate + import YAML, live without restart |
| 2l. Multi-cluster parallel scan | ✅ Done | `/scan/multi`, 5 concurrent, `MultiScanPage` |
| 3a. Per-rule severity override | ✅ Done | Migration 003; scanner applies per cluster |
| 3b. Scheduled scans | ✅ Done | Background scheduler, 5-min floor (migration 004/005) |
| 3c. Slack/Teams notifications | ✅ Done | Alerts only on *newly appeared* criticals |
| 3d. Cluster Doctor badges | ✅ Done | Sidebar critical count + Pod/Node findings panel |
| 3e. Cluster overview redesign | ✅ Done | `ClusterHealthPanel` on the cluster landing page |
| 3f. Keyboard shortcuts | ✅ Done | Ctrl+Shift+D scan, Ctrl+Shift+K findings (remappable) |
| 3g. Rebrand audit | ✅ Done | 3 strings × 17 locales, tray labels, a11y label |
| 4a. White-label branding | ✅ Done | Product name / colour / logo, Pro + admin gated |
| 4b. In-app RBAC roles | ✅ Done | viewer/operator/admin guardrail (see caveat below) |
| 4c. Audit CSV export | ✅ Done | `/audit-log/export` |
| 4d. SSO / team sharing | 🚫 Blocked | Needs a real IdP + Postgres (your infra) |
| 4e. Web-app (in-cluster) mode | 🟡 Mostly | Container + manifests done; single-replica only |
| 4f. Audit actor identity | ✅ Done | Derived from bearer-token claims (OIDC / service account) |
| 4g. PDF export | ✅ Done | Print stylesheet — browser renders the PDF, no bundled engine |
| 4h. Postgres backend (HA) | 🟡 Groundwork | Dialect layer tested; migrations not yet ported |
| 5. Distribution | ⬜ Not started | Deliberately deferred until features are complete |

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
- `scans.go` — `SaveScan` / `GetFindings` / `GetScan` / `ListScans`.
- `audit.go` — `WriteAudit` / `ListAudit` (Guided Fix audit trail).
- `db_smoke_test.go` — open→migrate→save→read round-trip test (passing).

### Reporting — `backend/pkg/clusterdoctor/reporter.go`
- `RenderHTMLReport` — fully self-contained HTML (inline CSS, system fonts,
  **zero external requests** — verified) so reports render on air-gapped
  machines and email as one file. PDF export deferred (would add bundled
  Chromium; HTML covers the demo-critical "send to your CTO" need for now).

### API — `backend/pkg/clusterdoctor/api/` + `backend/cmd/clusterdoctor.go`
Routes registered on Headlamp's existing mux router:
- `POST /cluster-doctor/scan` — start scan, returns `scanId` immediately.
- `GET  /cluster-doctor/scan/:id/status` — **SSE** progress stream (EventSource).
- `GET  /cluster-doctor/findings/:scanId` — findings (guided-fix enriched).
- `GET  /cluster-doctor/findings/:scanId/export?format=html|json` — report download.
- `GET  /cluster-doctor/history?cluster=` — scan history.
- `GET  /cluster-doctor/rules` — rule catalogue.
- `POST /cluster-doctor/guided-fix` — execute a confirmed safe fix (Pro).
- `GET  /cluster-doctor/audit-log?cluster=` — audit trail of fixes taken.

`livescan.go` buffers + fans out progress events so a late SSE subscriber
(frontend POSTs, *then* opens EventSource) still replays everything. Cluster
resolution reuses Headlamp's kubeconfig store + token flow, so no separate auth.
`guidedfix.go` enforces an action allowlist (delete_pod, delete_job,
uncordon_node, scale_deployment, restart_deployment) + `confirmed: true` +
audit write on every attempt.

### UI — React (MUI)
- `frontend/src/lib/cluster-doctor-api.ts` — typed API client (scan, findings,
  history, rules, export/download, guided fix).
- `frontend/src/lib/sse-client.ts` — EventSource wrapper.
- `frontend/src/components/cluster-doctor/` — `SeverityBadge`, `FindingsTable`
  (expandable rows + Apply Fix button), `FindingsFilter`, `ScanProgress`,
  `GuidedFixModal` (confirm-gated command preview).
- `frontend/src/pages/cluster-doctor/` — `ScanPage`, `FindingsPage`
  (export buttons), `HistoryPage`.
- Routes `clusterDoctorScan` / `clusterDoctorFindings` / `clusterDoctorHistory`
  + sidebar entry (stethoscope icon).

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

6. **Guided-fix availability is derived at read time, not persisted (Stage 2).**
   The `findings` table stores resource snapshots but no guided-fix columns.
   `Server.enrichGuidedFix` re-maps each finding's `rule_id` to the current
   rule set when findings are read back, so `guidedFixAvailable/Action/Warning`
   always reflect the live rules (and updating a rule's guided fix applies to
   historical scans). Avoids a schema migration and keeps guided-fix logic
   single-sourced in the rule YAML.

7. **Guided Fix safety model (Stage 2).**
   Server enforces an action **allowlist** (delete_pod, delete_job,
   uncordon_node, scale_deployment, restart_deployment) — anything else is 403.
   Every request needs `confirmed: true` (400 otherwise), and **every attempt**
   (success or failure) writes an `audit_log` row with actor/action/resource/
   result. This matches the context doc's "explicit human intent + audit trail"
   requirement for regulated customers. Runbook auto-fix (Tier 3) remains out
   of scope until Phase 4.

8. **Per-feature frontend API modules (Stage 2e).**
   `apiFetch`/`apiUrl` in `cluster-doctor-api.ts` are exported so each feature
   (rules, audit, diff, suppression) ships its own `cluster-doctor-*-api.ts`
   importing them, instead of all editing one file. This was done to let three
   features be built in parallel (isolated git worktrees) without fighting over
   one shared API client. Rule enable/disable is persisted in `rule_overrides`
   and honored by the scanner in `runScan`; suppression state lives in the
   `suppressions` table (migration 002) keyed by resource identity
   (cluster+rule+namespace+kind+name) so a mute survives across scans, and is
   re-derived onto findings at read time via `enrichSuppressions`.

---

## Known limitations (read before shipping)

1. **In-app roles are a guardrail, not a security boundary.** The role lives in
   a local `role.json`, so a determined local user can edit it. It exists to
   prevent fat-fingered writes and to enable a read-only install. Real
   enforcement is (a) the cluster's own RBAC, which still governs every request
   K8sense makes, and (b) SSO-backed identity, which is not built yet.
2. ~~Test coverage is uneven.~~ **Closed.** 112 tests across all 5 packages
   (`go vet` clean, `-race` clean, no skips) now cover the HTTP handlers,
   licence/role gating, the guided-fix allowlist and audit trail, suppression
   scoping, retention, schedule due-logic and webhook rendering. Route
   registration was moved into `api.RegisterRoutes` so tests exercise the same
   routing table the binary serves. The tests were mutation-verified: removing
   a licence gate and breaking the suppression key each made the relevant test
   fail immediately.
3. **PDF export is not implemented.** HTML and JSON export are. PDF needs a
   bundled Chromium, which is best done alongside the Phase 5 packaging work.
4. **Nothing is packaged for the desktop.** The Electron shell has never been
   built or launched, so the tray-label rebrand is untested at runtime. The
   *container* image, by contrast, now builds and is the basis of web mode.

5. **Web mode is single-replica only.** SQLite is a single-writer store, so two
   pods sharing the volume would corrupt it. `deploy/k8sense-web.yaml` pins
   replicas to 1 and uses the Recreate strategy for exactly this reason.
   Multi-replica HA needs the Postgres backend (groundwork only — see
   `db/dialect.go`).

6. **Web mode shares one ServiceAccount.** Every browser user acts with the
   same cluster permissions, so the ClusterRole must be scoped to what you'd
   grant everyone, and the Service should sit behind an authenticating proxy.
   The audit log now records real per-user identity from the request token, so
   *attribution* is correct even though *authorisation* is shared.

## Next up (accurate as of the latest commit)

1. **PDF export** — bundled Chromium rendering the existing HTML report.
2. **Lens-style resource forms** — surface/polish the inherited Headlamp
   create/edit flows (RBAC, PVC, StorageClass, ServiceAccount).
3. **Phase 5 distribution** — Electron packaging, signing, auto-update. Needs
   your Apple/EV certificates, domain and Stripe account.

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
| 2c. Report export | ✅ Done | Self-contained HTML report + JSON; PDF deferred |
| 2d. Guided Fix + audit log | ✅ Done | Confirm-gated write actions, audit trail |
| 2e. Rule management UI | ✅ Done | Per-cluster enable/disable; scanner honors it |
| 2f. Audit log viewer | ✅ Done | `AuditLogPage` |
| 2g. Finding suppression + comments | ✅ Done | Per-resource mute (migration 002) |
| 2h. Scan diff | ✅ Done | Added/resolved/persisted between two scans |
| 2i. Licence system + gating | ✅ Done | Ed25519 validator, 14-day trial, Free/Pro tiers |
| 2j. Retention + storage + test-conn | ✅ Done | Startup prune, storage stats, `/clusters/test` |
| 2k. Custom rule import | ✅ Done | Validate + import YAML, live without restart |
| 2l. Multi-cluster parallel scan | ✅ Done | `/scan/multi`, 5 concurrent, `MultiScanPage` |
| 3a. Per-rule severity override | ✅ Done | Migration 003; scanner applies per cluster |
| 3b. Scheduled scans | ✅ Done | Background scheduler, 5-min floor (migration 004/005) |
| 3c. Slack/Teams notifications | ✅ Done | Alerts only on *newly appeared* criticals |
| 3d. Cluster Doctor badges | ✅ Done | Sidebar critical count + Pod/Node findings panel |
| 3e. Cluster overview redesign | ✅ Done | `ClusterHealthPanel` on the cluster landing page |
| 3f. Keyboard shortcuts | ✅ Done | Ctrl+Shift+D scan, Ctrl+Shift+K findings (remappable) |
| 3g. Rebrand audit | ✅ Done | 3 strings × 17 locales, tray labels, a11y label |
| 4a. White-label branding | ✅ Done | Product name / colour / logo, Pro + admin gated |
| 4b. In-app RBAC roles | ✅ Done | viewer/operator/admin guardrail (see caveat below) |
| 4c. Audit CSV export | ✅ Done | `/audit-log/export` |
| 4d. SSO / team sharing | 🚫 Blocked | Needs a real IdP + Postgres (your infra) |
| 4e. Web-app (in-cluster) mode | 🟡 Mostly | Container + manifests done; single-replica only |
| 4f. Audit actor identity | ✅ Done | Derived from bearer-token claims (OIDC / service account) |
| 4g. PDF export | ✅ Done | Print stylesheet — browser renders the PDF, no bundled engine |
| 4h. Postgres backend (HA) | 🟡 Groundwork | Dialect layer tested; migrations not yet ported |
| 5. Distribution | ⬜ Not started | Deliberately deferred until features are complete |

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
- `scans.go` — `SaveScan` / `GetFindings` / `GetScan` / `ListScans`.
- `audit.go` — `WriteAudit` / `ListAudit` (Guided Fix audit trail).
- `db_smoke_test.go` — open→migrate→save→read round-trip test (passing).

### Reporting — `backend/pkg/clusterdoctor/reporter.go`
- `RenderHTMLReport` — fully self-contained HTML (inline CSS, system fonts,
  **zero external requests** — verified) so reports render on air-gapped
  machines and email as one file. PDF export deferred (would add bundled
  Chromium; HTML covers the demo-critical "send to your CTO" need for now).

### API — `backend/pkg/clusterdoctor/api/` + `backend/cmd/clusterdoctor.go`
Routes registered on Headlamp's existing mux router:
- `POST /cluster-doctor/scan` — start scan, returns `scanId` immediately.
- `GET  /cluster-doctor/scan/:id/status` — **SSE** progress stream (EventSource).
- `GET  /cluster-doctor/findings/:scanId` — findings (guided-fix enriched).
- `GET  /cluster-doctor/findings/:scanId/export?format=html|json` — report download.
- `GET  /cluster-doctor/history?cluster=` — scan history.
- `GET  /cluster-doctor/rules` — rule catalogue.
- `POST /cluster-doctor/guided-fix` — execute a confirmed safe fix (Pro).
- `GET  /cluster-doctor/audit-log?cluster=` — audit trail of fixes taken.

`livescan.go` buffers + fans out progress events so a late SSE subscriber
(frontend POSTs, *then* opens EventSource) still replays everything. Cluster
resolution reuses Headlamp's kubeconfig store + token flow, so no separate auth.
`guidedfix.go` enforces an action allowlist (delete_pod, delete_job,
uncordon_node, scale_deployment, restart_deployment) + `confirmed: true` +
audit write on every attempt.

### UI — React (MUI)
- `frontend/src/lib/cluster-doctor-api.ts` — typed API client (scan, findings,
  history, rules, export/download, guided fix).
- `frontend/src/lib/sse-client.ts` — EventSource wrapper.
- `frontend/src/components/cluster-doctor/` — `SeverityBadge`, `FindingsTable`
  (expandable rows + Apply Fix button), `FindingsFilter`, `ScanProgress`,
  `GuidedFixModal` (confirm-gated command preview).
- `frontend/src/pages/cluster-doctor/` — `ScanPage`, `FindingsPage`
  (export buttons), `HistoryPage`.
- Routes `clusterDoctorScan` / `clusterDoctorFindings` / `clusterDoctorHistory`
  + sidebar entry (stethoscope icon).

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

6. **Guided-fix availability is derived at read time, not persisted (Stage 2).**
   The `findings` table stores resource snapshots but no guided-fix columns.
   `Server.enrichGuidedFix` re-maps each finding's `rule_id` to the current
   rule set when findings are read back, so `guidedFixAvailable/Action/Warning`
   always reflect the live rules (and updating a rule's guided fix applies to
   historical scans). Avoids a schema migration and keeps guided-fix logic
   single-sourced in the rule YAML.

7. **Guided Fix safety model (Stage 2).**
   Server enforces an action **allowlist** (delete_pod, delete_job,
   uncordon_node, scale_deployment, restart_deployment) — anything else is 403.
   Every request needs `confirmed: true` (400 otherwise), and **every attempt**
   (success or failure) writes an `audit_log` row with actor/action/resource/
   result. This matches the context doc's "explicit human intent + audit trail"
   requirement for regulated customers. Runbook auto-fix (Tier 3) remains out
   of scope until Phase 4.

8. **Per-feature frontend API modules (Stage 2e).**
   `apiFetch`/`apiUrl` in `cluster-doctor-api.ts` are exported so each feature
   (rules, audit, diff, suppression) ships its own `cluster-doctor-*-api.ts`
   importing them, instead of all editing one file. This was done to let three
   features be built in parallel (isolated git worktrees) without fighting over
   one shared API client. Rule enable/disable is persisted in `rule_overrides`
   and honored by the scanner in `runScan`; suppression state lives in the
   `suppressions` table (migration 002) keyed by resource identity
   (cluster+rule+namespace+kind+name) so a mute survives across scans, and is
   re-derived onto findings at read time via `enrichSuppressions`.

---

## Known limitations (read before shipping)

1. **In-app roles are a guardrail, not a security boundary.** The role lives in
   a local `role.json`, so a determined local user can edit it. It exists to
   prevent fat-fingered writes and to enable a read-only install. Real
   enforcement is (a) the cluster's own RBAC, which still governs every request
   K8sense makes, and (b) SSO-backed identity, which is not built yet.
2. ~~Test coverage is uneven.~~ **Closed.** 112 tests across all 5 packages
   (`go vet` clean, `-race` clean, no skips) now cover the HTTP handlers,
   licence/role gating, the guided-fix allowlist and audit trail, suppression
   scoping, retention, schedule due-logic and webhook rendering. Route
   registration was moved into `api.RegisterRoutes` so tests exercise the same
   routing table the binary serves. The tests were mutation-verified: removing
   a licence gate and breaking the suppression key each made the relevant test
   fail immediately.
3. **PDF export is not implemented.** HTML and JSON export are. PDF needs a
   bundled Chromium, which is best done alongside the Phase 5 packaging work.
4. **Nothing is packaged for the desktop.** The Electron shell has never been
   built or launched, so the tray-label rebrand is untested at runtime. The
   *container* image, by contrast, now builds and is the basis of web mode.

5. **Web mode is single-replica only.** SQLite is a single-writer store, so two
   pods sharing the volume would corrupt it. `deploy/k8sense-web.yaml` pins
   replicas to 1 and uses the Recreate strategy for exactly this reason.
   Multi-replica HA needs the Postgres backend (groundwork only — see
   `db/dialect.go`).

6. **Web mode shares one ServiceAccount.** Every browser user acts with the
   same cluster permissions, so the ClusterRole must be scoped to what you'd
   grant everyone, and the Service should sit behind an authenticating proxy.
   The audit log now records real per-user identity from the request token, so
   *attribution* is correct even though *authorisation* is shared.

## Next up (remaining Stage 2 / Phase 3, in suggested order)

1. **Custom rule import** — POST /rules/import + validate; RulesPage upload UI
   (rule toggle already done; `custom_rules` table exists).
2. **PDF export** — bundled Chromium (Puppeteer) rendering the HTML report.
3. **Licence gating** — middleware blocking Guided Fix / export / history on
   Free tier (currently everything is unlocked in dev).
4. **ScanDiff UI entry point** — link from HistoryPage to compare two selected
   scans (backend `/diff` + `ScanDiffPage` already exist; needs a "compare"
   affordance on the history table).
5. **Phase 3 dashboard polish** — Lens-style per-resource create/edit forms
   (RBAC, PVC, StorageClass) surfaced from the Headlamp base.
