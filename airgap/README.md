# K8sense air-gapped deployment

K8sense runs fully offline. Its own code never reaches the internet, and in
air-gapped mode (`K8SENSE_AIRGAPPED=1`) egress is disabled outright — no
telemetry, no update checks, no phone-home. The only things that need to come
from *somewhere* are the container images and Helm charts that features deploy
**into your cluster** — because Kubernetes nodes pull those from a registry, not
from the K8sense app.

This directory makes that turnkey: seed your internal registry once, point
K8sense at it once, done.

## What runs where

| Feature | Talks to | Offline? |
| --- | --- | --- |
| Cluster Doctor, Compliance, Maps, Timeline, Cost, Upgrade, Guided Fix | Cluster API only | ✅ nothing to configure |
| Copilot (AI) | A model **you host** (local Ollama or in-cluster) | ✅ once a model is running |
| Runbooks | An `ansible-runner` image in your registry | ✅ via internal registry |
| App Catalog | Bundled charts + images in your registry | ✅ via bundled charts + registry |
| Vulnerabilities | A Trivy image + Trivy DB in your registry | ✅ via internal registry |

## One-time setup

1. **Seed your internal registry** (on a host that can reach both the internet
   and your registry):

   ```bash
   skopeo login registry.bank.internal
   ./seed-registry.sh registry.bank.internal
   ```

   This copies every image in [`images.txt`](./images.txt) into your registry,
   preserving the repository path exactly the way K8sense rewrites references.

2. **Set the internal registry in K8sense** — once, globally:

   - UI: **Settings → Internal registry** → `registry.bank.internal`
   - or env: `K8SENSE_INTERNAL_REGISTRY=registry.bank.internal`

   Every feature's default image (runner, Trivy, Trivy DB) is now pulled from
   there. Explicit per-feature overrides (Runbooks runner image, Trivy image)
   still win if you set them.

3. **Bundle Helm charts** (App Catalog): drop the chart `.tgz` files into the
   charts directory so the catalog installs from disk instead of a public repo:

   - default: a `charts/` directory next to the `k8sense-server` executable
   - or env: `K8SENSE_CHARTS_DIR=/path/to/charts`

   `helm pull <repo>/<chart>` on a connected host produces these `.tgz` files.
   Without them, the catalog falls back to the public chart repo (which an
   air-gapped cluster can't reach), so bundle the ones you intend to offer.

4. **Host a model for the Copilot** (optional): run Ollama on the K8sense host,
   or point **Copilot → Configure** at any in-cluster / shared OpenAI-compatible
   endpoint. No API key, no internet.

## Notes

- **Image tags**: [`images.txt`](./images.txt) lists `:latest` for readability —
  pin the exact tags you validate before seeding.
- **Private registry auth**: if the internal registry needs credentials, ensure
  the relevant namespaces (`k8sense-runbooks`, `k8sense-vulnscan`, and each app's
  namespace) carry the appropriate `imagePullSecrets`.
- **Charts and `global.imageRegistry`**: charts that honour the Bitnami-style
  `global.imageRegistry` value pick up the internal registry automatically.
  Charts that don't may need their own image values set — bundle a values file
  or use a registry that supports pull-through/proxy caching.
