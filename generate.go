package semfrag

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type SectionType string

const (
	SectionTypeList SectionType = "list"
	SectionTypeRaw  SectionType = "raw"
)

var SectionTypesList = []SectionType{SectionTypeList, SectionTypeRaw}

func IsSectionType(value string) bool {
	for _, sectionType := range SectionTypesList {
		if string(sectionType) == value {
			return true
		}
	}
	return false
}

type SectionTypeMap map[string]SectionType

type Section struct {
	Title string
	Type  SectionType
	Lines []string
	Body  string
}

type Fragment struct {
	Name    string
	Content string
}

const UnreleasedMarker = "UNRELEASED"

type VersionBlock struct {
	Version    string
	Unreleased bool
	Sections   []Section
	Raw        string
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

func trimEnd(line string) string {
	return strings.TrimRightFunc(line, unicode.IsSpace)
}

type fenceState struct {
	marker string
}

func (f *fenceState) update(line string, re *regexp.Regexp) (match []string, inFence bool) {
	match = re.FindStringSubmatch(line)
	inFence = f.marker != ""
	if match == nil {
		return match, inFence
	}
	marker := match[1]
	if f.marker == "" {
		f.marker = marker
	} else if marker[0] == f.marker[0] && len(marker) >= len(f.marker) && strings.TrimSpace(line) == marker {
		f.marker = ""
	}
	return match, inFence
}

func ParseFragment(content string, types SectionTypeMap) ([]Section, error) {
	sections := []*Section{}
	var current *Section
	var rawLines []string
	hasRaw := false

	flushRaw := func() {
		if current == nil || !hasRaw {
			return
		}
		current.Body = strings.Trim(strings.Join(rawLines, "\n"), "\n")
	}

	var fence fenceState
	for index, rawLine := range splitLines(content) {
		fenceMatch, inFence := fence.update(rawLine, fenceStartRe)
		var match []string
		if !inFence && fenceMatch == nil {
			match = sectionRe.FindStringSubmatch(rawLine)
		}
		if match != nil {
			flushRaw()
			title := match[1]
			current = &Section{Title: title, Lines: []string{}}
			if types[title] == SectionTypeRaw {
				current.Type = SectionTypeRaw
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
			if strings.TrimSpace(rawLine) != "" {
				return nil, fmt.Errorf("Line %d: expected a ## section heading before content.", index+1)
			}
			continue
		}
		if hasRaw {
			if !inFence && rawHeadingRe.MatchString(rawLine) {
				return nil, fmt.Errorf("Raw section \"## %s\" may not contain a level-1 or level-2 heading: %q. Use ### or deeper.", current.Title, strings.TrimSpace(rawLine))
			}
			rawLines = append(rawLines, rawLine)
			continue
		}
		if !inFence && rawHeadingRe.MatchString(rawLine) {
			return nil, fmt.Errorf("Line %d: unexpected heading %q. Use ### or deeper.", index+1, rawLine)
		}
		current.Lines = append(current.Lines, trimEnd(rawLine))
	}

	flushRaw()
	result := make([]Section, len(sections))
	for i, section := range sections {
		for len(section.Lines) > 0 && section.Lines[0] == "" {
			section.Lines = section.Lines[1:]
		}
		for len(section.Lines) > 0 && section.Lines[len(section.Lines)-1] == "" {
			section.Lines = section.Lines[:len(section.Lines)-1]
		}
		if section.Lines == nil {
			section.Lines = []string{}
		}
		result[i] = *section
	}
	return result, nil
}

func listItems(lines []string) [][]string {
	var fence fenceState
	matches := make([][]string, len(lines))
	for i, line := range lines {
		marker, inFence := fence.update(line, fenceMarkerRe)
		if marker != nil {
			matches[i] = nil
			continue
		}
		if inFence {
			matches[i] = nil
			continue
		}
		matches[i] = listItemRe.FindStringSubmatch(line)
	}

	indent := 0
	haveIndent := false
	for _, match := range matches {
		if match != nil {
			if !haveIndent || len(match[1]) < indent {
				indent = len(match[1])
				haveIndent = true
			}
		}
	}

	items := [][]string{}
	for i, line := range lines {
		match := matches[i]
		if len(items) == 0 || (match != nil && len(match[1]) == indent) {
			items = append(items, []string{})
		}
		items[len(items)-1] = append(items[len(items)-1], line)
	}
	return items
}

func itemKey(item []string) string {
	return strings.TrimRight(strings.Join(item, "\n"), "\n")
}

func MergeSections(groups [][]Section, order []string, types SectionTypeMap) ([]Section, error) {
	ordered := []*Section{}
	byTitle := map[string]*Section{}
	rawBodies := map[string]map[string]bool{}

	for _, group := range groups {
		for _, section := range group {
			if order != nil {
				if err := assertKnownSection(section.Title, order); err != nil {
					return nil, err
				}
			}

			sectionType := section.Type
			if sectionType == "" {
				sectionType = types[section.Title]
			}
			if sectionType == SectionTypeRaw {
				body := section.Body
				if strings.TrimSpace(body) == "" {
					continue
				}

				merged := byTitle[section.Title]
				if merged == nil {
					merged = &Section{Title: section.Title, Type: SectionTypeRaw, Lines: []string{}}
					byTitle[section.Title] = merged
					rawBodies[section.Title] = map[string]bool{}
					ordered = append(ordered, merged)
				}

				seen := rawBodies[section.Title]
				if seen == nil {
					seen = map[string]bool{}
					rawBodies[section.Title] = seen
				}
				if seen[body] {
					continue
				}
				seen[body] = true
				if merged.Body == "" {
					merged.Body = body
				} else {
					merged.Body = merged.Body + "\n\n" + body
				}
				continue
			}

			if len(section.Lines) == 0 {
				continue
			}

			merged := byTitle[section.Title]
			if merged == nil {
				merged = &Section{Title: section.Title, Lines: []string{}}
				byTitle[section.Title] = merged
				ordered = append(ordered, merged)
			}
			seen := map[string]bool{}
			for _, item := range listItems(merged.Lines) {
				seen[itemKey(item)] = true
			}
			for _, item := range listItems(section.Lines) {
				key := itemKey(item)
				if !seen[key] {
					merged.Lines = append(merged.Lines, item...)
					seen[key] = true
				}
			}
		}
	}

	if order != nil {
		return orderSections(ordered, order), nil
	}
	result := make([]Section, len(ordered))
	for i, section := range ordered {
		result[i] = *section
	}
	return result, nil
}

func MergeFragments(fragments []Fragment, order []string, types SectionTypeMap) ([]Section, error) {
	sorted := append([]Fragment{}, fragments...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	groups := make([][]Section, 0, len(sorted))
	for _, fragment := range sorted {
		sections, err := ParseFragment(fragment.Content, types)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fragment.Name, err)
		}
		hasEntries := false
		for _, section := range sections {
			if len(section.Lines) > 0 || strings.TrimSpace(section.Body) != "" {
				hasEntries = true
				break
			}
		}
		if !hasEntries {
			return nil, fmt.Errorf("%s: Fragment contains no section entries.", fragment.Name)
		}
		if order != nil {
			for _, section := range sections {
				if err := assertKnownSection(section.Title, order); err != nil {
					return nil, fmt.Errorf("%s: %w", fragment.Name, err)
				}
			}
		}
		groups = append(groups, sections)
	}

	return MergeSections(groups, order, types)
}

func ParseChangelog(markdown string, types SectionTypeMap) ([]VersionBlock, error) {
	_, blocks, err := ParseChangelogDocument(markdown, types)
	return blocks, err
}

func releaseVersion(title string) string {
	dated := datedHeadingRe.FindStringSubmatch(title)
	if dated != nil {
		return strings.TrimSpace(dated[1])
	}
	return title
}

func ParseChangelogDocument(markdown string, types SectionTypeMap) (string, []VersionBlock, error) {
	lines := splitLines(markdown)
	blocks := []VersionBlock{}
	start := -1
	version := ""
	unreleased := false
	hasHeading := false
	firstVersion := -1

	push := func(end int) error {
		if !hasHeading || start < 0 {
			return nil
		}
		raw := strings.TrimRightFunc(strings.Join(lines[start:end], "\n"), unicode.IsSpace)
		sections, err := ParseFragment(strings.Join(lines[start+1:end], "\n"), types)
		if err != nil {
			return err
		}
		blocks = append(blocks, VersionBlock{
			Version:    version,
			Unreleased: unreleased,
			Sections:   sections,
			Raw:        raw,
		})
		return nil
	}

	var fence fenceState
	for index, line := range lines {
		fenceMatch, _ := fence.update(line, fenceStartRe)
		if fenceMatch != nil {
			continue
		}
		if fence.marker != "" {
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
			return "", nil, fmt.Errorf("Unexpected level-1 heading: %q.", title)
		}
		blockVersion := title
		if unreleasedMatch != nil {
			blockVersion = strings.TrimSpace(unreleasedMatch[1])
		} else {
			blockVersion = releaseVersion(title)
		}
		if _, err := ParseVersion(blockVersion); err != nil {
			return "", nil, err
		}
		if err := push(index); err != nil {
			return "", nil, err
		}
		if firstVersion < 0 {
			firstVersion = index
		}
		version = blockVersion
		unreleased = unreleasedMatch != nil
		hasHeading = true
		start = index
	}

	if err := push(len(lines)); err != nil {
		return "", nil, err
	}
	preambleEnd := firstVersion
	if firstVersion < 0 {
		preambleEnd = len(lines)
	}
	preamble := strings.Join(lines[:preambleEnd], "\n")
	return preamble, blocks, nil
}

func assertKnownSection(title string, order []string) error {
	if !contains(order, title) {
		quoted := make([]string, len(order))
		for i, section := range order {
			quoted[i] = fmt.Sprintf("%q", "## "+section)
		}
		return fmt.Errorf("Unknown changelog section %q. Expected one of: %s.", "## "+title, strings.Join(quoted, ", "))
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func orderSections(sections []*Section, order []string) []Section {
	index := map[string]int{}
	for position, title := range order {
		index[title] = position
	}
	sorted := append([]*Section{}, sections...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, ok := index[sorted[i].Title]
		if !ok {
			left = maxInt
		}
		right, ok := index[sorted[j].Title]
		if !ok {
			right = maxInt
		}
		return left < right
	})
	result := make([]Section, len(sorted))
	for i, section := range sorted {
		result[i] = *section
	}
	return result
}

func RenderChangelog(sections []Section, title string) string {
	parts := []string{"# " + title}

	for _, section := range sections {
		if section.Type == SectionTypeRaw {
			parts = append(parts, "## "+section.Title, section.Body)
		} else {
			parts = append(parts, "## "+section.Title, strings.Join(section.Lines, "\n"))
		}
	}

	return strings.Join(parts, "\n\n") + "\n"
}

func PrependChangelog(existing, entry string) string {
	trimmed := strings.TrimSpace(existing)
	if trimmed == "" {
		return entry
	}
	return entry + "\n" + trimmed + "\n"
}

const maxInt = int(^uint(0) >> 1)
