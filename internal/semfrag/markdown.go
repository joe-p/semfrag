package semfrag

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// SectionType is how a section's fragment content is rendered.
type SectionType string

const (
	SectionList SectionType = "list"
	SectionRaw  SectionType = "raw"
)

// IsSectionType reports whether value is a known section type.
func IsSectionType(value string) bool {
	return value == string(SectionList) || value == string(SectionRaw)
}

// SectionTypes overrides the type of sections by title.
type SectionTypes map[string]SectionType

// Section is a single "## <title>" block. List sections keep their content in
// Lines; raw sections keep it verbatim in Body. An empty Type means list.
type Section struct {
	Title string
	Type  SectionType
	Lines []string
	Body  string
}

// Fragment is a changelog fragment file.
type Fragment struct {
	Name    string
	Content string
}

// UnreleasedMarker appears in the heading of an unreleased section.
const UnreleasedMarker = "UNRELEASED"

// VersionBlock is one "# <version>" section of a changelog.
type VersionBlock struct {
	Version    string
	Unreleased bool
	Sections   []Section
	Raw        string
}

// ChangelogDocument is a parsed changelog: the preamble before the first
// version heading and the version blocks that follow.
type ChangelogDocument struct {
	Preamble string
	Blocks   []VersionBlock
}

var (
	sectionRe           = regexp.MustCompile(`^##\s+(.*\S)\s*$`)
	versionHeadingRe    = regexp.MustCompile(`^#\s+(.*\S)\s*$`)
	unreleasedHeadingRe = regexp.MustCompile(`(?i)^(.*?)\s*-\s*UNRELEASED\s*$`)
	datedHeadingRe      = regexp.MustCompile(`^(.*?)\s+-\s+.*\S\s*$`)
	rawHeadingRe        = regexp.MustCompile(`^#{1,2}(\s|$)`)
	fenceStartRe        = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	fenceMarkerRe       = regexp.MustCompile("^\\s*(`{3,}|~{3,})")
	listItemRe          = regexp.MustCompile(`^( *)(?:[-+*]|\d+[.)])\s+`)
	versionStartRe      = regexp.MustCompile(`^v?\d`)
)

func splitLines(content string) []string {
	return strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
}

// fenceScanner tracks fenced code blocks so headings inside them are ignored.
type fenceScanner struct {
	marker string
}

// scan updates the state for line and reports whether line is a fence
// delimiter and whether the scanner was inside a fence before processing it.
func (f *fenceScanner) scan(line string, markerRe *regexp.Regexp) (isMarker, inFence bool) {
	match := markerRe.FindStringSubmatch(line)
	inFence = f.marker != ""
	if match == nil {
		return false, inFence
	}
	marker := match[1]
	switch {
	case f.marker == "":
		f.marker = marker
	case marker[0] == f.marker[0] && len(marker) >= len(f.marker) && strings.TrimSpace(line) == marker:
		f.marker = ""
	}
	return true, inFence
}

// ParseFragment splits fragment content into sections. types selects raw
// rendering for the named sections.
func ParseFragment(content string, types SectionTypes) ([]Section, error) {
	var (
		sections []*Section
		current  *Section
		rawLines []string
		hasRaw   bool
	)
	flushRaw := func() {
		if current != nil && hasRaw {
			current.Body = strings.Trim(strings.Join(rawLines, "\n"), "\n")
		}
	}

	var fence fenceScanner
	for index, line := range splitLines(content) {
		isMarker, inFence := fence.scan(line, fenceStartRe)
		var match []string
		if !isMarker && !inFence {
			match = sectionRe.FindStringSubmatch(line)
		}
		if match != nil {
			flushRaw()
			title := match[1]
			current = &Section{Title: title, Lines: []string{}}
			if types[title] == SectionRaw {
				current.Type = SectionRaw
				rawLines = []string{}
				hasRaw = true
			} else {
				rawLines = nil
				hasRaw = false
			}
			sections = append(sections, current)
			continue
		}
		if current == nil {
			if strings.TrimSpace(line) != "" {
				return nil, fmt.Errorf("line %d: expected a ## section heading before content", index+1)
			}
			continue
		}
		if hasRaw {
			if !inFence && rawHeadingRe.MatchString(line) {
				return nil, fmt.Errorf("raw section %q may not contain a level-1 or level-2 heading: %q, use ### or deeper", "## "+current.Title, strings.TrimSpace(line))
			}
			rawLines = append(rawLines, line)
			continue
		}
		if !inFence && rawHeadingRe.MatchString(line) {
			return nil, fmt.Errorf("line %d: unexpected heading %q, use ### or deeper", index+1, line)
		}
		current.Lines = append(current.Lines, strings.TrimRightFunc(line, unicode.IsSpace))
	}

	flushRaw()
	result := make([]Section, len(sections))
	for i, section := range sections {
		section.Lines = trimBlankLines(section.Lines)
		result[i] = *section
	}
	return result, nil
}

