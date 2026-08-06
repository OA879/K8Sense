# K8sense — End-to-End Test Checklist

Manual acceptance pass before cutting a release candidate. Unit tests and builds
are green; this validates the things only a **real cluster** can prove. Work
top to bottom. For anything that fails, note it under "If it fails" and file it.

Legend: ☐ = to do, mark ✅/❌ as you go.

---

## 0. Prerequisites
- ☐ A real cluster you can break (kind / minikube / a non-prod cluster) with a
  kubeconfig context loaded into K8sense.
- ☐ A **second** cluster added **live in the UI** (Add cluster → paste kubeconfig)
  — this is the "stateless cluster" path; test *both* kinds everywhere it matters.
- ☐ (Optional, for Copilot) Ollama running locally: `ollama pull qwen2.5 && ollama serve`.
- ☐ (Optional, for Runbooks/Vuln/Catalog) the target cluster can pull images
  from the internet, OR an internal registry seeded via `airgap/seed-registry.sh`.
- ☐ (Optional) a Slack/Teams incoming webhook URL for alert tests.

---

## 1. First-run / onboarding
- ☐ Fresh install shows the **Welcome screen**, then the **Feature Tour** (Back /
  Next / Skip / dots all work), then the app.
- ☐ Restarting does **not** show onboarding again.
- **If it fails:** check `k8sense.onboarded` / `k8sense.tourDone` localStorage flags.

## 2. Cluster connection (do the rest of the tests on BOTH cluster types)
- ☐ Disk kubeconfig cluster loads and is selectable.
- ☐ **Live-added (stateless)** cluster loads and is selectable.
- ☐ Settings → **Test Connection** reports reachable + k8s version for each.
- **If it fails:** stateless cluster 404s → confirm the browser is sending
  `KUBECONFIG` + `X-K8SENSE-USER-ID` headers (DevTools → Network).

## 3. Cluster Doctor
- ☐ Run a scan; findings appear, severity-sorted, within seconds.
- ☐ Open a finding — description + remediation present.
- ☐ Export report (HTML + JSON) downloads.
- ☐ Scan history lists past scans; diff/compare works.

## 4. Guided Fix + undo + audit
- ☐ A fixable finding shows a Guided Fix action with a plain-language prompt.
- ☐ Apply a **reversible** fix (e.g. scale / uncordon) → succeeds; cluster state
  actually changes (verify with kubectl).
- ☐ Audit Log shows the action (who/what/when/result).
- ☐ **Undo** the fix from the audit log → state reverts; a revert entry appears.
- **If it fails:** confirm the acting context has RBAC for the mutation.

## 5. Compliance / CIS
- ☐ Compliance page shows a score, pass/fail counts, controls grouped by section.
- ☐ Expand a failing control → violating resources listed.
- ☐ Create a bad resource (e.g. a privileged pod) → re-open → the matching control
  now fails.
- ☐ Monitoring banner shows (or the "enable scheduled scans" nudge).

## 6. Vulnerabilities (Trivy)  ⚠️ highest-risk / dependency-heavy
- ☐ "Scan running images" → phase progresses → a report appears.
- ☐ Severity totals + per-image expandable CVE table (CVE, package, installed→fixed).
- ☐ An image with known CVEs shows them; a clean image shows "clean".
- **If it fails:**
  - All images "scan failed" → the **Trivy image can't pull** or the **vuln DB
    can't download**. Set a working Trivy image + DB mirror in the gear.
  - Empty/garbled → check the Trivy Job's pod logs in `k8sense-vulnscan`.
  - Private images fail → add `imagePullSecrets` to the `k8sense-vulnscan` ns.

## 7. Copilot (offline AI)
- ☐ Status dot shows **Online** when a model is running (Offline otherwise, with
  setup help).
- ☐ Gear → set endpoint/model → status re-checks.
- ☐ Ask "what's critical right now?" → answer references **real** findings/resources.
- ☐ Ask "what is crashing?" → reflects live pod state.
- ☐ With no model running → clean "offline" message, no hang.
- **If it fails:** confirm the model endpoint is reachable from the K8sense host;
  a wrong model name returns a model error (surfaced plainly).

