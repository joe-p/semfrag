package semfrag

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	testOrder = []string{"Breaking Changes", "Fixes", "Features"}
	testBump  = map[string]BumpLevel{"Breaking Changes": BumpMajor, "Fixes": BumpPatch, "Features": BumpMinor}
	testDate  = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
)

const releasedOn = "January 1st, 2026"

func orderWith(extra ...string) []string {
	return append(append([]string{}, testOrder...), extra...)
}

func makeRoot(t *testing.T) (root, dir, output string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "changelog.d")
	output = filepath.Join(root, "CHANGELOG.md")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}
	return root, dir, output
}

func makeInitRoot(t *testing.T) (root, dir, output, config string) {
	t.Helper()
	root = t.TempDir()
	return root, filepath.Join(root, "changelog.d"), filepath.Join(root, "CHANGELOG.md"), filepath.Join(root, "semfrag.json")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", path, err)
	}
	return string(data)
}

func readDir(t *testing.T, path string) []string {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("ReadDir(%s) returned error: %v", path, err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func TestGenerateReadsVersionFromChangelog(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.0\n\n## Features\n\n- Released 1.0!\n")
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n\n- Some fix\n")

	first, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: testOrder, Bump: testBump})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, first.Version, "1.0.1")
	assertEqual(t, first.Previous, "1.0.0")
	assertEqual(t, first.Level, BumpPatch)
	assertEqual(t, readFile(t, output), "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- Some fix\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")
	assertEqual(t, readDir(t, dir), []string{})

	writeFile(t, filepath.Join(dir, "feat.md"), "## Features\n\n- A new feature!\n")
	second, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: testOrder, Bump: testBump})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, second.Version, "1.1.0")
	assertEqual(t, second.Level, BumpMinor)
	assertEqual(t, readFile(t, output), "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- Some fix\n\n## Features\n\n- A new feature!\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")
}

func TestGenerateKeepsInitialUnreleasedVersion(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n\n- Some fix\n")

	first, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: testOrder, Bump: testBump})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, first.Version, "1.0.0")
	assertEqual(t, first.Previous, "")
	assertEqual(t, readFile(t, output), "# 1.0.0 - UNRELEASED\n\n## Fixes\n\n- Some fix\n")

	writeFile(t, filepath.Join(dir, "feat.md"), "## Features\n\n- A new feature!\n")
	second, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: testOrder, Bump: testBump})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, second.Version, "1.0.0")
	assertEqual(t, readFile(t, output), "# 1.0.0 - UNRELEASED\n\n## Fixes\n\n- Some fix\n\n## Features\n\n- A new feature!\n")
}

func TestGenerateIsIdempotentWhenFragmentsKept(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n\n- Some fix\n")

	options := GenerateOptions{Dir: dir, Output: output, Clear: false, DryRun: false, Order: testOrder, Bump: testBump}
	if _, err := Generate(options); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if _, err := Generate(options); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, strings.Count(readFile(t, output), "- Some fix"), 1)
}

func TestGeneratePreservesRawSectionsAndNestedMarkdown(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, filepath.Join(dir, "details.md"), "## Details\n\nSome **bold** text.\n\n### Nested\n\n- item\n")

	result, err := Generate(GenerateOptions{
		Dir:    dir,
		Output: output,
		Clear:  true,
		DryRun: false,
		Order:  orderWith("Details"),
		Bump:   testBump,
		Types:  SectionTypes{"Details": SectionRaw},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, result.Version, "1.0.0")
	assertEqual(t, readFile(t, output), "# 1.0.0 - UNRELEASED\n\n## Details\n\nSome **bold** text.\n\n### Nested\n\n- item\n")
}

func TestGenerateIsIdempotentForRawSections(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, filepath.Join(dir, "details.md"), "## Details\n\nLine one.\n\nLine two.\n")

	options := GenerateOptions{
		Dir:    dir,
		Output: output,
		Clear:  false,
		DryRun: false,
		Order:  []string{"Details"},
		Bump:   map[string]BumpLevel{},
		Types:  SectionTypes{"Details": SectionRaw},
	}
	if _, err := Generate(options); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if _, err := Generate(options); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, strings.Count(readFile(t, output), "Line one."), 1)
}

