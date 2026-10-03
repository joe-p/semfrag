import { test, mock } from "bun:test";
import assert from "node:assert/strict";
import fs, { mkdtemp, mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import { existsSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { generate, init, latest, notes, release } from "../src/changelog.ts";
import { readConfig, sectionBumps, sectionOrder } from "../src/config.ts";

const ORDER = ["Breaking Changes", "Fixes", "Features"];
const BUMP = { "Breaking Changes": "MAJOR", Fixes: "PATCH", Features: "MINOR" } as const;
const RELEASE_DATE = new Date(2026, 0, 1);
const RELEASED_ON = "January 1st, 2026";

async function makeRoot(): Promise<{ root: string; dir: string; output: string }> {
  const root = await mkdtemp(path.join(tmpdir(), "semfrag-"));
  const dir = path.join(root, "changelog.d");
  const output = path.join(root, "CHANGELOG.md");
  await mkdir(dir);
  return { root, dir, output };
}

async function makeInitRoot(): Promise<{
  root: string;
  dir: string;
  output: string;
  config: string;
}> {
  const root = await mkdtemp(path.join(tmpdir(), "semfrag-"));
  return {
    root,
    dir: path.join(root, "changelog.d"),
    output: path.join(root, "CHANGELOG.md"),
    config: path.join(root, "semfrag.json"),
  };
}

test("generate reads the version from the changelog and marks it unreleased", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(output, "# 1.0.0\n\n## Features\n\n- Released 1.0!\n");
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n\n- Some fix\n");

  const first = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: ORDER,
    bump: BUMP,
  });
  assert.equal(first.version, "1.0.1");
  assert.equal(first.previous, "1.0.0");
  assert.equal(first.level, "PATCH");
  assert.equal(
    await readFile(output, "utf8"),
    "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- Some fix\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n",
  );
  assert.deepEqual(await readdir(dir), []);

  await writeFile(path.join(dir, "feat.md"), "## Features\n\n- A new feature!\n");

  const second = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: ORDER,
    bump: BUMP,
  });
  assert.equal(second.version, "1.1.0");
  assert.equal(second.level, "MINOR");
  assert.equal(
    await readFile(output, "utf8"),
    "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- Some fix\n\n## Features\n\n- A new feature!\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n",
  );
});

test("generate keeps 1.0.0 while the initial release is unreleased", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n\n- Some fix\n");

  const first = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: ORDER,
    bump: BUMP,
  });
  assert.equal(first.version, "1.0.0");
  assert.equal(first.previous, undefined);
  assert.equal(await readFile(output, "utf8"), "# 1.0.0 - UNRELEASED\n\n## Fixes\n\n- Some fix\n");

  await writeFile(path.join(dir, "feat.md"), "## Features\n\n- A new feature!\n");
  const second = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: ORDER,
    bump: BUMP,
  });
  assert.equal(second.version, "1.0.0");
  assert.equal(
    await readFile(output, "utf8"),
    "# 1.0.0 - UNRELEASED\n\n## Fixes\n\n- Some fix\n\n## Features\n\n- A new feature!\n",
  );
});

test("generate is idempotent when fragments are kept", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n\n- Some fix\n");

  await generate({ dir, output, clear: false, dryRun: false, order: ORDER, bump: BUMP });
  await generate({ dir, output, clear: false, dryRun: false, order: ORDER, bump: BUMP });

  const text = await readFile(output, "utf8");
  assert.equal((text.match(/- Some fix/g) ?? []).length, 1);
});

test("generate preserves raw sections and nested markdown", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(
    path.join(dir, "details.md"),
    "## Details\n\nSome **bold** text.\n\n### Nested\n\n- item\n",
  );

  const result = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: [...ORDER, "Details"],
    bump: BUMP,
    types: { Details: "raw" },
  });

  assert.equal(result.version, "1.0.0");
  assert.equal(
    await readFile(output, "utf8"),
    "# 1.0.0 - UNRELEASED\n\n## Details\n\nSome **bold** text.\n\n### Nested\n\n- item\n",
  );
});

test("generate is idempotent for raw sections when fragments are kept", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(path.join(dir, "details.md"), "## Details\n\nLine one.\n\nLine two.\n");

  const options = {
    dir,
    output,
    clear: false,
    dryRun: false,
    order: ["Details"],
    bump: {},
    types: { Details: "raw" as const },
  };
  await generate(options);
  await generate(options);

  const text = await readFile(output, "utf8");
  assert.equal((text.match(/Line one\./g) ?? []).length, 1);
});

