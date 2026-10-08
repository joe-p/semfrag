package semfrag

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const DefaultConfigFile = "semfrag.json"

type SectionConfig struct {
	Title string      `json:"title"`
	Bump  BumpLevel   `json:"bump,omitempty"`
	Type  SectionType `json:"type,omitempty"`
}

type ChangelogConfig struct {
	Sections []SectionConfig `json:"sections"`
}

func DefaultConfig(initialVersion string) (ChangelogConfig, error) {
	parsed, err := ParseVersion(initialVersion)
	if err != nil {
		return ChangelogConfig{}, err
	}
	bump := BumpMajor
	if parsed.Major == 0 {
		bump = BumpMinor
	}
	return ChangelogConfig{
		Sections: []SectionConfig{
			{Title: "Breaking Changes", Bump: bump},
			{Title: "Features", Bump: BumpMinor},
			{Title: "Fixes", Bump: BumpPatch},
		},
	}, nil
}

func SerializeConfig(config ChangelogConfig) (string, error) {
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded) + "\n", nil
}

func SectionOrder(config ChangelogConfig) []string {
	order := make([]string, len(config.Sections))
	for i, section := range config.Sections {
		order[i] = section.Title
	}
	return order
}

func SectionBumps(config ChangelogConfig) map[string]BumpLevel {
	bumps := map[string]BumpLevel{}
	for _, section := range config.Sections {
		if section.Bump != "" {
			bumps[section.Title] = section.Bump
		}
	}
	return bumps
}

func SectionTypes(config ChangelogConfig) SectionTypeMap {
	types := SectionTypeMap{}
	for _, section := range config.Sections {
		if section.Type != "" {
			types[section.Title] = section.Type
		}
	}
	return types
}

func jsonStringify(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}

func ParseConfig(raw, source string) (ChangelogConfig, error) {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return ChangelogConfig{}, fmt.Errorf("%s: invalid JSON (%s)", source, err.Error())
	}

	object, ok := parsed.(map[string]any)
	if !ok {
		return ChangelogConfig{}, fmt.Errorf("%s: expected a JSON object", source)
	}

	sections, ok := object["sections"].([]any)
	if !ok || len(sections) == 0 {
		return ChangelogConfig{}, fmt.Errorf(`%s: "sections" must be a non-empty array of section objects`, source)
	}

	titles := []string{}
	result := []SectionConfig{}

	for index, entry := range sections {
		sectionObject, ok := entry.(map[string]any)
		if !ok {
			return ChangelogConfig{}, fmt.Errorf("%s: sections[%d] must be an object", source, index)
		}

		title, ok := sectionObject["title"].(string)
		if !ok || strings.TrimSpace(title) == "" {
			return ChangelogConfig{}, fmt.Errorf("%s: sections[%d].title must be a non-empty string", source, index)
		}
		if contains(titles, title) {
			return ChangelogConfig{}, fmt.Errorf(`%s: duplicate section %q`, source, title)
		}
		titles = append(titles, title)

		section := SectionConfig{Title: title}
		if bump, present := sectionObject["bump"]; present {
			normalized := bump
			if text, ok := bump.(string); ok {
				normalized = strings.ToUpper(text)
			}
			level, ok := normalized.(string)
			if !ok || !IsBumpLevel(level) {
				return ChangelogConfig{}, fmt.Errorf(`%s: invalid bump level for %q: %s. Expected MAJOR, MINOR or PATCH.`, source, title, jsonStringify(bump))
			}
			section.Bump = BumpLevel(level)
		}
		if sectionType, present := sectionObject["type"]; present {
			normalized := sectionType
			if text, ok := sectionType.(string); ok {
				normalized = strings.ToLower(text)
			}
			text, ok := normalized.(string)
			if !ok || !IsSectionType(text) {
				return ChangelogConfig{}, fmt.Errorf(`%s: invalid section type for %q: %s. Expected list or raw.`, source, title, jsonStringify(sectionType))
			}
			section.Type = SectionType(text)
		}

		result = append(result, section)
	}

	return ChangelogConfig{Sections: result}, nil
}

func ReadConfig(configPath string) (ChangelogConfig, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return ChangelogConfig{}, err
	}
	return ParseConfig(string(raw), configPath)
}

func LoadConfig(configPath string) (*ChangelogConfig, error) {
	if configPath != "" {
		if !fileExists(configPath) {
			return nil, fmt.Errorf("Config file not found: %s", configPath)
		}
		config, err := ReadConfig(configPath)
		if err != nil {
			return nil, err
		}
		return &config, nil
	}

	if !fileExists(DefaultConfigFile) {
		return nil, nil
	}
	config, err := ReadConfig(DefaultConfigFile)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
