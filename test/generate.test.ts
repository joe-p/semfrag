import { test } from "bun:test";
import assert from "node:assert/strict";
import {
  parseFragment,
  mergeFragments,
  mergeSections,
  parseChangelog,
  renderChangelog,
  prependChangelog,
} from "../src/generate.ts";

test("parseFragment splits sections", () => {
  const sections = parseFragment("## Fixes\n\n- a\n- b\n\n## Features\n\n- c\n");
  assert.deepEqual(sections, [
    { title: "Fixes", lines: ["- a", "- b"] },
    { title: "Features", lines: ["- c"] },
  ]);
});

test("parseFragment rejects content before the first heading", () => {
  assert.throws(
    () => parseFragment("intro\n\n## Fixes\n\n- a\n"),
    /Line 1: expected a ## section heading/,
  );
});

test("merging keeps multiline list items intact and deduplicates whole items", () => {
  const fragments = [
    { name: "a.md", content: "## Fixes\n- Fix A\n  - Details\n\n  More details.\n" },
    { name: "b.md", content: "## Fixes\n- Fix B\n  - Details\n" },
    { name: "c.md", content: "## Fixes\n- Fix A\n  - Details\n\n  More details.\n" },
  ];
  assert.equal(
    renderChangelog(mergeFragments(fragments)),
    "# Unreleased\n\n## Fixes\n\n- Fix A\n  - Details\n\n  More details.\n- Fix B\n  - Details\n",
  );
});

test("fragment validation reports filenames and rejects empty fragments", () => {
  for (const content of ["", "## Fixes\n", "Missing a heading"]) {
    assert.throws(() => mergeFragments([{ name: "bad.md", content }]), /bad\.md:/);
  }
});

test("list item deduplication preserves fenced examples and blank lines between items", () => {
  const content = "## Fixes\n\n- Example\n  ```md\n- Sample\n- Sample\n  ```\n\n- Another fix\n";
  const sections = mergeFragments([
    { name: "a.md", content },
    { name: "b.md", content },
  ]);
  assert.equal(renderChangelog(sections), `# Unreleased\n\n${content}`);
});

test("changelog title and introduction are not version blocks", () => {
  const blocks = parseChangelog("# Changelog\n\nProject notes.\n\n# 1.0.0\n\n## Fixes\n- fix\n");
  assert.equal(blocks.length, 1);
  assert.equal(blocks[0]?.version, "1.0.0");
});

test("fenced headings remain raw content rather than delimiters", () => {
  const content = "# 1.0.0\n\n## Details\n\n```md\n# Example\n## Example section\n```\n";
  const blocks = parseChangelog(content, { Details: "raw" });
  assert.equal(blocks.length, 1);
  assert.equal(blocks[0]?.sections[0]?.body, "```md\n# Example\n## Example section\n```");
});

test("parseFragment preserves raw markdown for raw sections", () => {
  const sections = parseFragment(
    "## Details\n\nSome intro.\n\n### Nested\n\n- item\n\n```js\nconst a = 1;\n```\n",
    { Details: "raw" },
  );

  assert.deepEqual(sections, [
    {
      title: "Details",
      type: "raw",
      lines: [],
      body: "Some intro.\n\n### Nested\n\n- item\n\n```js\nconst a = 1;\n```",
    },
  ]);
});

test("parseFragment rejects level-1 and level-2 headings inside raw sections", () => {
  assert.throws(
    () => parseFragment("## Details\n\n# Nope\n", { Details: "raw" }),
    /may not contain a level-1 or level-2 heading/,
  );
});

test("mergeFragments groups sections by title in first-seen order", () => {
  const merged = mergeFragments([
    { name: "fix-01.md", content: "## Fixes\n\n- fix #01\n" },
    { name: "fix-02.md", content: "## Fixes\n\n- fix #02\n" },
    { name: "feat-01.md", content: "## Features\n\n- Added a new feature!\n" },
  ]);

  assert.deepEqual(merged, [
    { title: "Features", lines: ["- Added a new feature!"] },
    { title: "Fixes", lines: ["- fix #01", "- fix #02"] },
  ]);
});

test("mergeFragments drops sections with no entries", () => {
  const merged = mergeFragments([
    { name: "a.md", content: "## Fixes\n\n- fix #01\n\n## Features\n" },
  ]);
  assert.deepEqual(merged, [{ title: "Fixes", lines: ["- fix #01"] }]);
});

