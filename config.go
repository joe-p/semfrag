package semfrag

import (
	"encoding/json"
	"fmt"
	"os"
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
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return ChangelogConfig{}, fmt.Errorf("%s: invalid JSON (%s)", source, err)
	}

	object, ok := parsed.(map[string]any)
	if !ok {
		return ChangelogConfig{}, fmt.Errorf("%s: expected a JSON object", source)
	}

	sections, ok := object["sections"].([]any)
	if !ok || len(sections) == 0 {
		return ChangelogConfig{}, fmt.Errorf(`%s: "sections" must be a non-empty array of section objects`, source)
	}

	titles := make([]string, 0, len(sections))
	result := make([]SectionConfig, 0, len(sections))

	for index, entry := range sections {
		sectionObject, ok := entry.(map[string]any)
		if !ok {
			return ChangelogConfig{}, fmt.Errorf("%s: sections[%d] must be an object", source, index)
		}

		title, ok := sectionObject["title"].(string)
		if !ok || strings.TrimSpace(title) == "" {
			return ChangelogConfig{}, fmt.Errorf("%s: sections[%d].title must be a non-empty string", source, index)
		}
		if containsString(titles, title) {
			return ChangelogConfig{}, fmt.Errorf("%s: duplicate section %q", source, title)
		}
		titles = append(titles, title)

		section := SectionConfig{Title: title}
		if bump, present := sectionObject["bump"]; present {
			level, ok := normalizeString(bump, strings.ToUpper)
			if !ok || !IsBumpLevel(level) {
				return ChangelogConfig{}, fmt.Errorf("%s: invalid bump level for %q: %s, expected MAJOR, MINOR or PATCH", source, title, jsonStringify(bump))
			}
			section.Bump = BumpLevel(level)
		}
		if sectionType, present := sectionObject["type"]; present {
			kind, ok := normalizeString(sectionType, strings.ToLower)
			if !ok || !IsSectionType(kind) {
				return ChangelogConfig{}, fmt.Errorf("%s: invalid section type for %q: %s, expected list or raw", source, title, jsonStringify(sectionType))
			}
			section.Type = SectionType(kind)
		}
		result = append(result, section)
	}

	return ChangelogConfig{Sections: result}, nil
}

// normalizeString upper/lower-cases a JSON string value, reporting whether the
// value was a string at all.
func normalizeString(value any, transform func(string) string) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return transform(text), true
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func jsonStringify(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}

// ReadConfig reads and validates a config file.
func ReadConfig(path string) (ChangelogConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ChangelogConfig{}, err
	}
	return ParseConfig(string(raw), path)
}

// LoadConfig reads path when given, otherwise DefaultConfigFile if it exists.
// It returns nil when no config is found.
func LoadConfig(path string) (*ChangelogConfig, error) {
	if path == "" {
		if !fileExists(DefaultConfigFile) {
			return nil, nil
		}
		path = DefaultConfigFile
	} else if !fileExists(path) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}
	config, err := ReadConfig(path)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
