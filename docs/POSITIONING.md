# K8sense — Positioning

> **The air-gapped Kubernetes security & compliance platform for regulated environments.**

Lead with the wedge. Everything else is "…and it also does that." One buyer, one
pain, solved better than their status quo.

---

## The problem
Banks and other regulated operators run Kubernetes **on-prem and air-gapped** —
clusters that must never touch the internet. Yet almost every k8s tool
(observability, security scanning, GitOps, AI assistants) **assumes internet
access**: cloud control planes, hosted vuln databases, SaaS dashboards, phone-home
telemetry. So the teams who are held to the *highest* security and audit bar are
served by tools built for the opposite environment.

## Why existing tools don't fit
- **Lens / Rancher / cloud consoles** — great UX, but built around connected
  clusters and vendor cloud. Air-gap is an afterthought or impossible.
- **Cloud security scanners** — need a hosted CVE DB and egress.
- **AI copilots** — send your cluster data to a vendor API.

None of them can stand in front of a bank examiner and say *"nothing leaves the
perimeter."* K8sense can.

## The wedge — what actually wins the deal
1. **Provably air-gapped.** `K8SENSE_AIRGAPPED=1` disables all egress; a single
   internal-registry setting + one seed step makes every feature run offline.
   Verifiable, not a checkbox.
2. **Continuous compliance (CIS).** Audit-ready benchmark scoring with **drift
   alerts** — you're told the moment a cluster slips out of compliance.
3. **Offline vulnerability scanning (Trivy).** CVEs on the images actually
   running, with a mirrored DB — no internet, no SaaS.
4. **Audited, reversible actions.** Guided Fix + undo, every change logged with
   who/what/when — the change-control story auditors expect.
5. **Upgrade readiness.** Deprecated/removed API detection before an upgrade
   breaks production — a concrete pain most tools ignore.

That set is the product. It is coherent, differentiated, and maps 1:1 to what a
regulated operator is measured on.

## Proof points
- Zero egress in air-gapped mode (telemetry, update checks, proxies all off).
- Runs as a desktop app on macOS / Linux / **Windows** — no server to stand up.
- All diagnostics are API-only; no agents to install on nodes.
- Every mutating action is recorded and exportable for audit.

## Also includes (supporting depth — not the headline)
Cluster Doctor (65 checks), an offline **Copilot** (read-only, grounded, never
acts without a human), **Runbooks** (governed Ansible via in-cluster Jobs), an
**App Catalog** (one-click Helm), network/DB maps, cost & waste, incident
timeline, multi-cluster scanning. These make the platform *complete*; they are
not why someone switches.

## Ideal customer
- Regulated / banking / government running **on-prem or air-gapped** Kubernetes.
- Has a security & compliance mandate and an audit relationship.
- Frustrated that their security/ops tooling assumes internet.

## The one-sentence pitch
> **K8sense keeps your air-gapped Kubernetes clusters secure, compliant, and
> audit-ready — entirely inside your perimeter, with no internet, no agents, and
> no data leaving the building.**

## Positioning discipline
- **Don't** pitch "15 features." Pitch the wedge; let breadth surface as answers
  to "does it also…?"
- **Do** position the Copilot as a *read-only, offline advisor* — never as
  "AI that runs your cluster." Conservative buyers distrust the latter.
- **Do** frame Runbooks as *unified, in-cluster, audited* automation — not "we
  also do Ansible" (they already have AWX).
