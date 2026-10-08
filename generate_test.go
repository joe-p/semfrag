package semfrag

import "testing"

func TestParseFragmentSplitsSections(t *testing.T) {
	sections, err := ParseFragment("## Fixes\n\n- a\n- b\n\n## Features\n\n- c\n", nil)
	if err != nil {
		t.Fatalf("ParseFragment returned error: %v", err)
	}
	assertEqual(t, sections, []Section{
		{Title: "Fixes", Lines: []string{"- a", "- b"}},
		{Title: "Features", Lines: []string{"- c"}},
	})
}

func TestParseFragmentRejectsContentBeforeHeading(t *testing.T) {
	_, err := ParseFragment("intro\n\n## Fixes\n\n- a\n", nil)
	assertErrorContains(t, err, "Line 1: expected a ## section heading")
}

func TestMergingKeepsMultilineItemsAndDeduplicates(t *testing.T) {
	fragments := []Fragment{
		{Name: "a.md", Content: "## Fixes\n- Fix A\n  - Details\n\n  More details.\n"},
		{Name: "b.md", Content: "## Fixes\n- Fix B\n  - Details\n"},
		{Name: "c.md", Content: "## Fixes\n- Fix A\n  - Details\n\n  More details.\n"},
	}
	sections, err := MergeFragments(fragments, nil, nil)
	if err != nil {
		t.Fatalf("MergeFragments returned error: %v", err)
	}
	assertEqual(t, RenderChangelog(sections, "Unreleased"), "# Unreleased\n\n## Fixes\n\n- Fix A\n  - Details\n\n  More details.\n- Fix B\n  - Details\n")
}

func TestFragmentValidationReportsFilenames(t *testing.T) {
	for _, content := range []string{"", "## Fixes\n", "Missing a heading"} {
		_, err := MergeFragments([]Fragment{{Name: "bad.md", Content: content}}, nil, nil)
		assertErrorContains(t, err, "bad.md:")
	}
}

func TestListDeduplicationPreservesFencedExamples(t *testing.T) {
	content := "## Fixes\n\n- Example\n  ```md\n- Sample\n- Sample\n  ```\n\n- Another fix\n"
	sections, err := MergeFragments([]Fragment{{Name: "a.md", Content: content}, {Name: "b.md", Content: content}}, nil, nil)
	if err != nil {
		t.Fatalf("MergeFragments returned error: %v", err)
	}
	assertEqual(t, RenderChangelog(sections, "Unreleased"), "# Unreleased\n\n"+content)
}

func TestChangelogTitleAndIntroductionAreNotBlocks(t *testing.T) {
	blocks, err := ParseChangelog("# Changelog\n\nProject notes.\n\n# 1.0.0\n\n## Fixes\n- fix\n", nil)
	if err != nil {
		t.Fatalf("ParseChangelog returned error: %v", err)
	}
	assertEqual(t, len(blocks), 1)
	assertEqual(t, blocks[0].Version, "1.0.0")
}

func TestFencedHeadingsRemainRawContent(t *testing.T) {
	content := "# 1.0.0\n\n## Details\n\n```md\n# Example\n## Example section\n```\n"
	blocks, err := ParseChangelog(content, SectionTypeMap{"Details": SectionTypeRaw})
	if err != nil {
		t.Fatalf("ParseChangelog returned error: %v", err)
	}
	assertEqual(t, len(blocks), 1)
	assertEqual(t, blocks[0].Sections[0].Body, "```md\n# Example\n## Example section\n```")
}

func TestParseFragmentPreservesRawMarkdown(t *testing.T) {
	sections, err := ParseFragment(
		"## Details\n\nSome intro.\n\n### Nested\n\n- item\n\n```js\nconst a = 1;\n```\n",
		SectionTypeMap{"Details": SectionTypeRaw},
	)
	if err != nil {
		t.Fatalf("ParseFragment returned error: %v", err)
	}
	assertEqual(t, sections, []Section{
		{
			Title: "Details",
			Type:  SectionTypeRaw,
			Lines: []string{},
			Body:  "Some intro.\n\n### Nested\n\n- item\n\n```js\nconst a = 1;\n```",
		},
	})
}

