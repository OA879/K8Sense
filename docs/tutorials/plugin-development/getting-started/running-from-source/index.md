---
title: "Tutorial 1: Running K8sense from Source"
sidebar_label: "1. Running from Source"
sidebar_position: 2
---

# Running K8sense from Source

This tutorial guides you through building and running K8sense from source code. By the end, you'll have K8sense running locally in development mode—ready for plugin development or contributing to the project.

---

## Table of Contents

1. [Introduction](#introduction)
2. [Prerequisites](#prerequisites)
3. [Clone the Repository](#clone-the-repository)
4. [Repository Structure](#repository-structure)
5. [Install Dependencies](#install-dependencies)
6. [Build the Code](#build-the-code)
7. [Run in Development Mode](#run-in-development-mode)
8. [Run the Desktop App](#run-the-desktop-app)
9. [Connect to a Kubernetes Cluster](#connect-to-a-kubernetes-cluster)
10. [Troubleshooting](#troubleshooting)
11. [Next Steps](#next-steps)

---

## Introduction

**K8sense** is an open-source, extensible Kubernetes web UI. It provides:

- A clean, modern interface for managing Kubernetes clusters
- Multi-cluster support
- A powerful plugin system for customization
- Desktop and in-cluster deployment options

K8sense has three main components:

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Frontend** | TypeScript, React | The web UI you interact with |
| **Backend** | Go | API server that proxies requests to Kubernetes |
| **Desktop App** | Electron | Native app wrapper for macOS, Windows, Linux |

Building from source lets you modify K8sense, develop plugins with live reload, and contribute to the project.

---

## Prerequisites

Install these tools before proceeding:

### Required

| Tool | Version | Installation |
|------|---------|--------------|
| **Git** | Latest | [git-scm.com](https://git-scm.com/downloads) |
| **Node.js** | ≥22.0.0 LTS  | [nodejs.org](https://nodejs.org/en/download) or use [nvm](https://github.com/nvm-sh/nvm) |
| **npm**     | ≥11.0.0      | Included with Node.js |
| **Go**      | ≥1.25.9      | [go.dev/doc/install](https://go.dev/doc/install) |

### Optional (for testing with a cluster)

| Tool | Purpose | Installation |
|------|---------|--------------|
| **minikube** | Local Kubernetes cluster | [minikube.sigs.k8s.io](https://minikube.sigs.k8s.io/docs/start/) |
| **kubectl** | Kubernetes CLI | [kubernetes.io/docs/tasks/tools](https://kubernetes.io/docs/tasks/tools/) |

### Verify Installation

```bash
# Check versions
node --version    # Should be v22.0.0 or higher
npm --version     # Should be 11.0.0 or higher
go version        # Should be go1.25.9 or higher
git --version     # Any recent version
```

---

## Clone the Repository

### Option A: Fork First (for contributors)

1. Fork the repo at [github.com/kubernetes-sigs/k8sense](https://github.com/kubernetes-sigs/k8sense)
2. Clone your fork:

```bash
git clone https://github.com/YOUR_USERNAME/k8sense.git
cd k8sense
```

### Option B: Clone Directly

```bash
git clone https://github.com/kubernetes-sigs/k8sense.git
cd k8sense
```

---

## Repository Structure

K8sense is organized as a monorepo with three main components: a React **frontend** for the UI, a Go **backend** that proxies requests to Kubernetes, and an Electron **app** for the desktop version. All build commands are orchestrated from the root `package.json`.

```
k8sense/
├── frontend/          # React/TypeScript web UI
│   ├── src/           # Source code, components, and tests
│   └── package.json   # Frontend dependencies
│
├── backend/           # Go server
│   ├── cmd/           # Main application entry point
│   ├── pkg/           # Reusable packages (auth, cache, helm, etc.)
│   └── go.mod         # Go dependencies
│
├── app/               # Electron desktop application
│   ├── electron/      # Main process code
│   └── package.json   # App dependencies
│
├── plugins/           # Plugin system
│   ├── examples/      # Example plugins to learn from
│   └── k8sense-plugin/  # Plugin development tools
│
├── docs/              # Documentation (markdown)
├── package.json       # Root scripts for building/running
└── README.md          # Project overview
```

**Key insight**: The root `package.json` contains npm scripts that orchestrate building and running all components. You'll use commands like `npm run backend:build` and `npm run frontend:start` from the root directory.

---

## Install Dependencies

From the repository root, install all dependencies:

```bash
# Install root dependencies
npm install

# Install frontend dependencies
npm run frontend:install

# Install app dependencies (only needed if running desktop app)
npm run app:install
```

> **Note**: Go dependencies are fetched automatically during the build step. No separate install is needed for the backend.

**Quick install everything:**

```bash
npm run install:all
```

---

## Build the Code

### Build Everything

```bash
npm run build
```

This runs `backend:build` and `frontend:build` sequentially.

### Build Individually

```bash
# Build backend only (compiles Go → backend/k8sense-server binary)
npm run backend:build

# Build frontend only (compiles TypeScript → frontend/build/)
npm run frontend:build
```

> **Note**: Building creates the artifacts but doesn't run them. See the next section to start K8sense.

---

## Run in Development Mode

### Option 1: Run Both Together (Recommended)

```bash
npm start
```

This starts both backend and frontend with live reload. You'll see color-coded output:
- 🔵 Blue: Backend logs
- 🟢 Green: Frontend logs

![Screenshot of terminal output showing backend and frontend running with color-coded logs](./npm-start-terminal.png)

**Access points:**
- Frontend: [http://localhost:3000](http://localhost:3000)
- Backend API: [http://localhost:4466](http://localhost:4466)

Open http://localhost:3000 in your browser. You should see K8sense's welcome screen:

![Screenshot of K8sense running in a web browser showing the welcome screen](./k8sense-browser.png)

### Option 2: Run Separately (Two Terminals)

**Terminal 1 - Backend:**

```bash
npm run backend:build   # Build first (required)
npm run backend:start
```

**Terminal 2 - Frontend:**

```bash
npm run frontend:start
```

> **Tip**: Running separately is useful when you only want to restart one component.

---

## Run the Desktop App

The desktop app wraps the frontend and backend into a native application.

### Option 1: Full App Mode

Builds frontend and runs the complete Electron app:

```bash
npm run app:start
```

![Screenshot of the K8sense desktop application window](./k8sense-desktop-app.png)

### Option 2: App-Only Mode (Development)

If you already have `npm start` running (backend + frontend), you can run just the Electron shell:

```bash
npm run app:start:client
```

This connects to your running dev servers—useful for faster iteration on app-specific code.

### Option 3: Everything Together

Run backend, frontend, and desktop app all at once:

```bash
npm run start:with-app
```

---

## Connect to a Kubernetes Cluster

K8sense automatically detects Kubernetes clusters from your kubeconfig file. By default, it looks for the file at `~/.kube/config` on macOS/Linux or `%USERPROFILE%\.kube\config` on Windows. If you have multiple clusters configured, K8sense will show all of them and let you switch between them.

> **Already have a cluster?** If you already have `kubectl` working with a cluster (try `kubectl get nodes`), you can skip to the next section—K8sense will pick it up automatically.

### Quick Setup with minikube

```bash
# Start a local cluster
minikube start

# Verify it's running
kubectl get nodes
```

Now refresh K8sense, it should detect your cluster automatically.

![Screenshot of K8sense cluster overview showing minikube with resource metrics and sidebar navigation](./k8sense-with-cluster.png)

---

## Troubleshooting

### Port Already in Use

```bash
# Find and kill process on port 3000 (frontend)
lsof -i :3000  # macOS/Linux
netstat -ano | findstr :3000  # Windows

# Find and kill process on port 4466 (backend)
lsof -i :4466  # macOS/Linux
```

### Backend Won't Start

Ensure the binary exists:

```bash
ls backend/k8sense-server  # macOS/Linux
dir backend\k8sense-server.exe  # Windows
```

If missing, rebuild:

```bash
npm run backend:build
```

### Node/Go Version Mismatch

```bash
# Check versions match requirements
node --version  # Need ≥22.0.0
npm --version   # Need ≥11.0.0
go version      # Need ≥1.25.9
```

### kubeconfig Not Found

K8sense looks for `~/.kube/config` by default. Verify it exists:

```bash
cat ~/.kube/config  # macOS/Linux
type %USERPROFILE%\.kube\config  # Windows
```

### Clean Rebuild

When in doubt, clean and rebuild:

```bash
npm run clean
npm run install:all
npm run build
```

---

## Next Steps

🎉 **Congratulations!** You now have K8sense running from source!

Here's where to go next:

- **[Tutorial 2: Creating Your First Plugin](../creating-your-first-plugin/)** — Create your first K8sense plugin
- **[Architecture Overview](https://k8sense.dev/docs/latest/development/architecture/)** — Understand how K8sense is built
- **[Frontend Development](https://k8sense.dev/docs/latest/development/frontend/)** — Deep dive into the Frontend
- **[Backend Development](https://k8sense.dev/docs/latest/development/backend/)** — Learn about the Backend Server
- **[Contributing Guidelines](https://k8sense.dev/docs/latest/contributing/)** — How to submit changes

### Get Help

- 💬 [#k8sense on Kubernetes Slack](https://kubernetes.slack.com/messages/k8sense)
- 🐛 [GitHub Issues](https://github.com/kubernetes-sigs/k8sense/issues)
- 📖 [FAQ](https://k8sense.dev/docs/latest/faq/)

---

## Quick Reference

| Task | Command |
|------|---------|
| Install all dependencies | `npm run install:all` |
| Build everything | `npm run build` |
| Run dev mode (backend + frontend) | `npm start` |
| Run desktop app | `npm run app:start` |
| Run desktop app (dev, connects to running servers) | `npm run app:start:client` |
| Run everything including app | `npm run start:with-app` |
| Build backend only | `npm run backend:build` |
| Build frontend only | `npm run frontend:build` |
| Run tests | `npm test` |
| Lint code | `npm run lint` |
| Clean build artifacts | `npm run clean` |
