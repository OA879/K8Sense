---
title: Frequently Asked Questions
sidebar_position: 5
---

## General Questions

### What is K8sense and who is it for?

K8sense is a graphical user interface specifically tailored to simplify the management of Kubernetes clusters.

### What Kubernetes flavors or vendors does K8sense support?

K8sense is designed to be vendor-agnostic, supporting a variety of Kubernetes flavors. For a full list of compatible platforms, please refer to our [platforms section in the documentation](./platforms.md).

### Is K8sense a desktop or web application?

K8sense is available both as a desktop application and a web application. The desktop version can be installed on your local machine and the web version can be accessed through a browser.

### Is there any cost involved in using K8sense?

K8sense is a 100% open source project, under the Apache 2.0 License. It is thus available at no charge, and users are encouraged to modify and redistribute it in accordance with the license terms.

### Can I use K8sense for commercial purposes?

Yes, and it's encouraged. K8sense is developed under the permissive Apache 2.0 License making it ideal for personal and commercial use.

### Where can I find the source code for K8sense?

The source code for K8sense is publicly available on [GitHub](https://github.com/kubernetes-sigs/k8sense). You are welcome to explore, fork, and contribute to the codebase.

### Who maintains K8sense?

You can find the list of K8sense's maintainers in its [OWNERS_ALIASES](https://github.com/kubernetes-sigs/k8sense/blob/main/OWNERS_ALIASES) file in the repository. As a 100% open source project and a CNCF Sandbox project, K8sense encourages any users/developers to participate in it.

### What capabilities / credentials does K8sense require to access my Kubernetes cluster?

K8sense doesn't require access to the cluster itself; it instead relies on RBAC to connect to the Kubernetes API server. This means that it's the user(s) who must have the required credentials to access the cluster (via a service token or client certificate). K8sense may store the token in the browser's local storage, but never in its backend/server.

### Is K8sense customizable?

Yes, K8sense is highly customizable, thanks to its robust plugin system. This system extends K8sense's core functionalities, catering to specific use cases and workflows. For more information on creating and managing plugins, visit our [plugins page](./development/plugins/building.md).

### How often is K8sense updated?

K8sense tries to have a new feature version released every month. Sometimes, there may be delays of a couple of weeks. Bug fix versions can also be released between feature versions, whenever appropriate. These are often released quickly after a fix is added.

---

## Installation and Setup

### How can I install and access K8sense?

To install K8sense, follow the detailed instructions provided in the [official installation guide](./installation/index.mdx). The process will guide you through downloading the application, setting up your Kubernetes cluster access, and launching K8sense to connect to your cluster. For desktop applications, you can find additional information in the [desktop installation guide](./installation/desktop/index.mdx).

### Can I install K8sense in my Kubernetes cluster?

Absolutely! K8sense can be deployed directly within your Kubernetes cluster. Detailed instructions for in-cluster installation can be found in the [in-cluster installation guide](./installation/in-cluster/index.md).

---

## Usage and Features

### Can I monitor multiple clusters with K8sense?

Yes, K8sense allows you to monitor multiple clusters. You can switch between different clusters using the cluster switcher in the K8sense interface.

### Can I manage my Kubernetes resources directly through K8sense?

Yes, K8sense enables direct management of Kubernetes resources through its user interface as allowed by the user's role and permissions.

### K8sense is not showing delete/edit/scale buttons in a resource, why is that?

K8sense shows controls based on the user's role (RBAC), so if the user has, e.g., no permissions to delete a resource, the delete button is not shown.

### I cannot access any section in my cluster, it keeps saying Access Denied.

By default, K8sense assumes users can list all namespaces. If you only have the permissions to list resources in certain namespaces, please access the cluster settings and set up the accessible namespaces.

### How do I add or remove plugins in K8sense?

To add or remove plugins in K8sense, you can follow the plugin management instructions provided in the [K8sense plugin documentation](./development/plugins/index.md).

### Is there a way to contribute to the development of K8sense features?

Absolutely! As an open source project, K8sense thrives on community contributions. You can contribute in various ways, including submitting pull requests, creating plugins, reporting issues, and suggesting new features. For more details on how to get involved, visit our [contribution section](./contributing.md).

### How can I get help or assistance for K8sense?

For support, refer to the [K8sense documentation](./development/index.md). For further assistance, join the [K8sense community on Slack](https://kubernetes.slack.com/messages/k8sense) or file an issue on the [GitHub issues page](https://github.com/kubernetes-sigs/k8sense/issues).

Join our [monthly community meeting](https://zoom-lfx.platform.linuxfoundation.org/meetings/k8sense) if you want to chat in a zoom call.