func TestPromoteStableMergesRawSectionsFromPrereleases(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.1 - UNRELEASED\n\n## Details\n\nNew details.\n\n# 1.0.1-alpha.1\n\n## Details\n\nOld details.\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	result, err := Promote(PromoteOptions{
		Output:  output,
		Dir:     dir,
		DryRun:  false,
		Channel: ChannelStable,
		Order:   orderWith("Details"),
		Types:   SectionTypes{"Details": SectionRaw},
		Now:     testDate,
	})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, result, PromoteResult{Version: "1.0.1", Written: true})
	assertEqual(t, readFile(t, output), "# 1.0.1 - "+releasedOn+"\n\n## Details\n\nNew details.\n\nOld details.\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")
}

func TestGenerateDoesNothingWithoutFragments(t *testing.T) {
	_, dir, output := makeRoot(t)

	result, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: testOrder, Bump: testBump})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, result.Written, false)
	assertEqual(t, len(result.Fragments), 0)
	assertEqual(t, fileExists(output), false)
}

func TestGenerateKeepsLastReleasedVersionWhenNoBumps(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.2.3\n\n## Features\n\n- old\n")
	writeFile(t, filepath.Join(dir, "docs.md"), "## Docs\n\n- docs\n")

	result, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: []string{"Docs"}, Bump: map[string]BumpLevel{}})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, result.Version, "1.2.3")
	assertEqual(t, result.Level, BumpLevel(""))
}

func TestPromoteStableRemovesUnreleasedMarker(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.1.0 - UNRELEASED\n\n## Features\n\n- A new feature!\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	result, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelStable, Now: testDate})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, result, PromoteResult{Version: "1.1.0", Written: true})
	assertEqual(t, readFile(t, output), "# 1.1.0 - "+releasedOn+"\n\n## Features\n\n- A new feature!\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")
}

func TestLatestReturnsMostRecentReleasedVersion(t *testing.T) {
	_, _, output := makeRoot(t)
	writeFile(t, output, "# 1.2.0 - UNRELEASED\n\n## Features\n\n- Pending\n\n# 1.1.0\n\n## Features\n\n- Released\n\n# 1.0.0\n\n## Features\n\n- Old\n")

	result, err := Latest(LatestOptions{Output: output})
	if err != nil {
		t.Fatalf("Latest returned error: %v", err)
	}
	assertEqual(t, result, LatestResult{Version: "1.1.0"})
}

func TestLatestTreatsPrereleaseAsReleased(t *testing.T) {
	_, _, output := makeRoot(t)
	writeFile(t, output, "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- Pending\n\n# 1.0.1-alpha.1\n\n## Fixes\n\n- Preview\n\n# 1.0.0\n\n## Features\n\n- Old\n")

	result, err := Latest(LatestOptions{Output: output})
	if err != nil {
		t.Fatalf("Latest returned error: %v", err)
	}
	assertEqual(t, result, LatestResult{Version: "1.0.1-alpha.1"})
}

func TestLatestFailsWithoutReleasedVersion(t *testing.T) {
	_, _, output := makeRoot(t)
	writeFile(t, output, "# 1.0.0 - UNRELEASED\n\n## Features\n\n- Pending\n")

	_, err := Latest(LatestOptions{Output: output})
	assertErrorContains(t, err, "no released version")
}

func TestNotesReturnsBodyOfLatestRelease(t *testing.T) {
	_, _, output := makeRoot(t)
	writeFile(t, output, "# 1.2.0 - UNRELEASED\n\n## Features\n\n- Pending\n\n# 1.1.0\n\n## Fixes\n\n- Fixed\n\n## Features\n\n- Added\n\n# 1.0.0\n\n## Features\n\n- Old\n")

	result, err := Notes(NotesOptions{Output: output})
	if err != nil {
		t.Fatalf("Notes returned error: %v", err)
	}
	assertEqual(t, result, NotesResult{Version: "1.1.0", Notes: "## Fixes\n\n- Fixed\n\n## Features\n\n- Added"})
}

func TestNotesTreatsPrereleaseAsLatestRelease(t *testing.T) {
	_, _, output := makeRoot(t)
	writeFile(t, output, "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- Pending\n\n# 1.0.1-alpha.1\n\n## Fixes\n\n- Preview\n\n# 1.0.0\n\n## Features\n\n- Old\n")

	result, err := Notes(NotesOptions{Output: output})
	if err != nil {
		t.Fatalf("Notes returned error: %v", err)
	}
	assertEqual(t, result, NotesResult{Version: "1.0.1-alpha.1", Notes: "## Fixes\n\n- Preview"})
}

