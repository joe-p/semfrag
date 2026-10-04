import { mkdir, readdir, readFile, writeFile, rm, rename, stat, realpath } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import { existsSync } from "node:fs";
import path from "node:path";
import {
  mergeFragments,
  mergeSections,
  parseChangelogDocument,
  prependChangelog,
  renderChangelog,
  UNRELEASED_MARKER,
  type Fragment,
  type Section,
  type SectionTypes,
  type VersionBlock,
} from "./generate.ts";
import {
  baseVersion,
  highestBase,
  highestBump,
  isPrerelease,
  nextPrerelease,
  nextVersion,
  parseVersion,
  prereleaseOf,
  type BumpLevel,
} from "./bump.ts";
import { DEFAULT_CONFIG_FILE, defaultConfig, serializeConfig } from "./config.ts";
import { formatReleaseDate } from "./date.ts";

export interface GenerateOptions {
  dir: string;
  output: string;
  input?: string;
  clear: boolean;
  dryRun: boolean;
  order?: string[];
  bump?: Record<string, BumpLevel>;
  types?: SectionTypes;
}

export interface GenerateResult {
  entry: string;
  version: string;
  title: string;
  level: BumpLevel | undefined;
  previous: string | undefined;
  fragments: string[];
  written: boolean;
  cleared: string[];
}

export const PROMOTE_CHANNELS = ["stable", "alpha", "beta", "rc"] as const;

export type PromoteChannel = (typeof PROMOTE_CHANNELS)[number];

export function isPromoteChannel(value: unknown): value is PromoteChannel {
  return typeof value === "string" && (PROMOTE_CHANNELS as readonly string[]).includes(value);
}

// Which release states may be promoted to each target channel. A top section
// that is "unreleased" can start any channel; a prerelease can only move
// forward along the alpha -> beta -> rc -> stable ladder.
const PROMOTE_SOURCES: Record<PromoteChannel, readonly string[]> = {
  stable: ["unreleased", "alpha", "beta", "rc"],
  alpha: ["unreleased"],
  beta: ["unreleased", "alpha"],
  rc: ["unreleased", "alpha", "beta"],
};

export interface PromoteOptions {
  output: string;
  dir?: string;
  channel: PromoteChannel;
  dryRun: boolean;
  order?: string[];
  types?: SectionTypes;
  now?: Date;
}

export interface PromoteResult {
  version: string;
  written: boolean;
}

export interface LatestOptions {
  output: string;
}

export interface LatestResult {
  version: string;
}

export interface NotesOptions {
  output: string;
}

export interface NotesResult {
  version: string;
  notes: string;
}

export interface InitOptions {
  output: string;
  dir: string;
  version?: string;
  config?: string;
  dryRun: boolean;
}

export interface InitResult {
  version: string;
  title: string;
  output: string;
  dir: string;
  config: string;
  configWritten: boolean;
  written: boolean;
}

export const DEFAULT_INITIAL_VERSION = "1.0.0";

async function listFragments(dir: string): Promise<string[]> {
  if (!existsSync(dir)) return [];

  const entries = await readdir(dir, { withFileTypes: true });
  return entries
    .filter((entry) => entry.isFile() && !entry.name.startsWith(".") && entry.name.endsWith(".md"))
    .map((entry) => entry.name)
    .sort((a, b) => a.localeCompare(b));
}

async function atomicWrite(file: string, content: string): Promise<void> {
  // Resolve symlinks through the existing destination rather than replacing them.
  const destination = existsSync(file) ? await realpath(file) : path.resolve(file);
  const temporary = path.join(
    path.dirname(destination),
    `.${path.basename(destination)}.${randomUUID()}.tmp`,
  );
  try {
    const mode = existsSync(destination) ? (await stat(destination)).mode : undefined;
    await writeFile(temporary, content, { flag: "wx", mode });
    await rename(temporary, destination);
  } finally {
    await rm(temporary, { force: true });
  }
}

function withPreamble(preamble: string, content: string): string {
  if (preamble === "") return content;
  const separator = preamble.endsWith("\n\n") ? "" : preamble.endsWith("\n") ? "\n" : "\n\n";
  return `${preamble}${separator}${content}`;
}

export async function readFragments(dir: string): Promise<Fragment[]> {
  const names = await listFragments(dir);
  return Promise.all(
    names.map(async (name) => ({
      name,
      content: await readFile(path.join(dir, name), "utf8"),
    })),
  );
}