## 8. App Catalog (Helm)  ⚠️ real install/uninstall
- ☐ Catalog lists apps with correct install status.
- ☐ **Install Metrics Server** (smallest) → pods appear in `kube-system`.
- ☐ Status flips to installed.
- ☐ **Uninstall** → confirm dialog → pods are **gone** (verify with kubectl).
- ☐ Install a bigger one (kube-prometheus-stack) → completes (may take 20–60s).
- **If it fails:**
  - "chart repo unreachable" → air-gapped? bundle the chart in `charts/` or set an
    internal registry.
  - install hangs → chart download; check the release with `helm list -A`.

## 9. Runbooks (Ansible)  ⚠️ highest-risk / image dependency
- ☐ Runbooks → **Enable on this cluster** → succeeds (creates `k8sense-runbooks`
  ns + runner SA + RBAC; verify with kubectl).
- ☐ Pick "Apply default-deny NetworkPolicy" → enter a namespace → **Dry run** →
  live output streams, reports "would change", applies **nothing**.
- ☐ **Run** → output streams to `Succeeded`; the NetworkPolicy actually exists
  (kubectl get netpol -n <ns>).
- ☐ Re-run Cluster Doctor / Compliance → the CIS 5.3.2 finding for that ns is gone.
- ☐ Audit log records the run.
- **If it fails:**
  - Run fails at Ansible collection load → the **runner image lacks
    `kubernetes.core`**. Set a runner image that includes it (gear → Runner image).
    *This is the known rough edge — validate it here first.*
  - Enable fails → your context can't create namespaces/RBAC.
  - ImagePullBackOff → runner image not pullable (air-gap: mirror + set it).

## 10. Scheduling + drift
- ☐ Settings → enable **scheduled scans** with a short interval + set a webhook.
- ☐ Wait for a scheduled run (or lower the interval floor is 5 min) → a scan runs
  automatically; a compliance snapshot is recorded.
- ☐ Compliance page shows a **score-trend sparkline** after ≥2 runs.
- ☐ Introduce drift (make a namespace non-compliant) → next scheduled run fires a
  **webhook alert** naming the newly-failing control.
- ☐ New critical finding also fires the existing finding alert.
- **If it fails:** confirm `NotifyCritical` is on and the webhook URL is valid
  (Settings → Test notification).

## 11. Maps / timeline / cost / upgrade
- ☐ Network Map renders; inferred connections + live mesh overlay (if a mesh exists).
- ☐ DB Map renders.
- ☐ "What changed?" timeline merges actions + events + findings.
- ☐ Cost & Waste shows reservation bars + waste items + monthly estimate.
- ☐ Upgrade Readiness lists deprecated/removed APIs per resource with replacements.

## 12. Multi-cluster
- ☐ Multi-cluster scan runs across selected clusters (bounded to 5 concurrent).
- ☐ Per-cluster results/errors reported individually.

## 13. Air-gapped mode  ⚠️ the core wedge — prove it
- ☐ Start backend with `K8SENSE_AIRGAPPED=1`.
- ☐ **Update check does not fire** (no GitHub call — verify no egress with a proxy
  log or `lsof`/firewall).
- ☐ Settings → **Internal registry** → set it → Runbooks/Vuln default images now
  show the internal-registry prefix.
- ☐ `airgap/seed-registry.sh <registry>` copies images (dry-run on a connected host).
- ☐ Drop a chart `.tgz` in `charts/` → App Catalog installs it **without** a repo call.
- ☐ With the network cable pulled (or egress firewalled): Cluster Doctor,
  Compliance, maps, cost, upgrade all still work; Copilot works if a local model
  is running.
- **If it fails:** any outbound connection attempt in air-gapped mode is a bug —
  capture the destination.

## 14. Error clarity
- ☐ Hit a feature with a bogus cluster → message is **plain + actionable**
  ("Cluster … isn't connected …"), never raw JSON / HTTP codes.
- ☐ Trigger an unreachable-backend state → plain "backend couldn't be reached".

## 15. Cross-platform packaging
- ☐ macOS `.dmg` (arm64 + x64) installs and launches (right-click → Open for
  unsigned).
- ☐ Windows `.exe` installs and launches ("More info → Run anyway").
- ☐ Linux `.AppImage` / `.deb` runs.
- ☐ On each: backend starts, a cluster loads, a scan runs.
- **If it fails:** macOS "different Team IDs" → quarantine/signing; check
  `xattr -dr com.apple.quarantine`.

---

## Sign-off
- ☐ All ⚠️ (high-risk) sections pass on a real cluster.
- ☐ Air-gapped section (13) passes with verified zero egress.
- ☐ Known issues logged with severity.
- ☐ **Then**: bump to **1.0.0** and cut the release candidate.
