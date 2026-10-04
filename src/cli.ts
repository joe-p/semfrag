#!/usr/bin/env node
import { parseArgs } from "node:util";
import { readFileSync } from "node:fs";
import {
  DEFAULT_INITIAL_VERSION,
  generate,
  init,
  isPromoteChannel,
  latest,
  notes,
  promote,
  PROMOTE_CHANNELS,
  type PromoteChannel,
} from "./changelog.ts";
import {
  DEFAULT_CONFIG_FILE,
  loadConfig,
  sectionBumps,
  sectionOrder,
  sectionTypes,
} from "./config.ts";

// Replaced at build time by `bun build --define` for standalone binaries.
declare const SEMFRAG_VERSION: string | undefined;

function readVersion(): string {
  if (typeof SEMFRAG_VERSION === "string") return SEMFRAG_VERSION;
  try {
    const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
    return pkg.version ?? "0.0.0";
  } catch {
    return "0.0.0";
  }
}

const HELP = `semfrag - merge changelog.d fragments into a changelog

Usage:
  semfrag [generate] [options]
  semfrag promote <channel> [options]
  semfrag latest [options]
  semfrag notes [options]
  semfrag init [options]

Commands:
  generate              Merge pending fragments into an unreleased section and
                        prepend it to the changelog. This is the default command.
  promote <channel>     Promote the top section to a release. <channel> is one of
                        stable, alpha, beta or rc. "stable" finalizes the top
                        unreleased or prerelease section, merging same-version
                        prereleases. The other channels tag the top section as a
                        prerelease. Fails if fragments are pending.
  latest                Print the latest released version from the changelog.
  notes                 Print the changelog body of the latest release, without
                        the version heading. Useful for release notes.
  init                  Create an empty changelog with an unreleased heading, the
                        fragments directory and a config file. Fails if the
                        changelog exists. A 0.y.z start makes "Breaking Changes"
                        bump MINOR.

Options:
  -d, --dir <path>      Directory containing changelog fragments (default: changelog.d)
  -o, --output <path>   Changelog file, or "-" for stdout (default: CHANGELOG.md)
      --input <path>    Existing changelog to read (default: output, or CHANGELOG.md for stdout)
  -c, --config <path>   Config file to read or, for init, write (default: ${DEFAULT_CONFIG_FILE})
      --initial <ver>   init only: starting version, e.g. 0.1.0 or 1.0.0 (default: ${DEFAULT_INITIAL_VERSION})
      --dry-run         Print the result without writing or clearing
      --no-clear        Keep the fragment files after generating
  -h, --help            Show this help
  -v, --version         Show the version

Versions are read from the changelog itself. The next version is the highest
bump level among the pending sections applied to the last released version. While
the top section is "1.0.0 - UNRELEASED" and 1.0.0 has not been released, the
version stays 1.0.0 regardless of bump level.

Promotion:
  Prereleases move forward along the alpha -> beta -> rc -> stable ladder.
  "promote alpha" requires an unreleased top section, "promote beta" accepts an
  unreleased or alpha top, "promote rc" accepts an unreleased, alpha or beta top,
  and "promote stable" accepts any of them. For example, "promote alpha" renames
  the top "1.0.1 - UNRELEASED" section to "1.0.1-alpha.1 - January 1st, 2026".
  Repeating it for the same version increments the number; a different channel
  restarts at .1. New fragments still generate a plain "1.0.1 - UNRELEASED" on
  top. "promote stable" then merges the unreleased and all "1.0.1-*" prerelease
  sections into "1.0.1 - January 1st, 2026" and removes them.

Config file:
  A JSON object with a "sections" array listing the allowed sections in the order
  they should appear. Each section has a "title", an optional semantic version
  "bump" level (MAJOR, MINOR or PATCH) and an optional "type" of "list" (the
  default) or "raw", e.g.

    {
      "sections": [
        { "title": "Breaking Changes", "bump": "MAJOR" },
        { "title": "Features", "bump": "MINOR" },
        { "title": "Fixes", "bump": "PATCH" },
        { "title": "Upgrade Guide", "type": "raw" }
      ]
    }

  A "list" section preserves multiline items and drops duplicate items. A "raw"
  section preserves its markdown verbatim, so it may contain nested markdown
  such as blank lines, code blocks and "###" headings. It may not contain "#" or
  "##" headings outside fenced code. Sections found in fragments that are not
  listed cause an error.
`;