export async function init(options: InitOptions): Promise<InitResult> {
  const requested = options.version ?? DEFAULT_INITIAL_VERSION;
  const parsed = parseVersion(requested);
  if (parsed.prerelease !== undefined || parsed.build !== undefined) {
    throw new Error(
      `Invalid initial version: "${requested}". Expected a plain MAJOR.MINOR.PATCH version.`,
    );
  }
  if (existsSync(options.output)) {
    throw new Error(`${options.output} already exists. Remove it first or choose another output.`);
  }

  const version = baseVersion(requested);
  const title = `${version} - ${UNRELEASED_MARKER}`;
  const entry = renderChangelog([], title);
  const configPath = options.config ?? DEFAULT_CONFIG_FILE;
  const configWritten = !existsSync(configPath);

  if (!options.dryRun) {
    await mkdir(options.dir, { recursive: true });
    await atomicWrite(options.output, entry);
    if (configWritten) {
      await atomicWrite(configPath, serializeConfig(defaultConfig(version)));
    }
  }

  return {
    version,
    title,
    output: options.output,
    dir: options.dir,
    config: configPath,
    configWritten,
    written: !options.dryRun,
  };
}

export function selectBump(
  sections: Section[],
  bump: Record<string, BumpLevel>,
): BumpLevel | undefined {
  return highestBump(
    sections
      .map((section) => bump[section.title])
      .filter((level): level is BumpLevel => level !== undefined),
  );
}

function resolveNextVersion(
  blocks: VersionBlock[],
  sections: Section[],
  bump: Record<string, BumpLevel>,
): { version: string; level: BumpLevel | undefined; previous: string | undefined } {
  const level = selectBump(sections, bump);
  const lastReleased = blocks.find(
    (block) => !block.unreleased && !isPrerelease(block.version),
  )?.version;
  const prereleaseBases = blocks
    .filter((block) => isPrerelease(block.version))
    .map((block) => baseVersion(block.version));

  if (lastReleased === undefined) {
    // Until the initial version is released it is fixed, so a v0.y.z or 1.0.0
    // first release is never bumped past what was initialized.
    const base = blocks.find((block) => block.unreleased)?.version ?? DEFAULT_INITIAL_VERSION;
    return { version: highestBase(base, ...prereleaseBases), level, previous: undefined };
  }

  const bumped = nextVersion(lastReleased, level);
  return {
    version: highestBase(bumped, ...prereleaseBases),
    level,
    previous: lastReleased,
  };
}

export async function generate(options: GenerateOptions): Promise<GenerateResult> {
  const fragments = await readFragments(options.dir);
  const names = fragments.map((fragment) => fragment.name);
  const toStdout = options.output === "-";
  const input = options.input ?? (toStdout ? "CHANGELOG.md" : options.output);
  const existing = existsSync(input) ? await readFile(input, "utf8") : "";

  const { blocks, preamble } = parseChangelogDocument(existing, options.types);
  if (blocks.slice(1).some((block) => block.unreleased)) {
    throw new Error("An UNRELEASED section must appear only at the top of the changelog.");
  }
  const existingUnreleased = blocks[0]?.unreleased ? blocks[0].sections : [];
  const sections = mergeSections(
    [existingUnreleased, mergeFragments(fragments, options.order, options.types)],
    options.order,
    options.types,
  );

  if (sections.length === 0) {
    return {
      entry: "",
      version: "",
      title: "",
      level: undefined,
      previous: blocks.find((block) => !block.unreleased)?.version,
      fragments: names,
      written: false,
      cleared: [],
    };
  }

  const { version, level, previous } = resolveNextVersion(blocks, sections, options.bump ?? {});
  const title = `${version} - ${UNRELEASED_MARKER}`;
  const entry = renderChangelog(sections, title);

  const remainder = blocks
    .filter((block) => !block.unreleased)
    .map((block) => block.raw)
    .join("\n\n");

  const written = !options.dryRun && !toStdout;
  if (written) {
    await atomicWrite(options.output, withPreamble(preamble, prependChangelog(remainder, entry)));
  }

  const cleared: string[] = [];
  if (options.clear && written && names.length > 0) {
    for (const fragment of fragments) {
      const file = path.join(options.dir, fragment.name);
      // Keep fragments edited since the snapshot; a later generate can consume them.
      try {
        if ((await readFile(file, "utf8")) !== fragment.content) continue;
        await rm(file);
        cleared.push(fragment.name);
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
      }
    }
  }

  return { entry, version, title, level, previous, fragments: names, written, cleared };
}

