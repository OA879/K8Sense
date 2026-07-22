---
title: "Tutorial 2: Creating Your First Plugin"
sidebar_label: "2. Creating Your First Plugin"
sidebar_position: 3
---

# Creating Your First K8sense Plugin

This tutorial guides you through creating your first K8sense plugin from scratch. By the end, you'll have a working plugin that appears in K8sense's UI and you'll understand how hot reloading makes plugin development fast and enjoyable.

---

## Table of Contents

1. [Introduction](#introduction)
2. [Understanding the Plugin System](#understanding-the-plugin-system)
3. [Create Your First Plugin](#create-your-first-plugin)
4. [Explore the Plugin Structure](#explore-the-plugin-structure)
5. [Run the Plugin in Development Mode](#run-the-plugin-in-development-mode)
6. [See Your Plugin in K8sense](#see-your-plugin-in-k8sense)
7. [Hot Reloading in Action](#hot-reloading-in-action)
8. [Understanding What Happened](#understanding-what-happened)
9. [Troubleshooting](#troubleshooting)
10. [What's Next](#whats-next)
11. [Quick Reference](#quick-reference)

---

## Introduction

In [Tutorial 1](../running-from-source/), you set up K8sense to run locally from source. Now it's time to extend it!

**Plugins** let you add new features to K8sense without modifying its core code. You can:

- Add buttons, menus, and panels to the UI
- Create entirely new pages
- Customize how Kubernetes resources are displayed
- Change themes and branding
- And much more!

### What You'll Build

In this tutorial, you'll create a simple plugin called `hello-k8sense` that:

1. Displays "Hello" in K8sense's top navigation bar
2. Appears in the Settings → Plugins list

This gives you the foundation for all future plugin development.

### Prerequisites

Before starting, ensure you have:

- ✅ Completed [Tutorial 1: Running K8sense from Source](../running-from-source/)
- ✅ K8sense running locally (or ready to start)
- ✅ Node.js ≥22.0.0 and npm ≥11.0.0

Verify your setup:

```bash
node --version    # Should be v22.0.0 or higher
npm --version     # Should be 11.0.0 or higher
```

**Time to complete:** ~15 minutes

---

## Understanding the Plugin System

Before we dive into code, let's understand how plugins work at a high level.

### How Plugins Work

```
┌─────────────────────────────────────────────────────────┐
│                      K8sense                           │
│  ┌─────────────────────────────────────────────────┐    │
│  │                  Plugin Registry                │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐          │    │
│  │  │ Plugin A│  │ Plugin B│  │ Plugin C│   ...    │    │
│  │  └─────────┘  └─────────┘  └─────────┘          │    │
│  └─────────────────────────────────────────────────┘    │
│                         ↓                               │
│  ┌─────────────────────────────────────────────────┐    │
│  │               K8sense UI                       │    │
│  │   (App Bar, Sidebar, Pages, Details Views...)   │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

1. **Plugins are JavaScript/TypeScript modules** — They export code that K8sense loads at startup
2. **Plugins register themselves** — Using functions like `registerAppBarAction()` to tell K8sense where to display their components
3. **K8sense discovers plugins automatically** — During development, plugins make themselves available to K8sense

### Plugin Locations

K8sense looks for plugins in specific directories depending on how you're running it:

| Mode | Plugin Location |
|------|-----------------|
| **Development** | `~/.config/K8sense/plugins/` (Linux/macOS) or `%APPDATA%\K8sense\Config\plugins\` (Windows) — plugins running `npm start` automatically copy here |
| **Desktop App** | `~/.config/K8sense/plugins/` (Linux/macOS) or `%APPDATA%\K8sense\Config\plugins\` (Windows) |
| **In-Cluster** | Configured via K8sense deployment |

For this tutorial, we'll use **development mode**, the easiest way to build and test plugins.

---

## Create Your First Plugin

Let's create your first plugin! We'll use the `k8sense-plugin` tool which scaffolds a ready-to-use plugin project.

### Step 1: Choose a Location

Create your plugin **outside** the K8sense repository. This keeps your plugin code separate and organized.

```bash
# Go to your projects directory (create one if needed)
mkdir -p ~/projects
cd ~/projects
```

> **Why outside the K8sense repo?** Plugins are independent projects with their own dependencies. Keeping them separate makes them easier to manage, version, and share.

### Step 2: Create the Plugin

Run the following command to scaffold a new plugin:

```bash
npx --yes @kinvolk/k8sense-plugin create hello-k8sense
```

You'll see output like this:

```
Creating plugin: hello-k8sense...
...
Run `npm audit` for details.
"hello-k8sense" created.
1) Run the K8sense app (so the plugin can be used).
2) Open hello-k8sense/src/index.tsx in your editor.
3) Start development server of the plugin watching for plugin changes.
  cd "hello-k8sense"
  npm run start
4) See the plugin inside K8sense.
```

**What just happened?**

- `npx` downloaded and ran the `@kinvolk/k8sense-plugin` tool
- The `create` command generated a new plugin folder called `hello-k8sense`
- Dependencies were automatically installed (you saw `npm ci` in the output)
- The folder contains all the files you need to start developing

---

## Explore the Plugin Structure

Let's look at what was created:

```
hello-k8sense/
├── src/
│   └── index.tsx         # 👈 Main entry point - your plugin code goes here
├── package.json          # 👈 Plugin metadata and npm scripts
├── tsconfig.json         # TypeScript configuration
└── README.md             # Plugin documentation
```

### The Entry Point: `src/index.tsx`

Open `src/index.tsx` in your editor. You'll see:

```tsx
import { registerAppBarAction } from '@kinvolk/k8sense-plugin/lib';

// Below are some imports you may want to use.
//   See README.md for links to plugin development documentation.
// import { K8sense, K8s, useTranslation } from '@kinvolk/k8sense-plugin/lib';
// import { SectionBox } from '@kinvolk/k8sense-plugin/lib/CommonComponents';
// import { K8s } from '@kinvolk/k8sense-plugin/lib/K8s';
// import { Typography } from '@mui/material';

registerAppBarAction(<span>Hello</span>);
```

**Let's break this down:**

| Line | What it does |
|------|--------------|
| `import { registerAppBarAction }` | Imports a function from the K8sense plugin SDK |
| `registerAppBarAction(<span>Hello</span>)` | Registers a React component to display in the app bar |

That's it! Just two lines of meaningful code, and you have a working plugin.

### The Metadata: `package.json`

Open `package.json` to see your plugin's configuration:

```json
{
  "name": "hello-k8sense",
  "version": "0.1.0",
  "description": "Your K8sense plugin",
  "scripts": {
    "start": "k8sense-plugin start",
    "build": "k8sense-plugin build",
    "format": "k8sense-plugin format",
    "lint": "k8sense-plugin lint",
    "lint-fix": "k8sense-plugin lint --fix",
    "tsc": "k8sense-plugin tsc",
    "test": "k8sense-plugin test",
    "package": "k8sense-plugin package"
  },
  ...
}
```

**Key scripts you'll use:**

| Script | Purpose |
|--------|---------|
| `npm start` | Run plugin in development mode with hot reloading |
| `npm run build` | Build plugin for production |
| `npm run lint` | Check code for issues |
| `npm run format` | Auto-format your code |

---

## Run the Plugin in Development Mode

Now let's see your plugin in action! You'll need two terminal windows.

### Terminal 1: Start K8sense

Navigate to your K8sense repository and start it:

```bash
cd ~/git/k8sense  # or wherever you cloned K8sense
npm start
```

Wait until you see both backend and frontend are running:
- Frontend: http://localhost:3000
- Backend: http://localhost:4466

### Terminal 2: Start Your Plugin

In a new terminal, navigate to your plugin and start it:

```bash
cd ~/projects/hello-k8sense
npm start
```

You'll see output like:

```
Watching for changes...
Plugin is available for K8sense
```

**What's happening?**

- The plugin is being compiled from TypeScript to JavaScript
- It's watching for file changes (for hot reloading)
- It's announcing itself to K8sense running on localhost

> **Keep both terminals running!** K8sense needs to be running for your plugin to appear. The plugin's `npm start` watches for changes and automatically rebuilds.

---

## See Your Plugin in K8sense

Open your browser and go to **http://localhost:3000**.

### Step 1: Look for "Hello" in the App Bar

Look at the top-right area of the screen (the app bar). You should see **"Hello"** appearing among the icons!

![Screenshot of the K8sense app bar with a red highlight box around the "Hello" text in the top-right corner](./hello-in-appbar.png)

### Step 2: View Your Plugin in Settings

1. Click the **Settings** icon (⚙️) in the app bar
2. Click on **Plugins** in the settings menu

You'll see a list of all loaded plugins, including your `hello-k8sense` plugin!

![Screenshot of the K8sense Plugins settings page with a red highlight box around the hello-k8sense plugin entry in the table](./plugin-in-settings.png)

The plugin entry shows:
- **Name**: hello-k8sense
- **Version**: 0.1.0 (shown below the name)
- **Description**: Your K8sense plugin (from package.json)
- **Type**: Development (indicates this is a development plugin)
- **Enable/Disable toggle** (Desktop App only): You can turn plugins on or off in the desktop app

🎉 **Congratulations!** Your plugin is running in K8sense!

---

## Hot Reloading in Action

One of the best features of plugin development is **hot reloading**—changes you make appear instantly without restarting anything.

### Make a Change

Open `src/index.tsx` in your editor and change:

```tsx
registerAppBarAction(<span>Hello</span>);
```

To:

```tsx
registerAppBarAction(<span>🚀 Hello K8sense!</span>);
```

### Save and Watch

1. Save the file
2. Look at your plugin terminal—you'll see it rebuilding
3. Look at your browser—the app bar updates automatically!

![Screenshot of the K8sense app bar with a red highlight box around the "🚀 Hello K8sense!" text in the top-right corner](./hello-k8sense-emoji.png)

**No manual refresh needed!** This makes development fast and enjoyable.

### Try Another Change

Let's make it more interesting. Replace the entire content of `src/index.tsx`:

```tsx
import { registerAppBarAction } from '@kinvolk/k8sense-plugin/lib';
import { Button } from '@mui/material';

function HelloButton() {
  const handleClick = () => {
    alert('Hello from your first K8sense plugin!');
  };

  return (
    <Button
      variant="outlined"
      size="small"
      onClick={handleClick}
      sx={{ color: 'inherit', borderColor: 'inherit', mx: 1 }}
    >
      Say Hello
    </Button>
  );
}

registerAppBarAction(<HelloButton />);
```

Save the file and watch the magic:

1. The text changes to a **"Say Hello"** button
2. Click the button to see an alert!

![Screenshot showing the K8sense app bar with a red highlight box around the "Say Hello" button in the top-right corner, and a browser alert dialog with a red highlight box around the "Hello from your K8sense plugin!" message](./say-hello-button.png)

**What changed?**

- We imported `Button` from Material-UI (already available through K8sense)
- We created a React component `HelloButton` with click handling
- We registered that component instead of a plain `<span>`

This is the foundation of all plugin development—creating React components and registering them with K8sense.

---

## Understanding What Happened

Let's recap what makes your plugin work:

### The Register Function

```tsx
registerAppBarAction(<HelloButton />);
```

This tells K8sense: *"Hey, I have a component. Please display it in the app bar."*

### Available Register Functions

`registerAppBarAction` is just one of many registration functions. Here are some others you'll learn about:

| Function | What it does |
|----------|--------------|
| `registerAppBarAction` | Add items to the top navigation bar |
| `registerSidebarEntry` | Add items to the left sidebar menu |
| `registerRoute` | Create new pages/routes |
| `registerDetailsViewSection` | Add sections to resource detail pages |
| `registerPluginSettings` | Add a settings panel for your plugin |

Each function lets you extend a different part of K8sense. We'll explore these in upcoming tutorials!

### Shared Dependencies

Notice how we imported `Button` from `@mui/material`:

```tsx
import { Button } from '@mui/material';
```

You didn't need to install Material-UI—K8sense provides it! These shared dependencies are available to all plugins:

- **React** — UI framework
- **Material-UI** (`@mui/material`) — Component library
- **React Router** — Navigation
- **And more...**

This keeps plugins small and ensures consistent styling across K8sense.

---

## Troubleshooting

### Plugin Not Appearing in K8sense

**Check if both are running:**
- K8sense should be running (`npm start` in K8sense folder)
- Plugin should be running (`npm start` in plugin folder)

**Check the plugin terminal for errors:**
```bash
# In your plugin folder
npm start
```

Look for any red error messages.

**Try restarting both:**
1. Stop the plugin (`Ctrl+C`)
2. Stop K8sense (`Ctrl+C`)
3. Start K8sense first, wait for it to be ready
4. Start the plugin

### Changes Not Reflecting (Hot Reload Not Working)

**Ensure you saved the file** — Hot reload only triggers on save.

**Check the plugin terminal** — You should see "Compiling..." when you save.

**Hard refresh the browser:**
- Windows/Linux: `Ctrl + Shift + R`
- macOS: `Cmd + Shift + R`

**Clear browser cache:**
1. Open Developer Tools (`F12`)
2. Right-click the refresh button
3. Select "Empty Cache and Hard Reload"

### Port Conflicts

If you see errors about ports being in use:

```bash
# Find what's using port 3000
lsof -i :3000

# Find what's using port 4466
lsof -i :4466
```

Kill the conflicting processes or use different ports.

### Build Errors

If you see TypeScript or build errors:

```bash
# Check for TypeScript issues
npm run tsc

# Check for linting issues
npm run lint

# Auto-fix some issues
npm run lint-fix
```

---

## What's Next

You've just created your first K8sense plugin! 🎉

This tutorial covered `registerAppBarAction`—just one small piece of what plugins can do. K8sense's plugin system offers many more capabilities:

- **Tutorial 3: Adding Sidebar Navigation** — Create menu items that link to your custom pages
- **Working with Kubernetes Data** — Fetch and display cluster information
- **Customizing Resource Views** — Modify how Kubernetes resources are displayed
- **Adding Custom Themes** — Change colors, fonts, and overall appearance
- **Publishing Plugins** — Share your creations with the community

We'll cover these in the upcoming tutorials. In the meantime, check out the [example plugins](https://github.com/kubernetes-sigs/k8sense/tree/main/plugins/examples) in the K8sense repository for inspiration!

---

## Quick Reference

### Plugin Commands

Run these from your plugin directory (`hello-k8sense/`):

| Task | Command |
|------|---------|
| Start development mode | `npm start` |
| Build for production | `npm run build` |
| Check code for issues | `npm run lint` |
| Fix linting issues | `npm run lint-fix` |
| Format code | `npm run format` |
| Type check | `npm run tsc` |
| Run tests | `npm run test` |
| Package for distribution | `npm run package` |

### K8sense Commands

Run these from your K8sense directory:

| Task | Command |
|------|---------|
| Start K8sense (backend + frontend) | `npm start` |
| Start desktop app | `npm run app:start` |

### Key Files

| File | Purpose |
|------|---------|
| `src/index.tsx` | Main plugin entry point |
| `package.json` | Plugin metadata and scripts |
| `tsconfig.json` | TypeScript configuration |

### Useful Links

- [Plugin API Documentation](https://k8sense.dev/docs/latest/development/plugins/)
- [Example Plugins](https://github.com/kubernetes-sigs/k8sense/tree/main/plugins/examples)
- [#k8sense on Kubernetes Slack](https://kubernetes.slack.com/messages/k8sense)
- [GitHub Issues](https://github.com/kubernetes-sigs/k8sense/issues)
