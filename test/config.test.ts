import { test } from "bun:test";
import assert from "node:assert/strict";
import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import {
  parseConfig,
  loadConfig,
  sectionOrder,
  sectionBumps,
  sectionTypes,
} from "../src/config.ts";

test("parseConfig reads sections and normalizes bump levels", () => {
  const config = parseConfig(
    '{"sections":[{"title":"Fixes","bump":"patch"},{"title":"Features","bump":"MINOR"},{"title":"Docs"}]}',
    "test",
  );
  assert.deepEqual(config, {
    sections: [
      { title: "Fixes", bump: "PATCH" },
      { title: "Features", bump: "MINOR" },
      { title: "Docs" },
    ],
  });
  assert.deepEqual(sectionOrder(config), ["Fixes", "Features", "Docs"]);
  assert.deepEqual(sectionBumps(config), { Fixes: "PATCH", Features: "MINOR" });
});

test("parseConfig reads and normalizes section types", () => {
  const config = parseConfig(
    '{"sections":[{"title":"Fixes","bump":"PATCH"},{"title":"Details","type":"RAW"}]}',
    "test",
  );
  assert.deepEqual(config, {
    sections: [
      { title: "Fixes", bump: "PATCH" },
      { title: "Details", type: "raw" },
    ],
  });
  assert.deepEqual(sectionTypes(config), { Details: "raw" });
});

test("parseConfig rejects invalid bump levels", () => {
  assert.throws(
    () => parseConfig('{"sections":[{"title":"Fixes","bump":"HUGE"}]}', "test"),
    /invalid bump level for "Fixes"/,
  );
});

test("parseConfig rejects invalid section types", () => {
  assert.throws(
    () => parseConfig('{"sections":[{"title":"Details","type":"fancy"}]}', "test"),
    /invalid section type for "Details"/,
  );
});

test("parseConfig rejects invalid JSON", () => {
  assert.throws(() => parseConfig("{oops", "test"), /test: invalid JSON/);
});

test("parseConfig rejects a missing or empty sections array", () => {
  assert.throws(() => parseConfig("{}", "test"), /"sections" must be a non-empty array/);
  assert.throws(
    () => parseConfig('{"sections":[]}', "test"),
    /"sections" must be a non-empty array/,
  );
});

test("parseConfig rejects sections that are not objects", () => {
  assert.throws(
    () => parseConfig('{"sections":["Fixes"]}', "test"),
    /sections\[0\] must be an object/,
  );
});

test("parseConfig rejects missing or empty titles", () => {
  assert.throws(
    () => parseConfig('{"sections":[{"bump":"PATCH"}]}', "test"),
    /\.title must be a non-empty string/,
  );
  assert.throws(
    () => parseConfig('{"sections":[{"title":"  "}]}', "test"),
    /\.title must be a non-empty string/,
  );
});

test("parseConfig rejects duplicate sections", () => {
  assert.throws(
    () => parseConfig('{"sections":[{"title":"Fixes"},{"title":"Fixes"}]}', "test"),
    /duplicate section "Fixes"/,
  );
});

test("loadConfig reports an explicitly missing config file", async () => {
  const dir = await mkdtemp(path.join(tmpdir(), "semfrag-config-"));
  await assert.rejects(loadConfig(path.join(dir, "missing.json")), /Config file not found/);
});

test("loadConfig reads a config file from disk", async () => {
  const dir = await mkdtemp(path.join(tmpdir(), "semfrag-config-"));
  const file = path.join(dir, "semfrag.json");
  await writeFile(file, '{"sections":[{"title":"Fixes","bump":"PATCH"}]}');
  assert.deepEqual(await loadConfig(file), { sections: [{ title: "Fixes", bump: "PATCH" }] });
});
