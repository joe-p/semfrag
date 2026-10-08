package semfrag

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
)

// MergeSections merges groups of sections by title, keeping first-seen order
// (or order when given) and collapsing duplicate list items and raw bodies.
func MergeSections(groups [][]Section, order []string, types SectionTypes) ([]Section, error) {
	ordered := []*Section{}
	byTitle := map[string]*Section{}
	rawBodies := map[string]map[string]bool{}
	kinds := map[string]SectionType{}

	for _, group := range groups {
		for _, section := range group {
			if order != nil {
				if err := assertKnownSection(section.Title, order); err != nil {
					return nil, err
				}
			}

			kind := sectionKind(section, types)
			if previous, ok := kinds[section.Title]; ok && previous != kind {
				return nil, fmt.Errorf("conflicting types for section %q: %s and %s", section.Title, previous, kind)
			}
			kinds[section.Title] = kind
			if kind == SectionRaw {
				ordered = mergeRawSection(section, byTitle, rawBodies, ordered)
				continue
			}
			ordered = mergeListSection(section, byTitle, ordered)
		}
	}

	if order != nil {
		return orderSections(ordered, order), nil
	}
	return derefSections(ordered), nil
}

// sectionKind resolves a section's effective type, falling back to the config.
func sectionKind(section Section, types SectionTypes) SectionType {
	if section.Type != "" {
		return section.Type
	}
	if kind := types[section.Title]; kind != "" {
		return kind
	}
	return SectionList
}

func mergeRawSection(section Section, byTitle map[string]*Section, rawBodies map[string]map[string]bool, ordered []*Section) []*Section {
	body := section.Body
	if strings.TrimSpace(body) == "" {
		return ordered
	}

	target := byTitle[section.Title]
	if target == nil {
		target = &Section{Title: section.Title, Type: SectionRaw, Lines: []string{}}
		byTitle[section.Title] = target
		rawBodies[section.Title] = map[string]bool{}
		ordered = append(ordered, target)
	}

	seen := rawBodies[section.Title]
	if seen[body] {
		return ordered
	}
	seen[body] = true
	if target.Body == "" {
		target.Body = body
	} else {
		target.Body += "\n\n" + body
	}
	return ordered
}

func mergeListSection(section Section, byTitle map[string]*Section, ordered []*Section) []*Section {
	if len(section.Lines) == 0 {
		return ordered
	}

	target := byTitle[section.Title]
	if target == nil {
		target = &Section{Title: section.Title, Lines: []string{}}
		byTitle[section.Title] = target
		ordered = append(ordered, target)
	}

	seen := map[string]bool{}
	for _, item := range listItems(target.Lines) {
		seen[itemKey(item)] = true
	}
	for _, item := range listItems(section.Lines) {
		key := itemKey(item)
		if seen[key] {
			continue
		}
		target.Lines = append(target.Lines, item...)
		seen[key] = true
	}
	return ordered
}

func itemKey(item []string) string {
	return strings.TrimRight(strings.Join(item, "\n"), "\n")
}

// MergeFragments parses and merges fragments by filename order.
func MergeFragments(fragments []Fragment, order []string, types SectionTypes) ([]Section, error) {
	sorted := slices.Clone(fragments)
	slices.SortStableFunc(sorted, func(a, b Fragment) int {
		return cmp.Compare(a.Name, b.Name)
	})

	groups := make([][]Section, 0, len(sorted))
	for _, fragment := range sorted {
		sections, err := ParseFragment(fragment.Content, types)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fragment.Name, err)
		}
		if !hasEntries(sections) {
			return nil, fmt.Errorf("%s: fragment contains no section entries", fragment.Name)
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

func hasEntries(sections []Section) bool {
	for _, section := range sections {
		if len(section.Lines) > 0 || strings.TrimSpace(section.Body) != "" {
			return true
		}
	}
	return false
}

func orderSections(sections []*Section, order []string) []Section {
	position := make(map[string]int, len(order))
	for i, title := range order {
		position[title] = i
	}
	sorted := slices.Clone(sections)
	slices.SortStableFunc(sorted, func(a, b *Section) int {
		return cmp.Compare(sectionPosition(position, a.Title), sectionPosition(position, b.Title))
	})
	return derefSections(sorted)
}

func sectionPosition(position map[string]int, title string) int {
	if p, ok := position[title]; ok {
		return p
	}
	return math.MaxInt
}

func derefSections(sections []*Section) []Section {
	result := make([]Section, len(sections))
	for i, section := range sections {
		result[i] = *section
	}
	return result
}
