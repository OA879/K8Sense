#!/bin/bash -e

REPO_ROOT=$(git rev-parse --show-toplevel)
PLUGINS_TO_UPDATE=$1
K8SENSE_PLUGIN="@kinvolk/headlamp-plugin"

if [ -z "$PLUGINS_TO_UPDATE" ]; then
  PLUGINS_TO_UPDATE=$(ls $REPO_ROOT/plugins/examples)
fi

# Get the latest version of @kinvolk/headlamp-plugin
LATEST_VERSION=$(npm view $K8SENSE_PLUGIN version)
echo "Latest version of $K8SENSE_PLUGIN is $LATEST_VERSION"

# Check if tree is dirty
if ! git diff --quiet; then
  echo "You have pending changes, please commit or stash your changes before running this script."
  exit 1
fi

for plugin in $PLUGINS_TO_UPDATE; do
  # Check if the plugin exists
  if [ ! -f "$REPO_ROOT/plugins/examples/$plugin/package.json" ]; then
    continue
  fi
  pushd $REPO_ROOT/plugins/examples/$plugin > /dev/null

  echo "Updating $plugin"
  npm audit fix || true

  if ! git diff --quiet; then
    git add ./package.json ./package-lock.json
    git commit -s -m "plugins/examples/$plugin: Update dependencies with npm audit fix"
    echo "Updated dependencies in $plugin"
  else
    echo "No updates found for $plugin"
  fi

  # Update @kinvolk/headlamp-plugin
  if grep -q "$K8SENSE_PLUGIN" package.json; then
    echo "Updating $K8SENSE_PLUGIN in $plugin"

    # Get the current version to check if update is needed
    CURRENT_VERSION=$(grep -o "\"$K8SENSE_PLUGIN\": \"[^\"]*\"" package.json | cut -d'"' -f4)
    echo "Current version: $CURRENT_VERSION, updating to: $LATEST_VERSION"

    # Use a more robust sed pattern that accounts for potential spacing variations
    sed -i -E "s|(\"$K8SENSE_PLUGIN\"[[:space:]]*:[[:space:]]*\")[^\"]*(\")|\\1^$LATEST_VERSION\\2|" package.json

    if ! git diff --quiet; then
      npm ci
      git add -u .
      git commit -s -m "plugins/examples/$plugin: Update $K8SENSE_PLUGIN to version $LATEST_VERSION"
      echo "Updated $K8SENSE_PLUGIN to version $LATEST_VERSION in $plugin"
    else
      echo "Already using latest version of $K8SENSE_PLUGIN"
    fi
  else
    echo "$K8SENSE_PLUGIN not found in $plugin"
  fi

  popd > /dev/null
done
