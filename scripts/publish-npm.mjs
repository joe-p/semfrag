#!/usr/bin/env node
// Builds and publishes the semfrag npm packages: one per platform plus the
// wrapper that selects between them. Platform packages are published first so
// the wrapper, which is what users install, only appears once every binary it
// points at is available.

import { spawnSync } from "node:child_process";
import { chmodSync, cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const version = (process.env.VERSION || readFileSync(join(root, "VERSION"), "utf8")).trim();
const outdir = resolve(root, process.env.OUTDIR || "build/binaries");
const npmdir = resolve(root, process.env.NPMDIR || "build/npm");
const tag = process.env.NPM_TAG || "latest";
const dryRun = process.env.DRY_RUN === "true";

const repository = "https://github.com/joe-p/semfrag";
const description = "Semantic versioning and changelog generation based on markdown fragments";
// Platform packages live under the @semfrag org; the wrapper stays unscoped so
// `npm install -g semfrag` keeps working.
const scope = "@semfrag";

const targets = [
  { os: "linux", cpu: "x64", asset: "semfrag-linux-x64" },
  { os: "linux", cpu: "arm64", asset: "semfrag-linux-arm64" },
  { os: "darwin", cpu: "x64", asset: "semfrag-darwin-x64" },
  { os: "darwin", cpu: "arm64", asset: "semfrag-darwin-arm64" },
  { os: "win32", cpu: "x64", asset: "semfrag-windows-x64.exe" },
  { os: "win32", cpu: "arm64", asset: "semfrag-windows-arm64.exe" },
];

function run(cmd, args, opts = {}) {
  const result = spawnSync(cmd, args, { stdio: "inherit", ...opts });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`${cmd} ${args.join(" ")} exited with status ${result.status}`);
  }
}

function alreadyPublished(name) {
  const result = spawnSync("npm", ["view", `${name}@${version}`, "version"], {
    encoding: "utf8",
  });
  return result.status === 0 && result.stdout.trim() === version;
}

// Auth comes from npm trusted publishing (OIDC); provenance is generated
// automatically for publishes made this way, so no token or flag is needed.
function publish(dir, name) {
  const args = ["publish", `--tag=${tag}`];
  if (name.startsWith("@")) args.push("--access", "public");
  if (dryRun) args.push("--dry-run");
  run("npm", args, { cwd: dir });
}

function writeManifest(dir, manifest) {
  writeFileSync(join(dir, "package.json"), JSON.stringify(manifest, null, 2) + "\n");
}

function writePlatformPackage(target) {
  const name = `${scope}/${target.os}-${target.cpu}`;
  const dir = join(npmdir, `${target.os}-${target.cpu}`);
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(join(dir, "bin"), { recursive: true });

  const exe = target.os === "win32" ? "semfrag.exe" : "semfrag";
  const destination = join(dir, "bin", exe);
  cpSync(join(outdir, target.asset), destination);
  chmodSync(destination, 0o755);

  writeManifest(dir, {
    name,
    version,
    description: `${description} (${target.os}-${target.cpu} binary)`,
    os: [target.os],
    cpu: [target.cpu],
    files: ["bin"],
    license: "MIT",
    homepage: `${repository}#readme`,
    repository: { type: "git", url: `git+${repository}.git` },
  });

  return { name, dir };
}

function writeWrapper(platformNames) {
  const dir = join(npmdir, "semfrag");
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(join(dir, "bin"), { recursive: true });

  const shim = join(dir, "bin", "semfrag.js");
  cpSync(join(root, "scripts", "npm", "semfrag.js"), shim);
  chmodSync(shim, 0o755);

  const optionalDependencies = {};
  for (const name of platformNames) optionalDependencies[name] = version;

  writeManifest(dir, {
    name: "semfrag",
    version,
    description,
    bin: { semfrag: "bin/semfrag.js" },
    files: ["bin"],
    optionalDependencies,
    engines: { node: ">=18" },
    license: "MIT",
    homepage: `${repository}#readme`,
    repository: { type: "git", url: `git+${repository}.git` },
    keywords: ["changelog", "changelog.d", "release", "semver", "semantic-versioning", "cli"],
  });

  return dir;
}

mkdirSync(npmdir, { recursive: true });

const platforms = targets.map(writePlatformPackage);
for (const platform of platforms) {
  if (alreadyPublished(platform.name)) {
    console.log(`${platform.name}@${version} is already published; skipping.`);
    continue;
  }
  console.log(`Publishing ${platform.name}@${version} (tag: ${tag})...`);
  publish(platform.dir, platform.name);
}

const wrapperDir = writeWrapper(platforms.map((platform) => platform.name));
if (alreadyPublished("semfrag")) {
  console.log(`semfrag@${version} is already published; skipping.`);
} else {
  console.log(`Publishing semfrag@${version} (tag: ${tag})...`);
  publish(wrapperDir, "semfrag");
}