test("release merges raw sections from prereleases", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(
    output,
    "# 1.0.1 - UNRELEASED\n\n## Details\n\nNew details.\n\n# 1.0.1-alpha.1\n\n## Details\n\nOld details.\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n",
  );

  const result = await release({
    output,
    dir,
    dryRun: false,
    order: [...ORDER, "Details"],
    types: { Details: "raw" },
    now: RELEASE_DATE,
  });

  assert.deepEqual(result, { version: "1.0.1", written: true });
  assert.equal(
    await readFile(output, "utf8"),
    `# 1.0.1 - ${RELEASED_ON}\n\n## Details\n\nNew details.\n\nOld details.\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n`,
  );
});

test("generate does nothing without fragments or an unreleased section", async () => {
  const { dir, output } = await makeRoot();

  const result = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: ORDER,
    bump: BUMP,
  });

  assert.equal(result.written, false);
  assert.equal(result.fragments.length, 0);
  assert.equal(existsSync(output), false);
});

test("generate keeps the last released version when no section bumps", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(output, "# 1.2.3\n\n## Features\n\n- old\n");
  await writeFile(path.join(dir, "docs.md"), "## Docs\n\n- docs\n");

  const result = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: ["Docs"],
    bump: {},
  });

  assert.equal(result.version, "1.2.3");
  assert.equal(result.level, undefined);
});

test("release removes the unreleased marker", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(
    output,
    "# 1.1.0 - UNRELEASED\n\n## Features\n\n- A new feature!\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n",
  );

  const result = await release({ output, dir, dryRun: false, now: RELEASE_DATE });

  assert.deepEqual(result, { version: "1.1.0", written: true });
  assert.equal(
    await readFile(output, "utf8"),
    `# 1.1.0 - ${RELEASED_ON}\n\n## Features\n\n- A new feature!\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n`,
  );
});

test("latest returns the most recent released version", async () => {
  const { output } = await makeRoot();
  await writeFile(
    output,
    "# 1.2.0 - UNRELEASED\n\n## Features\n\n- Pending\n\n# 1.1.0\n\n## Features\n\n- Released\n\n# 1.0.0\n\n## Features\n\n- Old\n",
  );

  assert.deepEqual(await latest({ output }), { version: "1.1.0" });
});

test("latest treats a prerelease as released", async () => {
  const { output } = await makeRoot();
  await writeFile(
    output,
    "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- Pending\n\n# 1.0.1-alpha.1\n\n## Fixes\n\n- Preview\n\n# 1.0.0\n\n## Features\n\n- Old\n",
  );

  assert.deepEqual(await latest({ output }), { version: "1.0.1-alpha.1" });
});

test("latest fails without a released version", async () => {
  const { output } = await makeRoot();
  await writeFile(output, "# 1.0.0 - UNRELEASED\n\n## Features\n\n- Pending\n");

  await assert.rejects(latest({ output }), /No released version/);
});

test("notes returns the body of the latest release without the version heading", async () => {
  const { output } = await makeRoot();
  await writeFile(
    output,
    "# 1.2.0 - UNRELEASED\n\n## Features\n\n- Pending\n\n# 1.1.0\n\n## Fixes\n\n- Fixed\n\n## Features\n\n- Added\n\n# 1.0.0\n\n## Features\n\n- Old\n",
  );

  assert.deepEqual(await notes({ output }), {
    version: "1.1.0",
    notes: "## Fixes\n\n- Fixed\n\n## Features\n\n- Added",
  });
});

test("notes treats a prerelease as the latest release", async () => {
  const { output } = await makeRoot();
  await writeFile(
    output,
    "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- Pending\n\n# 1.0.1-alpha.1\n\n## Fixes\n\n- Preview\n\n# 1.0.0\n\n## Features\n\n- Old\n",
  );

  assert.deepEqual(await notes({ output }), {
    version: "1.0.1-alpha.1",
    notes: "## Fixes\n\n- Preview",
  });
});

test("notes fails without a released version", async () => {
  const { output } = await makeRoot();
  await writeFile(output, "# 1.0.0 - UNRELEASED\n\n## Features\n\n- Pending\n");

  await assert.rejects(notes({ output }), /No released version/);
});

