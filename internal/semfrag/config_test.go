package semfrag

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigRejectsInvalidFieldShapes(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  string
		want string
	}{
		{"null document", `null`, "expected a JSON object"},
		{"array document", `[]`, "expected a JSON object"},
		{"null sections", `{"sections":null}`, `"sections" must be a non-empty array`},
		{"object sections", `{"sections":{}}`, `"sections" must be a non-empty array`},
		{"null section", `{"sections":[null]}`, "sections[0] must be an object"},
		{"null title", `{"sections":[{"title":null}]}`, ".title must be a non-empty string"},
		{"numeric title", `{"sections":[{"title":1}]}`, ".title must be a non-empty string"},
		{"null bump", `{"sections":[{"title":"Fixes","bump":null}]}`, "invalid bump level"},
		{"numeric bump", `{"sections":[{"title":"Fixes","bump":1}]}`, "invalid bump level"},
		{"null type", `{"sections":[{"title":"Fixes","type":null}]}`, "invalid section type"},
		{"object type", `{"sections":[{"title":"Fixes","type":{}}]}`, "invalid section type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseConfig(test.raw, "test")
			assertErrorContains(t, err, test.want)
		})
	}
}

func TestLoadConfigPreservesFilesystemErrors(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(filepath.Join(parent, "semfrag.json"))
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected filesystem error other than not-exist, got %v", err)
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("expected preserved PathError, got %v", err)
	}
}

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
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected wrapped syntax error, got %v", err)
	}
}

func TestLoadConfigUsesOptionalDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	config, err := LoadConfig("")
	if err != nil || config != nil {
		t.Fatalf("expected no config for missing default, got %v, %v", config, err)
	}
	if err := os.Mkdir(DefaultConfigFile, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = LoadConfig("")
	if err == nil {
		t.Fatal("expected error when the default config is a directory")
	}
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