test("mergeFragments follows the configured section order", () => {
  const fragments = [
    { name: "fix-01.md", content: "## Fixes\n\n- fix #01\n" },
    { name: "feat-01.md", content: "## Features\n\n- Added a new feature!\n" },
  ];

  assert.deepEqual(mergeFragments(fragments, ["Fixes", "Features"]), [
    { title: "Fixes", lines: ["- fix #01"] },
    { title: "Features", lines: ["- Added a new feature!"] },
  ]);
  assert.deepEqual(mergeFragments(fragments, ["Features", "Fixes"]), [
    { title: "Features", lines: ["- Added a new feature!"] },
    { title: "Fixes", lines: ["- fix #01"] },
  ]);
});

test("mergeFragments throws on a section missing from the configured order", () => {
  assert.throws(
    () =>
      mergeFragments([{ name: "a.md", content: "## Chores\n\n- chore\n" }], ["Fixes", "Features"]),
    /Unknown changelog section "## Chores"/,
  );
});

test("renderChangelog produces the expected markdown", () => {
  const output = renderChangelog(
    [
      { title: "Fixes", lines: ["- fix #01", "- fix #02"] },
      { title: "Features", lines: ["- Added a new feature!"] },
    ],
    "Unreleased",
  );

  assert.equal(
    output,
    "# Unreleased\n\n## Fixes\n\n- fix #01\n- fix #02\n\n## Features\n\n- Added a new feature!\n",
  );
});

test("prependChangelog keeps existing content after the new entry", () => {
  const existing = "# Changelog\n\n## 1.0.0\n\n- old\n";
  const result = prependChangelog(existing, "# Unreleased\n\n## Fixes\n\n- new\n");
  assert.equal(result, "# Unreleased\n\n## Fixes\n\n- new\n\n# Changelog\n\n## 1.0.0\n\n- old\n");
});

test("mergeSections groups by title and drops duplicate lines", () => {
  const merged = mergeSections([
    [{ title: "Fixes", lines: ["- fix #01"] }],
    [{ title: "Fixes", lines: ["- fix #01", "- fix #02"] }],
  ]);
  assert.deepEqual(merged, [{ title: "Fixes", lines: ["- fix #01", "- fix #02"] }]);
});

test("mergeSections concatenates raw bodies and drops duplicate blocks", () => {
  const merged = mergeSections([
    [{ title: "Details", type: "raw", lines: [], body: "one" }],
    [{ title: "Details", type: "raw", lines: [], body: "two" }],
    [{ title: "Details", type: "raw", lines: [], body: "one" }],
  ]);
  assert.deepEqual(merged, [{ title: "Details", type: "raw", lines: [], body: "one\n\ntwo" }]);
});

test("renderChangelog renders raw sections verbatim", () => {
  const output = renderChangelog(
    [{ title: "Details", type: "raw", lines: [], body: "Some intro.\n\n### Nested\n\n- item" }],
    "Unreleased",
  );
  assert.equal(output, "# Unreleased\n\n## Details\n\nSome intro.\n\n### Nested\n\n- item\n");
});

test("parseChangelog splits released and unreleased blocks", () => {
  const blocks = parseChangelog(
    "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- new\n\n# 1.0.0\n\n## Features\n\n- old\n",
  );

  assert.deepEqual(blocks, [
    {
      version: "1.1.0",
      unreleased: true,
      sections: [{ title: "Fixes", lines: ["- new"] }],
      raw: "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- new",
    },
    {
      version: "1.0.0",
      unreleased: false,
      sections: [{ title: "Features", lines: ["- old"] }],
      raw: "# 1.0.0\n\n## Features\n\n- old",
    },
  ]);
});

test("parseChangelog reads a version from a dated heading", () => {
  const blocks = parseChangelog(
    "# 1.1.0 - January 1st, 2026\n\n## Fixes\n\n- new\n\n# 1.0.1-alpha.1 - December 31st, 2025\n\n## Fixes\n\n- preview\n",
  );

  assert.equal(blocks[0]?.version, "1.1.0");
  assert.equal(blocks[0]?.unreleased, false);
  assert.equal(blocks[1]?.version, "1.0.1-alpha.1");
  assert.equal(blocks[1]?.unreleased, false);
});

test("parseChangelog preserves raw block text", () => {
  const markdown = "# 1.0.0\n\n## Features\n\n- old\n\n# 0.9.0\n\n## Fixes\n\n- older\n";
  const [first] = parseChangelog(markdown);
  assert.equal(first?.raw, "# 1.0.0\n\n## Features\n\n- old");
});

test("parseChangelog preserves raw sections when types are given", () => {
  const blocks = parseChangelog("# 1.0.0\n\n## Details\n\nIntro.\n\n### Nested\n\n- x\n", {
    Details: "raw",
  });
  assert.deepEqual(blocks[0]?.sections, [
    { title: "Details", type: "raw", lines: [], body: "Intro.\n\n### Nested\n\n- x" },
  ]);
});