test("CLI latest prints the released version", async () => {
  const { root, output } = await makeRoot();
  await writeFile(output, "# 1.0.0\n\n## Features\n\n- Released\n");
  const cli = fileURLToPath(new URL("../src/cli.ts", import.meta.url));
  const run = promisify(execFile);

  const result = await run(process.execPath, [cli, "latest"], { cwd: root });

  assert.equal(result.stdout, "1.0.0\n");
});

test("CLI notes prints the latest release body", async () => {
  const { root, output } = await makeRoot();
  await writeFile(output, "# 1.0.0\n\n## Features\n\n- Released\n");
  const cli = fileURLToPath(new URL("../src/cli.ts", import.meta.url));
  const run = promisify(execFile);

  const result = await run(process.execPath, [cli, "notes"], { cwd: root });

  assert.equal(result.stdout, "## Features\n\n- Released\n");
});

test("release fails without an unreleased section", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(output, "# 1.0.0\n\n## Features\n\n- Released 1.0!\n");

  await assert.rejects(release({ output, dir, dryRun: false }), /No UNRELEASED section/);
});

test("release fails while fragments are pending", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(output, "# 1.1.0 - UNRELEASED\n\n## Features\n\n- A new feature!\n");
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n\n- Some fix\n");

  await assert.rejects(release({ output, dir, dryRun: false }), /pending fragment/);
});

test("release dry run does not write", async () => {
  const { dir, output } = await makeRoot();
  const before = "# 1.1.0 - UNRELEASED\n\n## Features\n\n- A new feature!\n";
  await writeFile(output, before);

  const result = await release({ output, dir, dryRun: true });

  assert.equal(result.written, false);
  assert.equal(await readFile(output, "utf8"), before);
});

test("prerelease flow: alpha, more work, then final merge", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(
    output,
    "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- Fixed a bug\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n",
  );

  const alpha = await release({
    output,
    dir,
    dryRun: false,
    prerelease: "alpha",
    now: RELEASE_DATE,
  });
  assert.deepEqual(alpha, { version: "1.0.1", prerelease: "1.0.1-alpha.1", written: true });
  assert.equal(
    await readFile(output, "utf8"),
    `# 1.0.1-alpha.1 - ${RELEASED_ON}\n\n## Fixes\n\n- Fixed a bug\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n`,
  );

  await writeFile(path.join(dir, "fix.md"), "## Fixes\n\n- Some new fix\n");
  const generated = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: ORDER,
    bump: BUMP,
  });
  assert.equal(generated.version, "1.0.1");
  assert.equal(generated.previous, "1.0.0");
  assert.equal(
    await readFile(output, "utf8"),
    `# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- Some new fix\n\n# 1.0.1-alpha.1 - ${RELEASED_ON}\n\n## Fixes\n\n- Fixed a bug\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n`,
  );

  const final = await release({ output, dir, dryRun: false, order: ORDER, now: RELEASE_DATE });
  assert.deepEqual(final, { version: "1.0.1", written: true });
  assert.equal(
    await readFile(output, "utf8"),
    `# 1.0.1 - ${RELEASED_ON}\n\n## Fixes\n\n- Some new fix\n- Fixed a bug\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n`,
  );
});

test("prerelease release increments the same channel", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(
    output,
    "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- More\n\n# 1.0.1-alpha.1\n\n## Fixes\n\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n",
  );

  const result = await release({
    output,
    dir,
    dryRun: false,
    prerelease: "alpha",
    now: RELEASE_DATE,
  });
  assert.equal(result.prerelease, "1.0.1-alpha.2");
  assert.match(await readFile(output, "utf8"), /^# 1\.0\.1-alpha\.2 - January 1st, 2026\n/);
});

test("prerelease release resets the number when switching channel", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(
    output,
    "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- More\n\n# 1.0.1-alpha.2\n\n## Fixes\n\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n",
  );

  const result = await release({ output, dir, dryRun: false, prerelease: "beta" });
  assert.equal(result.prerelease, "1.0.1-beta.1");
});

test("final release promotes a top prerelease without an unreleased section", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(
    output,
    "# 1.0.1-alpha.2\n\n## Fixes\n\n- More\n\n# 1.0.1-alpha.1\n\n## Fixes\n\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n",
  );

  const result = await release({ output, dir, dryRun: false, now: RELEASE_DATE });

  assert.deepEqual(result, { version: "1.0.1", written: true });
  assert.equal(
    await readFile(output, "utf8"),
    `# 1.0.1 - ${RELEASED_ON}\n\n## Fixes\n\n- More\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n`,
  );
});