func TestNotesFailsWithoutReleasedVersion(t *testing.T) {
	_, _, output := makeRoot(t)
	writeFile(t, output, "# 1.0.0 - UNRELEASED\n\n## Features\n\n- Pending\n")

	_, err := Notes(NotesOptions{Output: output})
	assertErrorContains(t, err, "no released version")
}

func TestPromoteStableFailsWithoutUnreleasedSection(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	_, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelStable})
	assertErrorContains(t, err, "cannot promote stable to stable")
}

func TestPromoteFailsWhileFragmentsPending(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.1.0 - UNRELEASED\n\n## Features\n\n- A new feature!\n")
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n\n- Some fix\n")

	_, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelStable})
	assertErrorContains(t, err, "pending fragment")
}

func TestPromoteDryRunDoesNotWrite(t *testing.T) {
	_, dir, output := makeRoot(t)
	before := "# 1.1.0 - UNRELEASED\n\n## Features\n\n- A new feature!\n"
	writeFile(t, output, before)

	result, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: true, Channel: ChannelStable})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, result.Written, false)
	assertEqual(t, readFile(t, output), before)
}

func TestPrereleaseFlow(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- Fixed a bug\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	alpha, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelAlpha, Now: testDate})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, alpha, PromoteResult{Version: "1.0.1-alpha.1", Written: true})
	assertEqual(t, readFile(t, output), "# 1.0.1-alpha.1 - "+releasedOn+"\n\n## Fixes\n\n- Fixed a bug\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n\n- Some new fix\n")
	generated, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: testOrder, Bump: testBump})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, generated.Version, "1.0.1")
	assertEqual(t, generated.Previous, "1.0.0")
	assertEqual(t, readFile(t, output), "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- Some new fix\n\n# 1.0.1-alpha.1 - "+releasedOn+"\n\n## Fixes\n\n- Fixed a bug\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	final, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelStable, Order: testOrder, Now: testDate})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, final, PromoteResult{Version: "1.0.1", Written: true})
	assertEqual(t, readFile(t, output), "# 1.0.1 - "+releasedOn+"\n\n## Fixes\n\n- Some new fix\n- Fixed a bug\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")
}

func TestPromoteAlphaIncrementsSameChannel(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- More\n\n# 1.0.1-alpha.1\n\n## Fixes\n\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	result, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelAlpha, Now: testDate})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, result.Version, "1.0.1-alpha.2")
	assertEqual(t, strings.HasPrefix(readFile(t, output), "# 1.0.1-alpha.2 - January 1st, 2026\n"), true)
}

func TestPromoteResetsNumberWhenSwitchingChannel(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.1 - UNRELEASED\n\n## Fixes\n\n- More\n\n# 1.0.1-alpha.2\n\n## Fixes\n\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	result, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelBeta})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, result.Version, "1.0.1-beta.1")
}

func TestPromoteStablePromotesTopPrerelease(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.1-alpha.2\n\n## Fixes\n\n- More\n\n# 1.0.1-alpha.1\n\n## Fixes\n\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	result, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelStable, Now: testDate})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, result, PromoteResult{Version: "1.0.1", Written: true})
	assertEqual(t, readFile(t, output), "# 1.0.1 - "+releasedOn+"\n\n## Fixes\n\n- More\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")
}

func TestPrereleaseFailsWithoutUnreleasedSection(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	_, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelAlpha})
	assertErrorContains(t, err, "cannot promote stable to alpha")
}

func TestPromoteEnforcesPrereleaseLadder(t *testing.T) {
	_, dir, output := makeRoot(t)
	cases := []struct {
		markdown string
		channel  PromoteChannel
	}{
		{"# 1.0.1-beta.1\n\n## Fixes\n- fix\n", ChannelAlpha},
		{"# 1.0.1-rc.1\n\n## Fixes\n- fix\n", ChannelAlpha},
		{"# 1.0.1-rc.1\n\n## Fixes\n- fix\n", ChannelBeta},
		{"# 1.0.1\n\n## Fixes\n- fix\n", ChannelRC},
	}
	for _, test := range cases {
		writeFile(t, output, test.markdown)
		_, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: test.channel})
		assertErrorContains(t, err, "cannot promote")
		assertErrorContains(t, err, " to "+string(test.channel))
		assertEqual(t, readFile(t, output), test.markdown)
	}
}

