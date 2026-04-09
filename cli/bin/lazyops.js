#!/usr/bin/env node

const { spawn } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const isWindows = process.platform === "win32";
const binaryName = isWindows ? "lazyops.exe" : "lazyops";
const binaryPath = path.join(__dirname, "..", "vendor", binaryName);

if (!fs.existsSync(binaryPath)) {
  console.error(
    "lazyops binary is missing. Reinstall package or run `npm rebuild -g @tawn/lazyops-cli`."
  );
  process.exit(1);
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});

