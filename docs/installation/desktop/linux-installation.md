---
title: Linux Installation
sidebar_label: Linux
sidebar_position: 1
---

We ship K8sense the Linux desktop in several formats: [Flatpak](#flatpak), [AppImage](#appimage), [Tarballs](#tarballs).

## Flatpak

[Flatpak](https://flatpak.org/) gives an isolated and bundled way of running K8sense, with decoupled runtime updates (besides other [benefits](https://en.wikipedia.org/wiki/Flatpak#Features)).

Make sure you [install Flatpak and enable the flathub repository](https://flatpak.org/setup/), then install K8sense with the following command:

```bash
flatpak install io.kinvolk.K8sense
```

For running it, just launch it as usually in your Linux desktop, or run:

```bash
flatpak run io.kinvolk.K8sense
```

### Upgrading

To upgrading K8sense when it's installed via Flatpak, run:

```bash
flatpak update io.kinvolk.K8sense
```

### Running External Tools

When using tools like `az`, `aws`, `gcloud`, etc. from e.g. kubeconfig user's
exec, Flatpak will need to run these tools from outside the sandbox. For that
to work, you need to grant the *talk-name* of *org.freedesktop.Flatpak*. To do
this, use the [Flatseal](https://flathub.org/apps/com.github.tchx84.Flatseal)
application to change K8sense's permissions, or run the following command
(before running K8sense):

```shell
sudo flatpak override --talk-name=org.freedesktop.Flatpak io.kinvolk.K8sense
```

## AppImage

K8sense can be used as an [AppImage](https://appimage.org/) by downloading and running it directly.

To download, choose the AppImage file from the [latest release page](https://github.com/kubernetes-sigs/k8sense/releases/latest).
You can then run it with the following command (exemplified for the AMD64, 0.16.0 version):

```bash
./K8sense-0.16.0-linux-x64.AppImage
```

## Tarballs

To run K8sense from one of the tarballs, first download the tarball for the [latest release](https://github.com/kubernetes-sigs/k8sense/releases/latest). Then, extract the contents from it and run
the `k8sense` binary in the resulting folder (exemplified below for the AMD64, 0.16.0 version):

```bash
tar xvzf ./K8sense-0.16.0-linux-x64.tar.gz
cd K8sense-0.16.0-linux-x64
./k8sense
```
