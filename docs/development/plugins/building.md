---
title: Building and Shipping Plugins
sidebar_label: Building & Shipping
---

Once you have a plugin ready, you may want to build it for production and
deploy with K8sense or publish it for other K8sense users to enjoy.

## Deploying Plugins (General Information)

Once your plugin is built and tested, you need to deploy it to your K8sense instances. This section covers all deployment scenarios.

Let's assume that K8sense will be run with the `-plugins-dir` option set to
`/k8sense/plugins`, which is the default for in-cluster deployments.

### Plugin Directory Structure

K8sense expects plugins to follow a specific directory structure:

```
my-plugins/                 # Plugin root directory
├── MyPlugin1/
│   ├── main.js             # Built plugin file
│   └── package.json        # Plugin metadata
├── MyPlugin2/
│   ├── main.js
│   └── package.json
└── MyPlugin3/
    ├── main.js
    └── package.json
```

### Extracting Built Plugins

To extract a single plugin, you can package it first, then extract the package to the right place:

```bash
npm install
npm run build
npm run package

# Extract single plugin
tar xvzf my-first-plugin-0.1.0.tar.gz -C /k8sense/plugins
```

If you prefer to export one or more plugins directly, use the `k8sense-plugin` tool. Run `npm run build` first. Then use the `extract` option on a folder with K8sense plugins.

For a directory like this:

```
# Directory structure
my-plugins/
├── MyPlugin1/
│   ├── dist/
│   │   └── main.js
│   └── package.json
└── MyPlugin2/
    ├── dist/
    │   └── main.js
    └── package.json
```

You can extract the plugins into a target directory like this:

```bash
npx @kinvolk/k8sense-plugin extract ./my-plugins /k8sense/plugins
```

## Plugins in K8sense Desktop

K8sense Desktop has a Plugin Catalog to install plugins easily. It includes plugins from K8sense developers and the community.

By default, only official plugins in the Plugin Catalog are allowed. The catalog confirms which plugin you want to install. It also shows where the plugin will be downloaded from.

:::important
The Plugin Catalog allows users to change the default behavior and instead show all
plugins. It is however extremely important that you only run plugins that you
trust, as plugins run in the same JavaScript context as the main application.
:::

To learn how to publish your plugin to make it available in the Plugin Catalog for other users, see the [Publishing Plugins guide](./publishing.md).

### Manual Installation

First, build and package the plugin in the plugin folder:

```bash
cd my-plugin/
npm install
npm run build
npm run package
```

You can install the plugin in the K8sense desktop app by exporting the plugin
archive to the plugins directory. E.g.:

On Linux/macOS:

```bash
mkdir -p ~/.config/K8sense/plugins/
tar xvf my-first-plugin-0.1.0.tar.gz -C ~/.config/K8sense/plugins/
```

These are the default plugin directory locations for the K8sense desktop app:

| Operating System | Default Plugin Directory |
|------------------|--------------------------|
| **MacOS** | `$HOME/.config/K8sense/plugins` |
| **Linux** | `$HOME/.config/K8sense/plugins` |
| **Windows** | `%APPDATA%/K8sense/Config/plugins` |

## Plugins in K8sense Deployments

### Using InitContainer with a Plugin Image

When deploying K8sense with plugins, it is easier to use a container image with the plugins already installed. Then, use an init container to mount the plugins into the K8sense container.

Some plugins already have a published container image. For K8sense's official plugins, see this [list](https://github.com/orgs/k8sense-k8s/packages?tab=packages&q=k8sense-plugin).

You can thus deploy K8sense with an init container, such as the [Flux UI plugin image](ghcr.io/k8sense-k8s/k8sense-plugin-flux:v0.3.0):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k8sense-with-flux
  labels:
    app: k8sense-with-flux
spec:
  selector:
    matchLabels:
      app: k8sense-with-flux
  template:
    metadata:
      labels:
        app: k8sense-with-flux
    spec:
      initContainers:
      - name: fetch-plugins
        image: ghcr.io/k8sense-k8s/k8sense-plugin-flux:latest
        # Copy the plugins
        command: ["/bin/sh", "-c"]
        args: ["cp -r /plugins/* /k8sense/plugins/ && ls -l /k8sense/plugins"]
        volumeMounts:
        - name: plugins
          mountPath: /k8sense/plugins
      containers:
      - name: k8sense
        image: ghcr.io/k8sense-k8s/k8sense:latest
        args: ["-plugins-dir=/k8sense/plugins"]
        ports:
        - containerPort: 4466
        volumeMounts:
        - name: plugins
          mountPath: /k8sense/plugins
      volumes:
      - name: plugins
        emptyDir: {}
```

## Creating a Plugin Image

The K8sense official plugins repository has a [Dockerfile](https://github.com/k8sense-k8s/plugins/blob/main/Dockerfile) to generate an image for a plugin. Here is how to use it with the Kompose plugin:

```bash
# Get the plugins
git clone https://github.com/k8sense-k8s/plugins k8sense-plugins

# Move to the plugins directory
cd k8sense-plugins

# Build a container image for the kompose plugin
docker build --build-arg PLUGIN=kompose -t kompose-plugin:latest -f ./Dockerfile .
```

After this step you will have a `kompose-plugin:latest` image that you can use in your deployments, with the actual kompose plugin in its /plugins/kompose directory.