test("prerelease release fails without an unreleased section", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(output, "# 1.0.0\n\n## Features\n\n- Released 1.0!\n");

  await assert.rejects(
    release({ output, dir, dryRun: false, prerelease: "alpha" }),
    /No UNRELEASED section/,
  );
});

test("generate and both release modes preserve the changelog preamble", async () => {
  const { dir, output } = await makeRoot();
  const preamble = "# Changelog\n\nRelease notes for this project.\n\n";
  await writeFile(output, `${preamble}# 1.0.0\n\n## Fixes\n\n- Old fix\n`);
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n\n- New fix\n");
  await generate({ dir, output, clear: true, dryRun: false, order: ORDER, bump: BUMP });
  assert.ok((await readFile(output, "utf8")).startsWith(`${preamble}# 1.0.1 - UNRELEASED\n`));
  await release({ dir, output, dryRun: false, prerelease: "preview.test", now: RELEASE_DATE });
  assert.ok(
    (await readFile(output, "utf8")).startsWith(
      `${preamble}# 1.0.1-preview.test.1 - ${RELEASED_ON}\n`,
    ),
  );
  await release({ dir, output, dryRun: false, now: RELEASE_DATE });
  assert.ok((await readFile(output, "utf8")).startsWith(`${preamble}# 1.0.1 - ${RELEASED_ON}\n`));
});

test("generate preserves a preamble when creating the first version", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(output, "# Changelog\n\nProject notes.\n");
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n- fix\n");
  await generate({ dir, output, clear: true, dryRun: false });
  assert.ok(
    (await readFile(output, "utf8")).startsWith(
      "# Changelog\n\nProject notes.\n\n# 1.0.0 - UNRELEASED",
    ),
  );
});

test("invalid fragments leave the changelog and all fragments untouched", async () => {
  const { dir, output } = await makeRoot();
  const original = "# 1.0.0\n\n## Fixes\n- Old fix\n";
  await writeFile(output, original);
  await writeFile(path.join(dir, "good.md"), "## Fixes\n- Good fix\n");
  await writeFile(path.join(dir, "bad.md"), "Forgot the heading\n");
  await assert.rejects(generate({ dir, output, clear: true, dryRun: false }), /bad\.md: Line 1/);
  assert.equal(await readFile(output, "utf8"), original);
  assert.deepEqual((await readdir(dir)).sort(), ["bad.md", "good.md"]);
});

test("invalid prerelease channels leave the changelog untouched", async () => {
  const { dir, output } = await makeRoot();
  const original = "# 1.0.1 - UNRELEASED\n\n## Fixes\n- fix\n";
  await writeFile(output, original);
  for (const prerelease of ["", "alpha+build", "alpha..test"]) {
    await assert.rejects(release({ dir, output, dryRun: false, prerelease }), /Invalid/);
    assert.equal(await readFile(output, "utf8"), original);
  }
});

test("invalid version headings and misplaced unreleased sections cannot discard history", async () => {
  const { dir, output } = await makeRoot();
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n- new\n");
  for (const original of [
    "# 01.2.3\n\n## Fixes\n- old\n",
    "# 1.2.3\n\n## Fixes\n- old\n\n# 1.2.4 - UNRELEASED\n\n## Fixes\n- pending\n",
  ]) {
    await writeFile(output, original);
    await assert.rejects(
      generate({ dir, output, clear: true, dryRun: false }),
      /Invalid semantic version|only at the top/,
    );
    assert.equal(await readFile(output, "utf8"), original);
    assert.deepEqual(await readdir(dir), ["fix.md"]);
  }
});

test("stdout generation reads the configured input without writing or clearing", async () => {
  const { dir, output } = await makeRoot();
  const original = "# 2.3.4\n\n## Fixes\n- old\n";
  await writeFile(output, original);
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n- new\n");
  const result = await generate({
    dir,
    output: "-",
    input: output,
    clear: true,
    dryRun: false,
    bump: BUMP,
  });
  assert.equal(result.version, "2.3.5");
  assert.equal(result.written, false);
  assert.deepEqual(result.cleared, []);
  assert.equal(await readFile(output, "utf8"), original);
  assert.deepEqual(await readdir(dir), ["fix.md"]);
});