function fail(message: string): never {
  process.stderr.write(`semfrag: ${message}\n`);
  process.exit(1);
}

async function main(): Promise<void> {
  let parsed;
  try {
    parsed = parseArgs({
      options: {
        dir: { type: "string", short: "d" },
        output: { type: "string", short: "o" },
        input: { type: "string" },
        config: { type: "string", short: "c" },
        initial: { type: "string" },
        "dry-run": { type: "boolean" },
        "no-clear": { type: "boolean" },
        help: { type: "boolean", short: "h" },
        version: { type: "boolean", short: "v" },
      },
      allowPositionals: true,
    });
  } catch (error) {
    fail(error instanceof Error ? error.message : String(error));
  }

  const { values, positionals } = parsed;

  if (values.help) {
    process.stdout.write(HELP);
    return;
  }
  if (values.version) {
    process.stdout.write(`${readVersion()}\n`);
    return;
  }

  const command = positionals[0] ?? "generate";
  if (
    command !== "generate" &&
    command !== "promote" &&
    command !== "latest" &&
    command !== "notes" &&
    command !== "init"
  ) {
    fail(`unknown command: ${command}`);
  }

  let channel: PromoteChannel | undefined;
  if (command === "promote") {
    const requested = positionals[1];
    if (requested === undefined) {
      fail(`promote requires a channel: ${PROMOTE_CHANNELS.join(", ")}`);
    }
    if (positionals.length > 2) {
      fail(`unexpected argument: ${positionals[2]}`);
    }
    if (!isPromoteChannel(requested)) {
      fail(`unknown channel: ${requested}. Expected one of: ${PROMOTE_CHANNELS.join(", ")}`);
    }
    channel = requested;
  } else if (positionals.length > 1) {
    fail(`unexpected argument: ${positionals[1]}`);
  }

  const dir = values.dir ?? "changelog.d";
  const output = values.output ?? "CHANGELOG.md";
  const dryRun = values["dry-run"] ?? false;

  if (command !== "generate" && values.input !== undefined) {
    fail("--input can only be used with generate");
  }
  if (command !== "init" && values.initial !== undefined) {
    fail("--initial can only be used with init");
  }

  if (command === "init") {
    if (output === "-") {
      fail("init cannot write to stdout");
    }
    const result = await init({
      output,
      dir,
      version: values.initial,
      config: values.config,
      dryRun,
    });
    const configNote = result.configWritten
      ? dryRun
        ? ` and write ${result.config}`
        : ` and wrote ${result.config}`
      : "";
    if (dryRun) {
      process.stdout.write(`Would initialize ${output} at ${result.title}${configNote}.\n`);
      return;
    }
    process.stdout.write(`Initialized ${output} at ${result.title}${configNote}.\n`);
    return;
  }

  if (command === "latest") {
    const result = await latest({ output });
    process.stdout.write(`${result.version}\n`);
    return;
  }

  if (command === "notes") {
    const result = await notes({ output });
    process.stdout.write(`${result.notes}\n`);
    return;
  }

  const config = await loadConfig(values.config);

  if (command === "promote") {
    const result = await promote({
      output,
      dir,
      dryRun,
      channel: channel!,
      order: config ? sectionOrder(config) : undefined,
      types: config ? sectionTypes(config) : undefined,
    });
    if (dryRun) {
      process.stdout.write(`Would promote to ${result.version} in ${output}.\n`);
      return;
    }
    process.stdout.write(`Promoted to ${result.version} in ${output}.\n`);
    return;
  }

  const result = await generate({
    dir,
    output,
    input: values.input,
    clear: !values["no-clear"],
    dryRun,
    order: config ? sectionOrder(config) : undefined,
    bump: config ? sectionBumps(config) : undefined,
    types: config ? sectionTypes(config) : undefined,
  });

  if (result.entry === "") {
    process.stdout.write("No changelog fragments found.\n");
    return;
  }

  if (dryRun || values.output === "-") {
    process.stdout.write(result.entry);
    return;
  }

  const from = result.previous ?? "initial";
  const bump = result.level && result.previous ? ` (${result.level})` : "";
  process.stderr.write(`semfrag: ${from} -> ${result.version}${bump}\n`);

  process.stdout.write(
    `Generated ${output} from ${result.fragments.length} fragment(s)` +
      (result.cleared.length > 0 ? ` and cleared ${result.cleared.length} file(s).\n` : ".\n"),
  );
}

main().catch((error: unknown) => {
  fail(error instanceof Error ? error.message : String(error));
});