func TestPromoteAdvancesTopPrereleaseUpLadder(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# 1.0.1-alpha.1\n\n## Fixes\n\n- First\n\n# 1.0.0\n\n## Features\n\n- Released 1.0!\n")

	beta, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelBeta, Now: testDate})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, beta, PromoteResult{Version: "1.0.1-beta.1", Written: true})
	assertEqual(t, strings.HasPrefix(readFile(t, output), "# 1.0.1-beta.1 - "), true)

	rc, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelRC, Now: testDate})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, rc, PromoteResult{Version: "1.0.1-rc.1", Written: true})
	assertEqual(t, strings.HasPrefix(readFile(t, output), "# 1.0.1-rc.1 - "), true)

	stable, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelStable, Now: testDate})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, stable, PromoteResult{Version: "1.0.1", Written: true})
	assertEqual(t, strings.HasPrefix(readFile(t, output), "# 1.0.1 - "), true)
}

func TestGenerateAndPromotePreservePreamble(t *testing.T) {
	_, dir, output := makeRoot(t)
	preamble := "# Changelog\n\nRelease notes for this project.\n\n"
	writeFile(t, output, preamble+"# 1.0.0\n\n## Fixes\n\n- Old fix\n")
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n\n- New fix\n")

	if _, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: testOrder, Bump: testBump}); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, strings.HasPrefix(readFile(t, output), preamble+"# 1.0.1 - UNRELEASED\n"), true)

	if _, err := Promote(PromoteOptions{Dir: dir, Output: output, DryRun: false, Channel: ChannelAlpha, Now: testDate}); err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, strings.HasPrefix(readFile(t, output), preamble+"# 1.0.1-alpha.1 - "+releasedOn+"\n"), true)

	if _, err := Promote(PromoteOptions{Dir: dir, Output: output, DryRun: false, Channel: ChannelStable, Now: testDate}); err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, strings.HasPrefix(readFile(t, output), preamble+"# 1.0.1 - "+releasedOn+"\n"), true)
}

func TestGeneratePreservesPreambleWhenCreatingFirstVersion(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, output, "# Changelog\n\nProject notes.\n")
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n- fix\n")

	if _, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false}); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, strings.HasPrefix(readFile(t, output), "# Changelog\n\nProject notes.\n\n# 1.0.0 - UNRELEASED"), true)
}

func TestInvalidFragmentsLeaveEverythingUntouched(t *testing.T) {
	_, dir, output := makeRoot(t)
	original := "# 1.0.0\n\n## Fixes\n- Old fix\n"
	writeFile(t, output, original)
	writeFile(t, filepath.Join(dir, "good.md"), "## Fixes\n- Good fix\n")
	writeFile(t, filepath.Join(dir, "bad.md"), "Forgot the heading\n")

	_, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false})
	assertErrorContains(t, err, "bad.md: line 1")
	assertEqual(t, readFile(t, output), original)
	assertEqual(t, readDir(t, dir), []string{"bad.md", "good.md"})
}

func TestInvalidVersionHeadingsCannotDiscardHistory(t *testing.T) {
	_, dir, output := makeRoot(t)
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n- new\n")
	for _, original := range []string{
		"# 01.2.3\n\n## Fixes\n- old\n",
		"# 1.2.3\n\n## Fixes\n- old\n\n# 1.2.4 - UNRELEASED\n\n## Fixes\n- pending\n",
	} {
		writeFile(t, output, original)
		_, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false})
		if err == nil {
			t.Fatalf("expected error for %q", original)
		}
		if !strings.Contains(err.Error(), "invalid semantic version") && !strings.Contains(err.Error(), "only at the top") {
			t.Fatalf("unexpected error: %v", err)
		}
		assertEqual(t, readFile(t, output), original)
		assertEqual(t, readDir(t, dir), []string{"fix.md"})
	}
}