// trimBlankLines drops leading and trailing empty lines.
func trimBlankLines(lines []string) []string {
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if lines == nil {
		return []string{}
	}
	return lines
}

// listItems splits lines into markdown list items, keeping continuation lines
// with their item and ignoring bullets inside fenced code blocks.
func listItems(lines []string) [][]string {
	var fence fenceScanner
	matches := make([][]string, len(lines))
	for i, line := range lines {
		isMarker, inFence := fence.scan(line, fenceMarkerRe)
		if isMarker || inFence {
			continue
		}
		matches[i] = listItemRe.FindStringSubmatch(line)
	}

	indent := math.MaxInt
	for _, match := range matches {
		if match != nil && len(match[1]) < indent {
			indent = len(match[1])
		}
	}

	items := [][]string{}
	for i, line := range lines {
		if len(items) == 0 || (matches[i] != nil && len(matches[i][1]) == indent) {
			items = append(items, []string{})
		}
		items[len(items)-1] = append(items[len(items)-1], line)
	}
	return items
}

// ParseChangelog parses markdown into version blocks.
func ParseChangelog(markdown string, types SectionTypes) ([]VersionBlock, error) {
	document, err := ParseChangelogDocument(markdown, types)
	return document.Blocks, err
}

// ParseChangelogDocument parses a changelog into its preamble and version
// blocks.
func ParseChangelogDocument(markdown string, types SectionTypes) (ChangelogDocument, error) {
	lines := splitLines(markdown)
	document := ChangelogDocument{Blocks: []VersionBlock{}}
	start := -1
	version := ""
	unreleased := false
	hasHeading := false
	firstVersion := -1

	push := func(end int) error {
		if !hasHeading || start < 0 {
			return nil
		}
		sections, err := ParseFragment(strings.Join(lines[start+1:end], "\n"), types)
		if err != nil {
			return err
		}
		document.Blocks = append(document.Blocks, VersionBlock{
			Version:    version,
			Unreleased: unreleased,
			Sections:   sections,
			Raw:        strings.TrimRightFunc(strings.Join(lines[start:end], "\n"), unicode.IsSpace),
		})
		return nil
	}

	var fence fenceScanner
	for index, line := range lines {
		if isMarker, inFence := fence.scan(line, fenceStartRe); isMarker || inFence {
			continue
		}
		match := versionHeadingRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		title := match[1]
		unreleasedMatch := unreleasedHeadingRe.FindStringSubmatch(title)
		if unreleasedMatch == nil && !versionStartRe.MatchString(title) {
			if start < 0 {
				continue
			}
			return ChangelogDocument{}, fmt.Errorf("unexpected level-1 heading %q", title)
		}
		blockVersion := releaseVersion(title)
		if unreleasedMatch != nil {
			blockVersion = strings.TrimSpace(unreleasedMatch[1])
		}
		if _, err := ParseVersion(blockVersion); err != nil {
			return ChangelogDocument{}, err
		}
		if err := push(index); err != nil {
			return ChangelogDocument{}, err
		}
		if firstVersion < 0 {
			firstVersion = index
		}
		version, unreleased, hasHeading, start = blockVersion, unreleasedMatch != nil, true, index
	}

	if err := push(len(lines)); err != nil {
		return ChangelogDocument{}, err
	}
	preambleEnd := firstVersion
	if firstVersion < 0 {
		preambleEnd = len(lines)
	}
	document.Preamble = strings.Join(lines[:preambleEnd], "\n")
	return document, nil
}

// releaseVersion extracts the version from a released heading, dropping any
// metadata such as a release date after " - ".
func releaseVersion(title string) string {
	if dated := datedHeadingRe.FindStringSubmatch(title); dated != nil {
		return strings.TrimSpace(dated[1])
	}
	return title
}

func assertKnownSection(title string, order []string) error {
	if slices.Contains(order, title) {
		return nil
	}
	quoted := make([]string, len(order))
	for i, section := range order {
		quoted[i] = fmt.Sprintf("%q", "## "+section)
	}
	return fmt.Errorf("unknown changelog section %q, expected one of: %s", "## "+title, strings.Join(quoted, ", "))
}

// RenderChangelog renders sections under a "# <title>" heading.
func RenderChangelog(sections []Section, title string) string {
	parts := []string{"# " + title}
	for _, section := range sections {
		if section.Type == SectionRaw {
			parts = append(parts, "## "+section.Title, section.Body)
		} else {
			parts = append(parts, "## "+section.Title, strings.Join(section.Lines, "\n"))
		}
	}
	return strings.Join(parts, "\n\n") + "\n"
}

// PrependChangelog places entry before existing, trimming existing's edges.
func PrependChangelog(existing, entry string) string {
	trimmed := strings.TrimSpace(existing)
	if trimmed == "" {
		return entry
	}
	return entry + "\n" + trimmed + "\n"
}
