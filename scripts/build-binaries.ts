#!/usr/bin/env bun
import { mkdir, rm } from "node:fs/promises";
import path from "node:path";
import pkg from "../package.json" with { type: "json" };

// Every target `bun build --compile` supports. The release workflow builds
// them all from a single Linux runner and attaches each to the GitHub release.
const TARGETS = [
  "bun-darwin-arm64",
  "bun-darwin-x64",
  "bun-linux-arm64",
  "bun-linux-arm64-musl",
  "bun-linux-x64",
  "bun-linux-x64-musl",
  "bun-windows-arm64",
  "bun-windows-x64",
] as const;

const OUTDIR = process.env.OUTDIR ?? "build/binaries";
const VERSION = process.env.VERSION ?? pkg.version;

function artifactName(target: string): string {
  return `semfrag-${target.replace(/^bun-/, "")}${target.startsWith("bun-windows") ? ".exe" : ""}`;
}

await rm(OUTDIR, { recursive: true, force: true });
await mkdir(OUTDIR, { recursive: true });

const requested = process.env.TARGET ? process.env.TARGET.split(",") : TARGETS;
for (const target of requested) {
  const outfile = path.join(OUTDIR, artifactName(target));
  console.log(`Building ${target} -> ${outfile}`);
  const result = await Bun.build({
    entrypoints: ["src/cli.ts"],
    compile: {
      target: target as Bun.Build.CompileTarget,
      outfile,
      autoloadDotenv: false,
      autoloadBunfig: false,
      autoloadPackageJson: false,
      autoloadTsconfig: false,
    },
    minify: true,
    define: {
      SEMFRAG_VERSION: JSON.stringify(VERSION),
    },
  });
  if (!result.success) {
    for (const log of result.logs) console.error(log);
    throw new Error(`Build failed for ${target}`);
  }
}

console.log(`Built ${requested.length} binaries for v${VERSION} in ${OUTDIR}`);