test("fragment edits and additions during writing are kept for the next generation", async () => {
  const { dir, output } = await makeRoot();
  const file = path.join(dir, "fix.md");
  await writeFile(file, "## Fixes\n- original\n");
  const rename = fs.rename;
  let onRename: (() => Promise<void>) | undefined;
  mock.module("node:fs/promises", () => ({
    ...fs,
    rename: async (...args: Parameters<typeof rename>) => {
      await rename(...args);
      await onRename?.();
    },
  }));
  onRename = async () => {
    await writeFile(file, "## Fixes\n- edited\n");
    await writeFile(path.join(dir, "new.md"), "## Fixes\n- added\n");
  };
  const result = await generate({ dir, output, clear: true, dryRun: false });
  onRename = undefined;
  assert.deepEqual(result.fragments, ["fix.md"]);
  assert.deepEqual(result.cleared, []);
  assert.match(await readFile(output, "utf8"), /- original/);
  assert.deepEqual((await readdir(dir)).sort(), ["fix.md", "new.md"]);
  assert.equal(await readFile(file, "utf8"), "## Fixes\n- edited\n");
});

test("failed atomic replacement leaves the original and fragments intact", async () => {
  const { root, dir, output } = await makeRoot();
  const original = "# 1.0.0\n\n## Fixes\n- old\n";
  await writeFile(output, original);
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n- new\n");
  let failRename = true;
  const rename = fs.rename;
  mock.module("node:fs/promises", () => ({
    ...fs,
    rename: async (...args: Parameters<typeof rename>) => {
      if (failRename) throw new Error("Simulated rename failure");
      await rename(...args);
    },
  }));
  await assert.rejects(
    generate({ dir, output, clear: true, dryRun: false }),
    /Simulated rename failure/,
  );
  failRename = false;
  assert.equal(await readFile(output, "utf8"), original);
  assert.deepEqual(await readdir(dir), ["fix.md"]);
  assert.deepEqual((await readdir(root)).sort(), ["CHANGELOG.md", "changelog.d"]);
});

