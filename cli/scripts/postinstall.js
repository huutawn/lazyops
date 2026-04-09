#!/usr/bin/env node

const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const rootDir = path.resolve(__dirname, "..");
const vendorDir = path.join(rootDir, "vendor");
const isWindows = process.platform === "win32";
const outputFile = path.join(vendorDir, isWindows ? "lazyops.exe" : "lazyops");

if (process.env.LAZYOPS_SKIP_POSTINSTALL === "1") {
  process.exit(0);
}

fs.mkdirSync(vendorDir, { recursive: true });

const checkGo = spawnSync("go", ["version"], {
  cwd: rootDir,
  stdio: "ignore",
});

if (checkGo.status !== 0) {
  console.error(
    "Go is required to build lazyops CLI during npm install. Install Go >= 1.22 and reinstall."
  );
  process.exit(1);
}

const build = spawnSync("go", ["build", "-o", outputFile, "./cmd/lazyops"], {
  cwd: rootDir,
  stdio: "inherit",
});

if (build.status !== 0) {
  console.error("Failed to build lazyops CLI binary.");
  process.exit(build.status ?? 1);
}

if (!isWindows) {
  fs.chmodSync(outputFile, 0o755);
}

console.log(`lazyops binary built: ${outputFile}`);

