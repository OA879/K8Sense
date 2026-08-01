<h1>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/src/resources/logo-light.svg">
    <img src="frontend/src/resources/logo-dark.svg" alt="K8sense" height="60">
  </picture>
</h1>

**K8sense** is a Kubernetes operations platform — a fast, modern cluster UI with
a built-in diagnostic engine that finds problems, fixes the safe ones with one
click, and shows you what can talk to what.

## Features

### Cluster Doctor — diagnostics

- **65 built-in checks** across 8 categories (nodes, pods, control plane,
  storage, network, resources, certificates, workloads), driven by a YAML rule
  library you can extend or override per cluster.
- **Live scans** with streaming progress, severity-sorted findings, filtering,
  suppression, scan history, and scan-to-scan diffs.
- **Reports** to hand to your team: self-contained HTML, PDF, and JSON.

### Guided Fix — one-click remediation

- Safe, pre-approved actions (restart/scale a workload, uncordon a node, clear a
  stuck or failed pod/job) behind an **explicit consequences prompt** — across
  all severities.
- **Undo** for reversible fixes (scale, uncordon): the prior state is captured
  and can be restored from the audit log.
- Every action requires confirmation and is recorded.

### Network Map

- Visualises **what can reach what**, derived purely from NetworkPolicies:
  workloads coloured by ingress exposure (open / restricted / isolated) with the
  policy-allowed connections between them.
- Optional **live-traffic overlay** — when the cluster runs a service mesh
  (Istio) with a reachable Prometheus, observed request rates are drawn on top.

### Platform

- **Multi-cluster** parallel scans, **scheduled** background scans, and
  Slack/Teams notifications on newly-appeared criticals.
- **Full audit log** of every action on the platform, exportable to CSV.
- **Licence tiers** (Free / Pro, 14-day trial), **white-label** branding, and
  in-app **RBAC roles**.
- Runs as a **desktop app** (macOS / Linux / Windows) or an **in-cluster** web
  deployment. Single static Go binary; **SQLite** by default, or **Postgres**
  for high availability.
- Extensible through the Headlamp plugin system.

## Running it

### Development

Requires Go, Node, and a kubeconfig pointing at a cluster.

```bash
npm run start   # backend on :4466, frontend on :3000
```

Open <http://localhost:3000>, select your cluster, and open **Cluster Doctor**
from the sidebar.

### Packaging

- **Desktop installers** (unsigned): push a version tag to build macOS `.dmg`,
  Linux `.AppImage`/`.deb`/`.tar.gz`, and Windows `.exe` via GitHub Actions.

  ```bash
  git tag v0.1.0 && git push origin v0.1.0
  ```

- **Container / in-cluster**: build the image with `make image` and deploy with
  [`deploy/k8sense-web.yaml`](./deploy/k8sense-web.yaml).


## Air-gapped mode

For regulated / on-prem deployments where nothing may leave the perimeter, set
`K8SENSE_AIRGAPPED=1`. The backend then contacts **only the Kubernetes API** you
point it at: no telemetry exporters, no external URL proxying (the plugin
catalog), and the desktop app skips update checks and MCP. On-prem OIDC (your
own identity provider) still works, since it's inside the perimeter. Verifiable
with a network monitor — the only egress is to the cluster API.

## Built on Headlamp

K8sense is built on [Headlamp](https://github.com/kubernetes-sigs/headlamp)
(Apache-2.0) and extends it with the Cluster Doctor engine, Guided Fix, the
audit log, and the Network Map. The original Apache-2.0 attribution is retained
in [`NOTICE`](./NOTICE), as the licence requires.

## License

Released under the [Apache 2.0](./LICENSE) license.
