import { test } from "bun:test";
import assert from "node:assert/strict";
import {
  applyBump,
  baseVersion,
  compareBase,
  highestBase,
  highestBump,
  isBumpLevel,
  isPrerelease,
  nextPrerelease,
  nextVersion,
  parseVersion,
  prereleaseOf,
  prereleaseVersion,
} from "../src/bump.ts";

test("parseVersion accepts plain, v-prefixed and prerelease versions", () => {
  assert.deepEqual(parseVersion("1.2.3"), { major: 1, minor: 2, patch: 3 });
  assert.deepEqual(parseVersion("v1.2.3"), { major: 1, minor: 2, patch: 3 });
  assert.deepEqual(parseVersion("1.2.3-beta.1"), {
    major: 1,
    minor: 2,
    patch: 3,
    prerelease: "beta.1",
  });
});

test("parseVersion rejects invalid versions", () => {
  assert.throws(() => parseVersion("1.2"), /Invalid semantic version/);
  assert.throws(() => parseVersion("not-a-version"), /Invalid semantic version/);
  for (const version of [
    "01.2.3",
    "1.02.3",
    "1.2.03",
    "1.2.3-alpha..1",
    "1.2.3-alpha.01",
    "1.2.3+build..1",
    "9007199254740992.0.0",
  ]) {
    assert.throws(() => parseVersion(version), /Invalid semantic version/);
  }
  assert.equal(parseVersion("1.2.3+build.01").build, "build.01");
});

test("custom dotted prerelease channels increment and invalid channels are rejected", () => {
  assert.equal(
    nextPrerelease("1.2.3", "preview.test", ["1.2.3-preview.test.1", "1.2.3-preview.test.3"]),
    "1.2.3-preview.test.4",
  );
  for (const channel of ["", "bad channel", "alpha..test", "alpha+build", "01"]) {
    assert.throws(() => nextPrerelease("1.2.3", channel, []), /Invalid/);
  }
  assert.throws(() => applyBump("9007199254740991.0.0", "MAJOR"), /Invalid semantic version/);
});

test("applyBump bumps and resets the right components", () => {
  assert.equal(applyBump("1.2.3", "PATCH"), "1.2.4");
  assert.equal(applyBump("1.2.3", "MINOR"), "1.3.0");
  assert.equal(applyBump("1.2.3", "MAJOR"), "2.0.0");
  assert.equal(applyBump("1.2.3-beta.1", "PATCH"), "1.2.4");
});

test("isBumpLevel only accepts known levels", () => {
  assert.equal(isBumpLevel("MAJOR"), true);
  assert.equal(isBumpLevel("major"), false);
  assert.equal(isBumpLevel("HUGE"), false);
});

test("highestBump picks the most significant level", () => {
  assert.equal(highestBump(["PATCH", "MINOR", "PATCH"]), "MINOR");
  assert.equal(highestBump(["PATCH", "MAJOR", "MINOR"]), "MAJOR");
  assert.equal(highestBump([]), undefined);
});

test("nextVersion applies a level or keeps the base", () => {
  assert.equal(nextVersion("1.2.3", "MINOR"), "1.3.0");
  assert.equal(nextVersion("1.2.3", undefined), "1.2.3");
});

test("baseVersion strips prerelease and build metadata", () => {
  assert.equal(baseVersion("1.2.3"), "1.2.3");
  assert.equal(baseVersion("1.2.3-alpha.4"), "1.2.3");
  assert.equal(baseVersion("1.2.3-rc.1+build.7"), "1.2.3");
});

test("prereleaseOf and isPrerelease detect prereleases", () => {
  assert.equal(prereleaseOf("1.2.3-alpha.4"), "alpha.4");
  assert.equal(prereleaseOf("1.2.3"), undefined);
  assert.equal(prereleaseOf("not-a-version"), undefined);
  assert.equal(isPrerelease("1.2.3-beta.1"), true);
  assert.equal(isPrerelease("1.2.3"), false);
});

test("prereleaseVersion tags the base version", () => {
  assert.equal(prereleaseVersion("1.2.3", "alpha", 1), "1.2.3-alpha.1");
  assert.equal(prereleaseVersion("1.2.3-beta.1", "rc", 2), "1.2.3-rc.2");
});

test("nextPrerelease increments the matching channel and resets others", () => {
  const versions = ["1.2.3-alpha.1", "1.2.3-alpha.2", "1.2.2", "1.3.0-alpha.1"];
  assert.equal(nextPrerelease("1.2.3", "alpha", versions), "1.2.3-alpha.3");
  assert.equal(nextPrerelease("1.2.3", "beta", versions), "1.2.3-beta.1");
  assert.equal(nextPrerelease("1.2.4", "alpha", versions), "1.2.4-alpha.1");
});

test("compareBase and highestBase compare core versions", () => {
  assert.equal(compareBase("1.2.3", "1.2.4"), -1);
  assert.equal(compareBase("2.0.0", "1.9.9"), 1);
  assert.equal(compareBase("1.2.3-alpha.1", "1.2.3"), 0);
  assert.equal(highestBase("1.0.1", "1.0.0", "1.1.0-alpha.1"), "1.1.0");
});
