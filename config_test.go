package semfrag

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigReadsSectionsAndNormalizesBumps(t *testing.T) {
	config, err := ParseConfig(
		`{"sections":[{"title":"Fixes","bump":"patch"},{"title":"Features","bump":"MINOR"},{"title":"Docs"}]}`,
		"test",
	)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	assertEqual(t, config, ChangelogConfig{Sections: []SectionConfig{
		{Title: "Fixes", Bump: BumpPatch},
		{Title: "Features", Bump: BumpMinor},
		{Title: "Docs"},
	}})
	assertEqual(t, config.Order(), []string{"Fixes", "Features", "Docs"})
	assertEqual(t, config.Bumps(), map[string]BumpLevel{"Fixes": BumpPatch, "Features": BumpMinor})
}

func TestParseConfigReadsAndNormalizesSectionTypes(t *testing.T) {
	config, err := ParseConfig(
		`{"sections":[{"title":"Fixes","bump":"PATCH"},{"title":"Details","type":"RAW"}]}`,
		"test",
	)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	assertEqual(t, config, ChangelogConfig{Sections: []SectionConfig{
		{Title: "Fixes", Bump: BumpPatch},
		{Title: "Details", Type: SectionRaw},
	}})
	assertEqual(t, config.Types(), SectionTypes{"Details": SectionRaw})
}

func TestParseConfigRejectsInvalidBumpLevels(t *testing.T) {
	_, err := ParseConfig(`{"sections":[{"title":"Fixes","bump":"HUGE"}]}`, "test")
	assertErrorContains(t, err, `invalid bump level for "Fixes"`)
}

func TestParseConfigRejectsInvalidSectionTypes(t *testing.T) {
	_, err := ParseConfig(`{"sections":[{"title":"Details","type":"fancy"}]}`, "test")
	assertErrorContains(t, err, `invalid section type for "Details"`)
}

func TestParseConfigRejectsInvalidJSON(t *testing.T) {
	_, err := ParseConfig("{oops", "test")
	assertErrorContains(t, err, "test: invalid JSON")
}

func TestParseConfigRejectsMissingOrEmptySections(t *testing.T) {
	_, err := ParseConfig("{}", "test")
	assertErrorContains(t, err, `"sections" must be a non-empty array`)
	_, err = ParseConfig(`{"sections":[]}`, "test")
	assertErrorContains(t, err, `"sections" must be a non-empty array`)
}

func TestParseConfigRejectsNonObjectSections(t *testing.T) {
	_, err := ParseConfig(`{"sections":["Fixes"]}`, "test")
	assertErrorContains(t, err, "sections[0] must be an object")
}

func TestParseConfigRejectsMissingOrEmptyTitles(t *testing.T) {
	_, err := ParseConfig(`{"sections":[{"bump":"PATCH"}]}`, "test")
	assertErrorContains(t, err, ".title must be a non-empty string")
	_, err = ParseConfig(`{"sections":[{"title":"  "}]}`, "test")
	assertErrorContains(t, err, ".title must be a non-empty string")
}

func TestParseConfigRejectsDuplicateSections(t *testing.T) {
	_, err := ParseConfig(`{"sections":[{"title":"Fixes"},{"title":"Fixes"}]}`, "test")
	assertErrorContains(t, err, `duplicate section "Fixes"`)
}

func TestLoadConfigReportsMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	assertErrorContains(t, err, "config file not found")
}

func TestLoadConfigReadsFileFromDisk(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "semfrag.json")
	if err := os.WriteFile(file, []byte(`{"sections":[{"title":"Fixes","bump":"PATCH"}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	config, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	assertEqual(t, *config, ChangelogConfig{Sections: []SectionConfig{{Title: "Fixes", Bump: BumpPatch}}})
}