function currentChannel(block: VersionBlock): string {
  if (block.unreleased) return "unreleased";
  const prerelease = prereleaseOf(block.version);
  if (prerelease === undefined) return "stable";
  const match = /^(.*)\.\d+$/.exec(prerelease);
  return match ? match[1]! : prerelease;
}

export async function promote(options: PromoteOptions): Promise<PromoteResult> {
  const existing = await readFile(options.output, "utf8");
  const { blocks, preamble } = parseChangelogDocument(existing, options.types);

  const dir = options.dir ?? "changelog.d";
  const pending = await listFragments(dir);
  if (pending.length > 0) {
    throw new Error(
      `Cannot promote: ${dir} still contains ${pending.length} pending fragment(s). Run generate first.`,
    );
  }

  const top = blocks[0];
  if (!top) {
    throw new Error(`No version to promote in ${options.output}.`);
  }

  const current = currentChannel(top);
  if (!PROMOTE_SOURCES[options.channel].includes(current)) {
    throw new Error(`Cannot promote ${current} to ${options.channel} in ${options.output}.`);
  }

  return options.channel === "stable"
    ? promoteStable(options, blocks, preamble)
    : promotePrerelease(options, blocks, options.channel, preamble);
}

function latestReleased(blocks: VersionBlock[], output: string): VersionBlock {
  const released = blocks.find((block) => !block.unreleased);
  if (!released) {
    throw new Error(`No released version found in ${output}.`);
  }
  return released;
}

export async function latest(options: LatestOptions): Promise<LatestResult> {
  const existing = await readFile(options.output, "utf8");
  const { blocks } = parseChangelogDocument(existing);
  return { version: latestReleased(blocks, options.output).version };
}

export async function notes(options: NotesOptions): Promise<NotesResult> {
  const existing = await readFile(options.output, "utf8");
  const { blocks } = parseChangelogDocument(existing);
  const released = latestReleased(blocks, options.output);
  // Drop the "# <version>" heading; the body is the release notes.
  const notes = released.raw
    .split("\n")
    .slice(1)
    .join("\n")
    .replace(/^\n+/, "")
    .replace(/\s+$/, "");
  return { version: released.version, notes };
}

async function promotePrerelease(
  options: PromoteOptions,
  blocks: VersionBlock[],
  channel: PromoteChannel,
  preamble: string,
): Promise<PromoteResult> {
  const top = blocks[0]!;
  parseVersion(top.version);
  const version = nextPrerelease(
    top.version,
    channel,
    blocks.map((block) => block.version),
  );
  const title = `${version} - ${formatReleaseDate(options.now)}`;
  const entry = renderChangelog(top.sections, title);
  const remainder = blocks
    .slice(1)
    .map((block) => block.raw)
    .join("\n\n");

  if (!options.dryRun) {
    await atomicWrite(options.output, withPreamble(preamble, prependChangelog(remainder, entry)));
  }

  return { version, written: !options.dryRun };
}

async function promoteStable(
  options: PromoteOptions,
  blocks: VersionBlock[],
  preamble: string,
): Promise<PromoteResult> {
  const top = blocks[0]!;
  const base = baseVersion(top.version);
  const consumed: VersionBlock[] = [];
  let index = 0;
  while (index < blocks.length) {
    const block = blocks[index]!;
    if (!block.unreleased && prereleaseOf(block.version) === undefined) break;
    if (baseVersion(block.version) !== base) break;
    consumed.push(block);
    index += 1;
  }

  const sections = mergeSections(
    consumed.map((block) => block.sections),
    options.order,
    options.types,
  );
  const entry = renderChangelog(sections, `${base} - ${formatReleaseDate(options.now)}`);
  const remainder = blocks
    .slice(index)
    .map((block) => block.raw)
    .join("\n\n");

  if (!options.dryRun) {
    await atomicWrite(options.output, withPreamble(preamble, prependChangelog(remainder, entry)));
  }

  return { version: base, written: !options.dryRun };
}
