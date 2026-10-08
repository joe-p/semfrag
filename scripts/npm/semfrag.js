#!/usr/bin/env node
"use strict";

// Wrapper entry point for the `semfrag` npm package. The actual executable is
// shipped in a per-platform package installed as an optional dependency; this
// shim finds the one that matches the current platform and runs it.

const { spawnSync } = require("node:child_process");
const { dirname, join } = require("node:path");

function binaryPath() {
  const name = `@semfrag/${process.platform}-${process.arch}`;
  let manifest;
  try {
    manifest = require.resolve(`${name}/package.json`);
  } catch {
    process.stderr.write(
      `semfrag: no prebuilt binary for ${process.platform}-${process.arch}.\n` +
        `The optional dependency ${name} was not installed. Reinstall with ` +
        `optional dependencies enabled (do not pass --omit=optional).\n`
    );
    process.exit(1);
  }
  const exe = process.platform === "win32" ? "semfrag.exe" : "semfrag";
  return join(dirname(manifest), "bin", exe);
}

const result = spawnSync(binaryPath(), process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  process.stderr.write(`semfrag: failed to run binary: ${result.error.message}\n`);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