test("CLI stdout uses the default changelog and previews existing unreleased content", async () => {
  const { root, dir, output } = await makeRoot();
  await writeFile(output, "# 2.3.4\n\n## Fixes\n- old\n");
  await writeFile(
    path.join(root, "semfrag.json"),
    JSON.stringify({ sections: [{ title: "Fixes", bump: "PATCH" }] }),
  );
  await writeFile(path.join(dir, "fix.md"), "## Fixes\n- new\n");
  const cli = fileURLToPath(new URL("../src/cli.ts", import.meta.url));
  const run = promisify(execFile);
  const preview = await run(process.execPath, [cli, "-o", "-"], { cwd: root });
  assert.match(preview.stdout, /^# 2\.3\.5 - UNRELEASED/);
  await generate({ dir, output, clear: true, dryRun: false, bump: BUMP });
  const pending = await run(process.execPath, [cli, "--dry-run"], { cwd: root });
  assert.match(pending.stdout, /^# 2\.3\.5 - UNRELEASED/);
});

test("init creates an unreleased 1.0.0 changelog, config and fragments directory", async () => {
  const { dir, output, config } = await makeInitRoot();

  const result = await init({ output, dir, config, dryRun: false });

  assert.equal(result.version, "1.0.0");
  assert.equal(result.title, "1.0.0 - UNRELEASED");
  assert.equal(result.written, true);
  assert.equal(result.config, config);
  assert.equal(result.configWritten, true);
  assert.equal(await readFile(output, "utf8"), "# 1.0.0 - UNRELEASED\n");
  assert.deepEqual(await readdir(dir), []);
  assert.deepEqual(JSON.parse(await readFile(config, "utf8")), {
    sections: [
      { title: "Breaking Changes", bump: "MAJOR" },
      { title: "Features", bump: "MINOR" },
      { title: "Fixes", bump: "PATCH" },
    ],
  });
});

test("init can start at 0.1.0 with breaking changes as a MINOR bump", async () => {
  const { dir, output, config } = await makeInitRoot();

  const result = await init({ output, dir, config, version: "0.1.0", dryRun: false });

  assert.equal(result.version, "0.1.0");
  assert.equal(await readFile(output, "utf8"), "# 0.1.0 - UNRELEASED\n");
  assert.deepEqual(JSON.parse(await readFile(config, "utf8")), {
    sections: [
      { title: "Breaking Changes", bump: "MINOR" },
      { title: "Features", bump: "MINOR" },
      { title: "Fixes", bump: "PATCH" },
    ],
  });
});

test("init leaves an existing config untouched", async () => {
  const { dir, output, config } = await makeInitRoot();
  const existing = '{ "sections": [{ "title": "Fixes", "bump": "PATCH" }] }\n';
  await writeFile(config, existing);

  const result = await init({ output, dir, config, dryRun: false });

  assert.equal(result.configWritten, false);
  assert.equal(await readFile(config, "utf8"), existing);
});

test("init refuses to overwrite an existing changelog", async () => {
  const { dir, output, config } = await makeInitRoot();
  const original = "# 1.0.0\n\n## Features\n\n- Released\n";
  await writeFile(output, original);

  await assert.rejects(init({ output, dir, config, dryRun: false }), /already exists/);
  assert.equal(await readFile(output, "utf8"), original);
  assert.equal(existsSync(config), false);
});

test("init rejects prerelease, build and invalid initial versions", async () => {
  const { dir, output, config } = await makeInitRoot();

  for (const version of ["1.0.0-alpha.1", "1.0.0+build", "nope"]) {
    await assert.rejects(
      init({ output, dir, config, version, dryRun: false }),
      /Invalid initial version|Invalid semantic version/,
    );
  }
  assert.equal(existsSync(output), false);
  assert.equal(existsSync(config), false);
});

test("init dry run reports without writing or creating the directory", async () => {
  const { dir, output, config } = await makeInitRoot();

  const result = await init({ output, dir, config, version: "0.1.0", dryRun: true });

  assert.equal(result.written, false);
  assert.equal(result.configWritten, true);
  assert.equal(existsSync(output), false);
  assert.equal(existsSync(dir), false);
  assert.equal(existsSync(config), false);
});

test("generate keeps 0.1.0 while the initial release is unreleased", async () => {
  const { dir, output, config } = await makeInitRoot();
  await init({ output, dir, config, version: "0.1.0", dryRun: false });
  await writeFile(path.join(dir, "breaking.md"), "## Breaking Changes\n\n- Overhaul\n");

  const result = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: ORDER,
    bump: BUMP,
  });

  assert.equal(result.version, "0.1.0");
  assert.equal(result.previous, undefined);
  assert.equal(
    await readFile(output, "utf8"),
    "# 0.1.0 - UNRELEASED\n\n## Breaking Changes\n\n- Overhaul\n",
  );
});

test("init 0.1.0 config makes breaking changes bump the minor version", async () => {
  const { dir, output, config } = await makeInitRoot();
  await init({ output, dir, config, version: "0.1.0", dryRun: false });
  const loaded = await readConfig(config);

  await writeFile(path.join(dir, "breaking.md"), "## Breaking Changes\n\n- Overhaul\n");
  await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: sectionOrder(loaded),
    bump: sectionBumps(loaded),
  });

  const released = await release({ output, dir, dryRun: false, order: sectionOrder(loaded) });
  assert.deepEqual(released, { version: "0.1.0", written: true });

  await writeFile(path.join(dir, "breaking.md"), "## Breaking Changes\n\n- Another overhaul\n");
  const next = await generate({
    dir,
    output,
    clear: true,
    dryRun: false,
    order: sectionOrder(loaded),
    bump: sectionBumps(loaded),
  });
  assert.equal(next.version, "0.2.0");
  assert.equal(next.previous, "0.1.0");
});

test("CLI init writes the requested initial version and default config", async () => {
  const { root, dir, output, config } = await makeInitRoot();
  const cli = fileURLToPath(new URL("../src/cli.ts", import.meta.url));
  const run = promisify(execFile);

  const result = await run(process.execPath, [cli, "init", "--initial", "0.1.0"], { cwd: root });

  assert.match(result.stdout, /Initialized .*0\.1\.0 - UNRELEASED and wrote .*semfrag\.json/);
  assert.equal(await readFile(output, "utf8"), "# 0.1.0 - UNRELEASED\n");
  assert.deepEqual(await readdir(dir), []);
  assert.equal(JSON.parse(await readFile(config, "utf8")).sections[0].bump, "MINOR");
});
