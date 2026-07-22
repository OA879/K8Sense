---
title: In-cluster
sidebar_position: 1
---

A common use case for any Kubernetes web UI is to deploy it in-cluster and
set up an ingress server for having it available to users.

## Using Helm

The easiest way to install k8sense in your existing cluster is to
use [helm](https://helm.sh/docs/intro/quickstart/) with our [helm chart](https://github.com/kubernetes-sigs/k8sense/tree/main/charts/k8sense).

```bash
# first add our custom repo to your local helm repositories
helm repo add k8sense https://kubernetes-sigs.github.io/k8sense/

# now you should be able to install k8sense via helm
helm install my-k8sense k8sense/k8sense --namespace kube-system
```

As usual, it is possible to configure the helm release via the [values file](https://github.com/kubernetes-sigs/k8sense/blob/main/charts/k8sense/values.yaml) or setting your preferred values directly.

```bash
# install k8sense with your own values.yaml
helm install my-k8sense k8sense/k8sense --namespace kube-system -f values.yaml

# install k8sense by setting your values directly
helm install my-k8sense k8sense/k8sense --namespace kube-system --set replicaCount=2
```

### Cluster Inventory

K8sense can discover clusters from Cluster Inventory API `ClusterProfile`
resources when Cluster Inventory is enabled. The Helm chart configures the
backend flags from `config.clusterInventory` and mounts an access provider
config file.

Create `cluster-inventory-values.yaml`:

```yaml
config:
  clusterInventory:
    enabled: true
    accessProvidersConfig:
      providers:
        - name: secretreader
          execConfig:
            apiVersion: client.authentication.k8s.io/v1
            command: /access-plugins/secretreader/bin/secretreader-plugin
            interactiveMode: Never
            provideClusterInfo: true
        - name: kubeconfig-secretreader
          execConfig:
            apiVersion: client.authentication.k8s.io/v1
            command: /access-plugins/kubeconfig-secretreader/bin/kubeconfig-secretreader-plugin
            interactiveMode: Never
            provideClusterInfo: true
    plugins:
      - name: secretreader
        image: registry.k8s.io/cluster-inventory-api/secretreader:v0.1.3@sha256:ec3090dc166aa2b42fb35d714d161c417d8b27bbc463404c8f615f5f4c610a1d
        mountPath: /access-plugins/secretreader
      - name: kubeconfig-secretreader
        image: registry.k8s.io/cluster-inventory-api/kubeconfig-secretreader:v0.1.3@sha256:b92966cc6e4ac78002a63862921022a71d54956826f6e4febcb7247495eb98c0
        mountPath: /access-plugins/kubeconfig-secretreader
```

Then install K8sense with the values file:

```bash
helm install my-k8sense k8sense/k8sense --namespace kube-system --values cluster-inventory-values.yaml
```

The `accessProvidersConfig` object is the provider config consumed by the
Cluster Inventory access provider. The `plugins` entries are Kubernetes `image`
volumes that mount provider binaries into the K8sense container, not K8sense
UI plugins. By default, K8sense watches `ClusterProfile` resources that do not
have the `k8sense.dev/ignore` label because the chart sets
`config.clusterInventory.labelSelector` to `!k8sense.dev/ignore`.

## Using simple yaml

We also maintain a simple/vanilla [file](https://github.com/kubernetes-sigs/k8sense/blob/main/kubernetes-k8sense.yaml)
for setting up a K8sense deployment and service. Be sure to review it and change
anything you need.

If you're happy with the options in this deployment file, and assuming
you have a running Kubernetes cluster and your `kubeconfig` pointing to it,
you can run:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/k8sense/main/kubernetes-k8sense.yaml
```

## Optional TLS Backend Termination

K8sense supports optional TLS termination at the backend server. The default is to terminate at the ingress (default) or optionally directly at the K8sense container. This enables use cases such as NGINX TLS passthrough and transport server. See [tls](./tls.md) for details and usage.

## Use a non-default kube config file

By default, K8sense uses the default service account from the namespace it is deployed to, and generates a kubeconfig from it named `main`.

If you wish to use another specific non-default kubeconfig file, then you can do it by mounting it to the default location at `/home/k8sense/.config/K8sense/kubeconfigs/config`, or
providing a custom path K8sense with the ` -kubeconfig` argument or the KUBECONFIG env (through helm values.env)

### Use several kubeconfig files

If you need to use more than one kubeconfig file at the same time, you can list
each config file path with a ":" separator in the KUBECONFIG env.

## Exposing K8sense with an ingress server

With the instructions in the previous section, the K8sense service should be
running, but you still need the
ingress server as mentioned. We provide a sample ingress YAML file
for this purpose, but you have to manually replace the **URL** placeholder
with the desired URL. The ingress file also assumes that you have Contour
and a cert-manager set up, but if you don't, then you'll just not have TLS.

Assuming your URL is `k8sense.mydeployment.io`, getting the sample ingress
file and changing the URL can quickly be done by:

```bash
curl -s https://raw.githubusercontent.com/kubernetes-sigs/k8sense/main/kubernetes-k8sense-ingress-sample.yaml | sed -e s/__URL__/k8sense.mydeployment.io/ > k8sense-ingress.yaml
```

and with that, you'll have a configured ingress file, so verify it and apply it:

```bash
kubectl apply -f ./k8sense-ingress.yaml
```

## Exposing K8sense with port-forwarding

If you want to quickly access K8sense (after having its service running) and
don't want to set up an ingress for it, you can run use port-forwarding as follows:

```bash
kubectl port-forward -n kube-system service/k8sense 8080:80
```

and then you can access `localhost:8080` in your browser.

## Accessing K8sense

Once K8sense is up and running, be sure to enable access to it either by creating
a [service account](../#create-a-service-account-token) or by setting up
[OIDC](./oidc).

## Plugin Management

K8sense supports managing plugins through a sidecar container when deployed in-cluster.

### Using values.yaml

You can directly specify the plugin configuration in your `values.yaml`:

```yaml
config:
  watchPlugins: true
pluginsManager:
  enabled: true
  configContent: |
    plugins:
      - name: my-plugin
        source: https://artifacthub.io/packages/k8sense/my-repo/my_plugin
        version: 1.0.0
    installOptions:
      parallel: true
      maxConcurrent: 2
  baseImage: node:lts-alpine
  version: latest
```

### Using a Separate plugin.yml

Alternatively, you can maintain a separate `plugin.yml` file:

1. Create a `plugin.yml` file:
```yaml
plugins:
  - name: my-plugin
    source: https://artifacthub.io/packages/k8sense/my-repo/my_plugin
    version: 1.0.0
    # Optional: specify dependencies if needed
    dependencies:
      - another-plugin

installOptions:
  parallel: true
  maxConcurrent: 2
```

2. Install/upgrade K8sense using the plugin configuration:
```bash
helm upgrade --install my-k8sense k8sense/k8sense --namespace kube-system -f values.yaml --set pluginsManager.configContent="$(cat plugin.yml)"
```

### Plugin Configuration Format

The plugin configuration supports the following fields:

- `plugins`: Array of plugins to install
  - `name`: Plugin name (required)
  - `source`: Plugin source URL from Artifact Hub (required)
  - `version`: Plugin version (required)
  - `dependencies`: Array of plugin names that this plugin depends on (optional)
- `installOptions`:
  - `parallel`: Whether to install plugins in parallel (default: false)
  - `maxConcurrent`: Maximum number of concurrent installations when parallel is true

### Auto-updating Plugins

K8sense's plugin manager can automatically watch for changes in the plugin configuration. However, you need to enable watch for these changes in the main k8sense container. This can be enabled through the `watchPlugins` setting in `values.yaml`:

```yaml
config:
  watchPlugins: true  # Set to true to enable automatic plugin updates in main k8sense container
```

When enabled, any plugins' changes (either through Helm upgrades or direct ConfigMap updates) wil update in the main k8sense container by enabling --watch-plugins-changes flag on k8sense server.