func TestStdoutGenerationReadsInputWithoutWriting(t *testing.T) {
	_, dir, output := makeRoot(t)
	original := "# 2.3.4\n\n## Fixes\n- old\n"
	writeFile(t, output, original)
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n- new\n")

	result, err := Generate(GenerateOptions{Dir: dir, Output: "-", Input: output, Clear: true, DryRun: false, Bump: testBump})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, result.Version, "2.3.5")
	assertEqual(t, result.Written, false)
	assertEqual(t, result.Cleared, []string{})
	assertEqual(t, readFile(t, output), original)
	assertEqual(t, readDir(t, dir), []string{"fix.md"})
}

func TestFragmentEditsDuringWritingAreKept(t *testing.T) {
	_, dir, output := makeRoot(t)
	file := filepath.Join(dir, "fix.md")
	writeFile(t, file, "## Fixes\n- original\n")

	originalRename := renameFile
	var onRename func()
	renameFile = func(oldPath, newPath string) error {
		if err := originalRename(oldPath, newPath); err != nil {
			return err
		}
		if onRename != nil {
			onRename()
		}
		return nil
	}
	defer func() { renameFile = originalRename }()

	onRename = func() {
		writeFile(t, file, "## Fixes\n- edited\n")
		writeFile(t, filepath.Join(dir, "new.md"), "## Fixes\n- added\n")
	}
	result, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false})
	onRename = nil
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, result.Fragments, []string{"fix.md"})
	assertEqual(t, result.Cleared, []string{})
	assertEqual(t, strings.Contains(readFile(t, output), "- original"), true)
	assertEqual(t, readDir(t, dir), []string{"fix.md", "new.md"})
	assertEqual(t, readFile(t, file), "## Fixes\n- edited\n")
}

func TestFailedAtomicReplacementLeavesOriginalIntact(t *testing.T) {
	root, dir, output := makeRoot(t)
	original := "# 1.0.0\n\n## Fixes\n- old\n"
	writeFile(t, output, original)
	writeFile(t, filepath.Join(dir, "fix.md"), "## Fixes\n- new\n")

	originalRename := renameFile
	failRename := true
	renameFile = func(oldPath, newPath string) error {
		if failRename {
			return errors.New("Simulated rename failure")
		}
		return originalRename(oldPath, newPath)
	}
	defer func() { renameFile = originalRename }()

	_, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false})
	assertErrorContains(t, err, "Simulated rename failure")
	failRename = false
	assertEqual(t, readFile(t, output), original)
	assertEqual(t, readDir(t, dir), []string{"fix.md"})
	assertEqual(t, readDir(t, root), []string{"CHANGELOG.md", "changelog.d"})
}

func TestPromoteChannelsReturnsIndependentSlices(t *testing.T) {
	channels := PromoteChannels()
	channels[0] = "custom"
	assertEqual(t, IsPromoteChannel("custom"), false)
	assertEqual(t, IsPromoteChannel("stable"), true)
	assertEqual(t, PromoteChannels()[0], ChannelStable)
}

func TestInitCreatesUnreleasedChangelogConfigAndDirectory(t *testing.T) {
	_, dir, output, config := makeInitRoot(t)

	result, err := Init(InitOptions{Output: output, Dir: dir, Config: config, DryRun: false})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	assertEqual(t, result.Version, "1.0.0")
	assertEqual(t, result.Title, "1.0.0 - UNRELEASED")
	assertEqual(t, result.Written, true)
	assertEqual(t, result.Config, config)
	assertEqual(t, result.ConfigWritten, true)
	assertEqual(t, readFile(t, output), "# 1.0.0 - UNRELEASED\n")
	assertEqual(t, readDir(t, dir), []string{})

	parsed, err := ParseConfig(readFile(t, config), config)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	assertEqual(t, parsed, ChangelogConfig{Sections: []SectionConfig{
		{Title: "Breaking Changes", Bump: BumpMajor},
		{Title: "Features", Bump: BumpMinor},
		{Title: "Fixes", Bump: BumpPatch},
	}})
}

func TestInitCanStartAtZero(t *testing.T) {
	_, dir, output, config := makeInitRoot(t)

	result, err := Init(InitOptions{Output: output, Dir: dir, Config: config, Version: "0.1.0", DryRun: false})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	assertEqual(t, result.Version, "0.1.0")
	assertEqual(t, readFile(t, output), "# 0.1.0 - UNRELEASED\n")

	parsed, err := ParseConfig(readFile(t, config), config)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	assertEqual(t, parsed, ChangelogConfig{Sections: []SectionConfig{
		{Title: "Breaking Changes", Bump: BumpMinor},
		{Title: "Features", Bump: BumpMinor},
		{Title: "Fixes", Bump: BumpPatch},
	}})
}

