---
title: Getting Started with Plugin Development
sidebar_label: Getting Started
sidebar_position: 1
---

# Getting Started with Plugin Development

This guide will walk you through creating your first K8sense plugin from start to finish, whether you're a beginner or experienced developer.

## What are K8sense Plugins?

K8sense plugins are JavaScript/TypeScript modules that extend the functionality of K8sense's web interface. Plugins can:

- Add custom components to various parts of the UI (app bar, sidebar, details views)
- Create new routes and pages
- Customize how Kubernetes resources are displayed
- Add new themes and branding
- Integrate with external tools and services
- Provide custom settings and configuration options
- Setting up tokens for clusters, so users do not have to manually enter them
- Customize how and what columns of lists/tables are displayed

## Prerequisites

Before starting plugin development, ensure you have:

- **Node.js** (v22.0.0 or later) - [Download here](https://nodejs.org/)
- **npm** (v11.0.0 or later) - npm is typically installed with Node.js, but may require a separate upgrade
- **A running K8sense instance** - Either the [desktop app](../../installation/desktop/) or [development setup](../index.md)
- **Basic knowledge of JavaScript/TypeScript and React**

## Quick Start: Your First Plugin

Let's create a simple plugin that displays "Hello K8sense!" in the top navigation bar.

### Step 1: Create the Plugin

Run the following command to scaffold a new plugin using the k8sense-plugin
tool. This command should be run where your plugin project (code) will live, typically in a dedicated projects directory (do not run this in K8sense's
plugin installation directory).

```bash
# Create a new plugin
npx --yes @kinvolk/k8sense-plugin create my-first-plugin

# Navigate to the plugin directory
cd my-first-plugin

# Install dependencies
npm install
```

### Step 2: Understand the Plugin Structure

Your new plugin will have this structure:

```
my-first-plugin/
├── src/
│   └── index.tsx         # Main plugin entry point
├── package.json          # Plugin metadata and dependencies
├── tsconfig.json         # TypeScript configuration
├── dist/                 # Built plugin files (created after build)
└── README.md             # Plugin documentation
```

### Step 3: Examine the Default Code

Open `src/index.tsx` to see the default plugin code:

```tsx
import { registerAppBarAction } from '@kinvolk/k8sense-plugin/lib';

// Below are some imports you may want to use.
//   See README.md for links to plugin development documentation.
// import { SectionBox } from '@kinvolk/k8sense-plugin/lib/CommonComponents';
// import { K8s } from '@kinvolk/k8sense-plugin/lib/K8s';
// import { Typography } from '@mui/material';

registerAppBarAction(<span>Hello</span>);
```

### Step 4: Start Development Mode

```bash
npm run start
```

This command:
- Makes the plugin available to K8sense
- Watches file changes to automatically rebuild the plugin

### Step 5: See Your Plugin in Action

1. **Desktop App**: Open K8sense desktop app - it automatically detects plugins in development mode; or
2. **Run K8sense in development mode**: Start the K8sense development server (see [development guide](../index.md)).

You should see "Hello" text in the top navigation bar!

### Step 6: Make Your First Change

Let's create a more interactive component. Replace the content of `src/index.tsx`:

```tsx
import { registerAppBarAction } from '@kinvolk/k8sense-plugin/lib';
import { Button } from '@mui/material';

function HelloButton() {
  const handleClick = () => {
    alert('Hello from your K8sense plugin!');
  };

  return (
    <Button
      variant="outlined"
      size="small"
      onClick={handleClick}
      sx={{ mx: 2 }} // Add some horizontal margin
    >
      Hello K8sense!
    </Button>
  );
}

registerAppBarAction(<HelloButton />);
```

Save the file and watch K8sense automatically reload with your changes!

## Understanding Plugin Development

### Core Concepts

#### 1. Plugin Registry

The plugin registry is the central system that manages all plugin functionality. Import functions from `@kinvolk/k8sense-plugin/lib` to register your functionality. For example:

```tsx
import {
  registerAppBarAction,
  registerRoute,
  registerSidebarEntry,
  registerDetailsViewSection
} from '@kinvolk/k8sense-plugin/lib';
```

#### 2. Shared Dependencies

K8sense provides common libraries that plugins can use without bundling them:

- **React & React DOM** - For building UI components
- **React Router** - For navigation and routing
- **Redux** - For state management
- **Material-UI (@mui/material)** - UI component library
- **Material-UI Lab (@mui/lab)** - Additional Material-UI components
- **Lodash** - Utility functions
- **Iconify** - Icon library
- **Notistack** - Snackbar notifications
- **Monaco Editor** - Code editor component
- **@iconify/react** - Icon components

I.e., even though these components can be imported by the plugin as normal in its
code, they are not bundled with the plugin. Instead, they are provided by K8sense itself. This means there is no need to add them as dependencies in your plugin's `package.json`.

#### 3. Kubernetes API Access

Access Kubernetes resources using the built-in K8s module:

```tsx
import { K8s } from '@kinvolk/k8sense-plugin/lib';

function PodList() {
  const [pods, error] = K8s.ResourceClasses.Pod.useList();

  if (error) return <div>Error loading pods</div>;
  if (!pods) return <div>Loading...</div>;

  return (
    <div>
      <h3>Pods ({pods.length})</h3>
      {pods.map(pod => (
        <div key={pod.metadata.uid}>{pod.metadata.name}</div>
      ))}
    </div>
  );
}
```

### Development Workflow

#### 1. Development Mode

Always use `npm run start` during development for:
- Automatic rebuilding
- Hot reloading
- Real-time error checking

#### 2. Code Quality Tools

Your plugin comes with built-in quality tools:

```bash
# Format code
npm run format

# Check for linting issues
npm run lint

# Fix auto-fixable linting issues
npm run lint-fix

# Type checking
npm run tsc

# Run tests
npm run test
```

#### 3. Building for Production

When ready to deploy:

```bash
npm run build
npm run package
```

This will create a tarball archive that can be then extracted into the
K8sense plugins directory.

```bash
# Created tarball: "/tmp/my-first-plugin/my-first-plugin-0.1.0.tar.gz".
# Tarball checksum (sha256): c45397ff5f8fac563c2b85a18c0dbbe732017bed71b24bf852b809911993be6f
```

## Next Steps

Now that you've created your first plugin, explore these advanced topics:

1. **[Common Patterns](./common-patterns.md)** - Learn best practices and reusable patterns for plugin development
2. **[Example Plugins](https://github.com/kubernetes-sigs/k8sense/tree/main/plugins/examples)** - Learn from real-world examples
3. **[Plugin Functionality](./functionality/index.md)** - Complete reference of available APIs
4. **[Building & Shipping](./building.md)** - Production deployment strategies
5. **[Publishing Plugins](./publishing.md)** - Share your plugins with the community

## Troubleshooting

### Plugin Not Loading
- Ensure K8sense is running and accessible
- Check the browser console for JavaScript errors
- Verify your plugin's `package.json` has correct metadata
- Make sure you're running `npm run start` in the plugin directory

### Plugin's Changes Not Reflecting
- Ensure you saved your changes
- Check if the development server is running (`npm run start`) without errors
- Remove the installed plugin from K8sense's plugins folder (see [plugin locations](./architecture.md#plugin-locations)) and re-run `npm run start`
- Restart K8sense if necessary

### Build Errors
- Run `npm run lint` to check for code issues
- Ensure all imports are correct
- Check TypeScript errors with `npm run tsc`

### Hot Reloading Issues
- Restart the development server (`npm run start`)
- Make sure you do not have several K8sense tabs in case you are running
  in development mode
- Clear browser cache
- Check file permissions in the plugin directory

## Getting Help

- **Documentation**: [Complete API Reference](../api/)
- **Examples**: [Plugin Examples Repository](https://github.com/kubernetes-sigs/k8sense/tree/main/plugins/examples)
- **Official Plugins (to use as inspiration/examples)**: [K8sense Plugin Catalog](https://github.com/k8sense-k8s/plugins/)
- **Community**: [#k8sense Slack Channel](https://kubernetes.slack.com/messages/k8sense)
- **Issues**: [GitHub Issues](https://github.com/kubernetes-sigs/k8sense/issues)
