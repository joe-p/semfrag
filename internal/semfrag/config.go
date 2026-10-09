package semfrag

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DefaultConfigFile is the config file read when none is given.
const DefaultConfigFile = "semfrag.json"

// SectionConfig is one entry of a changelog config.
type SectionConfig struct {
	Title string      `json:"title"`
	Bump  BumpLevel   `json:"bump,omitempty"`
	Type  SectionType `json:"type,omitempty"`
}

// ChangelogConfig lists the allowed sections in display order.
type ChangelogConfig struct {
	Sections []SectionConfig `json:"sections"`
}

// Order returns the section titles in display order.
func (c ChangelogConfig) Order() []string {
	order := make([]string, len(c.Sections))
	for i, section := range c.Sections {
		order[i] = section.Title
	}
	return order
}

// Bumps returns the bump level of each section that declares one.
func (c ChangelogConfig) Bumps() map[string]BumpLevel {
	bumps := map[string]BumpLevel{}
	for _, section := range c.Sections {
		if section.Bump != "" {
			bumps[section.Title] = section.Bump
		}
	}
	return bumps
}

// Types returns the type override of each section that declares one.
func (c ChangelogConfig) Types() SectionTypes {
	types := SectionTypes{}
	for _, section := range c.Sections {
		if section.Type != "" {
			types[section.Title] = section.Type
		}
	}
	return types
}

// DefaultConfig returns the default sections for a starting version. A 0.y.z
// start makes Breaking Changes a MINOR bump.
func DefaultConfig(initialVersion string) (ChangelogConfig, error) {
	parsed, err := ParseVersion(initialVersion)
	if err != nil {
		return ChangelogConfig{}, err
	}
	breaking := BumpMajor
	if parsed.Major == 0 {
		breaking = BumpMinor
	}
	return ChangelogConfig{Sections: []SectionConfig{
		{Title: "Breaking Changes", Bump: breaking},
		{Title: "Features", Bump: BumpMinor},
		{Title: "Fixes", Bump: BumpPatch},
	}}, nil
}

// SerializeConfig renders config as indented JSON with a trailing newline.
func SerializeConfig(config ChangelogConfig) (string, error) {
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded) + "\n", nil
}

// ParseConfig validates raw JSON and returns the config, reporting errors
// against source.
func ParseConfig(raw, source string) (ChangelogConfig, error) {
	var document json.RawMessage
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		return ChangelogConfig{}, fmt.Errorf("%s: invalid JSON: %w", source, err)
	}

	if bytes.TrimSpace(document)[0] != '{' {
		return ChangelogConfig{}, fmt.Errorf("%s: expected a JSON object", source)
	}
	var object struct {
		Sections []json.RawMessage `json:"sections"`
	}
	if err := json.Unmarshal(document, &object); err != nil || len(object.Sections) == 0 {
		return ChangelogConfig{}, fmt.Errorf(`%s: "sections" must be a non-empty array of section objects`, source)
	}

	titles := make(map[string]bool, len(object.Sections))
	result := make([]SectionConfig, 0, len(object.Sections))

	for index, entry := range object.Sections {
		var sectionObject struct {
			Title json.RawMessage `json:"title"`
			Bump  json.RawMessage `json:"bump"`
			Type  json.RawMessage `json:"type"`
		}
		if bytes.TrimSpace(entry)[0] != '{' {
			return ChangelogConfig{}, fmt.Errorf("%s: sections[%d] must be an object", source, index)
		}
		if err := json.Unmarshal(entry, &sectionObject); err != nil {
			return ChangelogConfig{}, fmt.Errorf("%s: sections[%d]: %w", source, index, err)
		}

		var title string
		if err := json.Unmarshal(sectionObject.Title, &title); err != nil || strings.TrimSpace(title) == "" {
			return ChangelogConfig{}, fmt.Errorf("%s: sections[%d].title must be a non-empty string", source, index)
		}
		if titles[title] {
			return ChangelogConfig{}, fmt.Errorf("%s: duplicate section %q", source, title)
		}
		titles[title] = true

		section := SectionConfig{Title: title}
		if bump := sectionObject.Bump; bump != nil {
			var level string
			if err := json.Unmarshal(bump, &level); err != nil || !IsBumpLevel(strings.ToUpper(level)) {
				return ChangelogConfig{}, fmt.Errorf("%s: invalid bump level for %q: %s, expected MAJOR, MINOR or PATCH", source, title, bump)
			}
			section.Bump = BumpLevel(strings.ToUpper(level))
		}
		if sectionType := sectionObject.Type; sectionType != nil {
			var kind string
			if err := json.Unmarshal(sectionType, &kind); err != nil || !IsSectionType(strings.ToLower(kind)) {
				return ChangelogConfig{}, fmt.Errorf("%s: invalid section type for %q: %s, expected list or raw", source, title, sectionType)
			}
			section.Type = SectionType(strings.ToLower(kind))
		}
		result = append(result, section)
	}

	return ChangelogConfig{Sections: result}, nil
}

// ReadConfig reads and validates a config file.
func ReadConfig(path string) (ChangelogConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ChangelogConfig{}, err
	}
	return ParseConfig(string(raw), path)
}

// LoadConfig reads path when given, otherwise it searches the current directory
// and each of its parents for DefaultConfigFile, stopping at the git root (a
// directory containing .git) or the filesystem root. It returns nil when no
// config is found.
func LoadConfig(path string) (*ChangelogConfig, error) {
	explicit := path != ""
	if path == "" {
		found, err := findDefaultConfig()
		if err != nil {
			return nil, err
		}
		if found == "" {
			return nil, nil
		}
		path = found
	}
	config, err := ReadConfig(path)
	if errors.Is(err, fs.ErrNotExist) {
		if explicit {
			return nil, fmt.Errorf("config file not found: %s: %w", path, err)
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// findDefaultConfig returns the path of the first DefaultConfigFile found in the
// current directory or one of its parents, stopping after the git root or at the
// filesystem root. It returns an empty string when none is found.
func findDefaultConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, DefaultConfigFile)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return "", nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}