func TestInitLeavesExistingConfigUntouched(t *testing.T) {
	_, dir, output, config := makeInitRoot(t)
	existing := `{ "sections": [{ "title": "Fixes", "bump": "PATCH" }] }` + "\n"
	writeFile(t, config, existing)

	result, err := Init(InitOptions{Output: output, Dir: dir, Config: config, DryRun: false})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	assertEqual(t, result.ConfigWritten, false)
	assertEqual(t, readFile(t, config), existing)
}

func TestInitRefusesToOverwriteExistingChangelog(t *testing.T) {
	_, dir, output, config := makeInitRoot(t)
	original := "# 1.0.0\n\n## Features\n\n- Released\n"
	writeFile(t, output, original)

	_, err := Init(InitOptions{Output: output, Dir: dir, Config: config, DryRun: false})
	assertErrorContains(t, err, "already exists")
	assertEqual(t, readFile(t, output), original)
	assertEqual(t, fileExists(config), false)
}

func TestInitRejectsPrereleaseBuildAndInvalidVersions(t *testing.T) {
	_, dir, output, config := makeInitRoot(t)

	for _, version := range []string{"1.0.0-alpha.1", "1.0.0+build", "nope"} {
		_, err := Init(InitOptions{Output: output, Dir: dir, Config: config, Version: version, DryRun: false})
		if err == nil {
			t.Fatalf("expected error for version %q", version)
		}
		if !strings.Contains(err.Error(), "invalid initial version") && !strings.Contains(err.Error(), "invalid semantic version") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	assertEqual(t, fileExists(output), false)
	assertEqual(t, fileExists(config), false)
}

func TestInitDryRunReportsWithoutWriting(t *testing.T) {
	_, dir, output, config := makeInitRoot(t)

	result, err := Init(InitOptions{Output: output, Dir: dir, Config: config, Version: "0.1.0", DryRun: true})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	assertEqual(t, result.Written, false)
	assertEqual(t, result.ConfigWritten, true)
	assertEqual(t, fileExists(output), false)
	assertEqual(t, fileExists(dir), false)
	assertEqual(t, fileExists(config), false)
}

func TestGenerateKeepsZeroInitialVersion(t *testing.T) {
	_, dir, output, config := makeInitRoot(t)
	if _, err := Init(InitOptions{Output: output, Dir: dir, Config: config, Version: "0.1.0", DryRun: false}); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	writeFile(t, filepath.Join(dir, "breaking.md"), "## Breaking Changes\n\n- Overhaul\n")

	result, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: testOrder, Bump: testBump})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, result.Version, "0.1.0")
	assertEqual(t, result.Previous, "")
	assertEqual(t, readFile(t, output), "# 0.1.0 - UNRELEASED\n\n## Breaking Changes\n\n- Overhaul\n")
}

func TestInitZeroConfigMakesBreakingChangesBumpMinor(t *testing.T) {
	_, dir, output, config := makeInitRoot(t)
	if _, err := Init(InitOptions{Output: output, Dir: dir, Config: config, Version: "0.1.0", DryRun: false}); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	loaded, err := ReadConfig(config)
	if err != nil {
		t.Fatalf("ReadConfig returned error: %v", err)
	}

	writeFile(t, filepath.Join(dir, "breaking.md"), "## Breaking Changes\n\n- Overhaul\n")
	if _, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: loaded.Order(), Bump: loaded.Bumps()}); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	released, err := Promote(PromoteOptions{Output: output, Dir: dir, DryRun: false, Channel: ChannelStable, Order: loaded.Order()})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	assertEqual(t, released, PromoteResult{Version: "0.1.0", Written: true})

	writeFile(t, filepath.Join(dir, "breaking.md"), "## Breaking Changes\n\n- Another overhaul\n")
	next, err := Generate(GenerateOptions{Dir: dir, Output: output, Clear: true, DryRun: false, Order: loaded.Order(), Bump: loaded.Bumps()})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	assertEqual(t, next.Version, "0.2.0")
	assertEqual(t, next.Previous, "0.1.0")
}