func TestParseFragmentRejectsHeadingsInRawSections(t *testing.T) {
	_, err := ParseFragment("## Details\n\n# Nope\n", SectionTypeMap{"Details": SectionTypeRaw})
	assertErrorContains(t, err, "may not contain a level-1 or level-2 heading")
}

func TestMergeFragmentsGroupsByTitleInFirstSeenOrder(t *testing.T) {
	merged, err := MergeFragments([]Fragment{
		{Name: "fix-01.md", Content: "## Fixes\n\n- fix #01\n"},
		{Name: "fix-02.md", Content: "## Fixes\n\n- fix #02\n"},
		{Name: "feat-01.md", Content: "## Features\n\n- Added a new feature!\n"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("MergeFragments returned error: %v", err)
	}
	assertEqual(t, merged, []Section{
		{Title: "Features", Lines: []string{"- Added a new feature!"}},
		{Title: "Fixes", Lines: []string{"- fix #01", "- fix #02"}},
	})
}

func TestMergeFragmentsDropsSectionsWithNoEntries(t *testing.T) {
	merged, err := MergeFragments([]Fragment{
		{Name: "a.md", Content: "## Fixes\n\n- fix #01\n\n## Features\n"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("MergeFragments returned error: %v", err)
	}
	assertEqual(t, merged, []Section{{Title: "Fixes", Lines: []string{"- fix #01"}}})
}

func TestMergeFragmentsFollowsConfiguredOrder(t *testing.T) {
	fragments := []Fragment{
		{Name: "fix-01.md", Content: "## Fixes\n\n- fix #01\n"},
		{Name: "feat-01.md", Content: "## Features\n\n- Added a new feature!\n"},
	}
	first, err := MergeFragments(fragments, []string{"Fixes", "Features"}, nil)
	if err != nil {
		t.Fatalf("MergeFragments returned error: %v", err)
	}
	assertEqual(t, first, []Section{
		{Title: "Fixes", Lines: []string{"- fix #01"}},
		{Title: "Features", Lines: []string{"- Added a new feature!"}},
	})
	second, err := MergeFragments(fragments, []string{"Features", "Fixes"}, nil)
	if err != nil {
		t.Fatalf("MergeFragments returned error: %v", err)
	}
	assertEqual(t, second, []Section{
		{Title: "Features", Lines: []string{"- Added a new feature!"}},
		{Title: "Fixes", Lines: []string{"- fix #01"}},
	})
}

func TestMergeFragmentsThrowsOnUnknownSection(t *testing.T) {
	_, err := MergeFragments([]Fragment{{Name: "a.md", Content: "## Chores\n\n- chore\n"}}, []string{"Fixes", "Features"}, nil)
	assertErrorContains(t, err, `Unknown changelog section "## Chores"`)
}

func TestRenderChangelogProducesExpectedMarkdown(t *testing.T) {
	output := RenderChangelog([]Section{
		{Title: "Fixes", Lines: []string{"- fix #01", "- fix #02"}},
		{Title: "Features", Lines: []string{"- Added a new feature!"}},
	}, "Unreleased")
	assertEqual(t, output, "# Unreleased\n\n## Fixes\n\n- fix #01\n- fix #02\n\n## Features\n\n- Added a new feature!\n")
}

func TestPrependChangelogKeepsExistingContent(t *testing.T) {
	existing := "# Changelog\n\n## 1.0.0\n\n- old\n"
	result := PrependChangelog(existing, "# Unreleased\n\n## Fixes\n\n- new\n")
	assertEqual(t, result, "# Unreleased\n\n## Fixes\n\n- new\n\n# Changelog\n\n## 1.0.0\n\n- old\n")
}

func TestMergeSectionsGroupsByTitleAndDropsDuplicateLines(t *testing.T) {
	merged, err := MergeSections([][]Section{
		{{Title: "Fixes", Lines: []string{"- fix #01"}}},
		{{Title: "Fixes", Lines: []string{"- fix #01", "- fix #02"}}},
	}, nil, nil)
	if err != nil {
		t.Fatalf("MergeSections returned error: %v", err)
	}
	assertEqual(t, merged, []Section{{Title: "Fixes", Lines: []string{"- fix #01", "- fix #02"}}})
}

func TestMergeSectionsConcatenatesRawBodies(t *testing.T) {
	merged, err := MergeSections([][]Section{
		{{Title: "Details", Type: SectionTypeRaw, Lines: []string{}, Body: "one"}},
		{{Title: "Details", Type: SectionTypeRaw, Lines: []string{}, Body: "two"}},
		{{Title: "Details", Type: SectionTypeRaw, Lines: []string{}, Body: "one"}},
	}, nil, nil)
	if err != nil {
		t.Fatalf("MergeSections returned error: %v", err)
	}
	assertEqual(t, merged, []Section{{Title: "Details", Type: SectionTypeRaw, Lines: []string{}, Body: "one\n\ntwo"}})
}

func TestRenderChangelogRendersRawSectionsVerbatim(t *testing.T) {
	output := RenderChangelog(
		[]Section{{Title: "Details", Type: SectionTypeRaw, Lines: []string{}, Body: "Some intro.\n\n### Nested\n\n- item"}},
		"Unreleased",
	)
	assertEqual(t, output, "# Unreleased\n\n## Details\n\nSome intro.\n\n### Nested\n\n- item\n")
}

func TestParseChangelogSplitsReleasedAndUnreleased(t *testing.T) {
	blocks, err := ParseChangelog("# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- new\n\n# 1.0.0\n\n## Features\n\n- old\n", nil)
	if err != nil {
		t.Fatalf("ParseChangelog returned error: %v", err)
	}
	assertEqual(t, blocks, []VersionBlock{
		{
			Version:    "1.1.0",
			Unreleased: true,
			Sections:   []Section{{Title: "Fixes", Lines: []string{"- new"}}},
			Raw:        "# 1.1.0 - UNRELEASED\n\n## Fixes\n\n- new",
		},
		{
			Version:    "1.0.0",
			Unreleased: false,
			Sections:   []Section{{Title: "Features", Lines: []string{"- old"}}},
			Raw:        "# 1.0.0\n\n## Features\n\n- old",
		},
	})
}

func TestParseChangelogReadsDatedHeading(t *testing.T) {
	blocks, err := ParseChangelog(
		"# 1.1.0 - January 1st, 2026\n\n## Fixes\n\n- new\n\n# 1.0.1-alpha.1 - December 31st, 2025\n\n## Fixes\n\n- preview\n",
		nil,
	)
	if err != nil {
		t.Fatalf("ParseChangelog returned error: %v", err)
	}
	assertEqual(t, blocks[0].Version, "1.1.0")
	assertEqual(t, blocks[0].Unreleased, false)
	assertEqual(t, blocks[1].Version, "1.0.1-alpha.1")
	assertEqual(t, blocks[1].Unreleased, false)
}

func TestParseChangelogPreservesRawBlockText(t *testing.T) {
	markdown := "# 1.0.0\n\n## Features\n\n- old\n\n# 0.9.0\n\n## Fixes\n\n- older\n"
	blocks, err := ParseChangelog(markdown, nil)
	if err != nil {
		t.Fatalf("ParseChangelog returned error: %v", err)
	}
	assertEqual(t, blocks[0].Raw, "# 1.0.0\n\n## Features\n\n- old")
}

func TestParseChangelogPreservesRawSectionsWithTypes(t *testing.T) {
	blocks, err := ParseChangelog("# 1.0.0\n\n## Details\n\nIntro.\n\n### Nested\n\n- x\n", SectionTypeMap{"Details": SectionTypeRaw})
	if err != nil {
		t.Fatalf("ParseChangelog returned error: %v", err)
	}
	assertEqual(t, blocks[0].Sections, []Section{
		{Title: "Details", Type: SectionTypeRaw, Lines: []string{}, Body: "Intro.\n\n### Nested\n\n- x"},
	})
}
