import { test } from "bun:test";
import assert from "node:assert/strict";
import { formatReleaseDate } from "../src/date.ts";

test("formatReleaseDate renders a human-readable date", () => {
  assert.equal(formatReleaseDate(new Date(2026, 0, 1)), "January 1st, 2026");
  assert.equal(formatReleaseDate(new Date(2026, 11, 31)), "December 31st, 2026");
});

test("formatReleaseDate uses the correct ordinal suffix", () => {
  const cases: Array<[number, string]> = [
    [1, "1st"],
    [2, "2nd"],
    [3, "3rd"],
    [4, "4th"],
    [11, "11th"],
    [12, "12th"],
    [13, "13th"],
    [21, "21st"],
    [22, "22nd"],
    [23, "23rd"],
  ];
  for (const [day, expected] of cases) {
    assert.equal(formatReleaseDate(new Date(2026, 5, day)), `June ${expected}, 2026`);
  }
});
