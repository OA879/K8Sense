'use strict';

const path = require('node:path');
const { execSync } = require('node:child_process');

// After electron-builder's signing step, force a consistent ad-hoc, deep
// signature on macOS. Without a real Developer ID, an unsigned build otherwise
// ships with mismatched signatures between the main binary and the nested
// Electron Framework, which crashes on launch with a "Library not loaded …
// different Team IDs" DYLD error on Apple Silicon / recent macOS. Re-signing
// the whole bundle with "-" (ad-hoc) makes every nested Mach-O share one
// identity, so the app launches once the user removes the download quarantine.
exports.default = async context => {
  if (context.electronPlatformName !== 'darwin') {
    return;
  }

  const appName = context.packager.appInfo.productFilename;
  const appPath = path.join(context.appOutDir, `${appName}.app`);

  console.info(`after-sign: ad-hoc deep-signing ${appPath}`);

  try {
    execSync(`codesign --force --deep --sign - "${appPath}"`, { stdio: 'inherit' });
  } catch (err) {
    console.error('after-sign: ad-hoc codesign failed:', err);
    throw err;
  }
};
