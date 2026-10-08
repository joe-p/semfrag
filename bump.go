package semfrag

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// BumpLevel is the component of a semantic version that a section bumps. The
// zero value means "no bump".
type BumpLevel string

const (
	BumpPatch BumpLevel = "PATCH"
	BumpMinor BumpLevel = "MINOR"
	BumpMajor BumpLevel = "MAJOR"
)

// bumpWeight orders levels so the most significant bump can be selected.
var bumpWeight = map[BumpLevel]int{BumpPatch: 1, BumpMinor: 2, BumpMajor: 3}

// IsBumpLevel reports whether value is one of MAJOR, MINOR or PATCH.
func IsBumpLevel(value string) bool {
	_, ok := bumpWeight[BumpLevel(value)]
	return ok
}

// ParsedVersion is a semantic version split into its components. Prerelease and
// Build are empty when the version does not carry them.
type ParsedVersion struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

const maxSafeInteger = 9007199254740991

var (
	versionRe       = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	numericRe       = regexp.MustCompile(`^\d+$`)
	channelRe       = regexp.MustCompile(`^[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*$`)
	prereleaseNumRe = regexp.MustCompile(`^(.*)\.(\d+)$`)
)

func invalidVersion(version string) error {
	return fmt.Errorf("invalid semantic version: %q, expected MAJOR.MINOR.PATCH", version)
}

func safeInteger(part string) (int, bool) {
	value, err := strconv.Atoi(part)
	if err != nil || value > maxSafeInteger {
		return 0, false
	}
	return value, true
}

// ParseVersion parses a semantic version, tolerating a leading v.
func ParseVersion(version string) (ParsedVersion, error) {
	match := versionRe.FindStringSubmatch(strings.TrimSpace(version))
	if match == nil {
		return ParsedVersion{}, invalidVersion(version)
	}

	major, ok1 := safeInteger(match[1])
	minor, ok2 := safeInteger(match[2])
	patch, ok3 := safeInteger(match[3])
	if !ok1 || !ok2 || !ok3 {
		return ParsedVersion{}, invalidVersion(version)
	}

	if match[4] != "" {
		for _, part := range strings.Split(match[4], ".") {
			if numericRe.MatchString(part) && len(part) > 1 && part[0] == '0' {
				return ParsedVersion{}, invalidVersion(version)
			}
		}
	}

	parsed := ParsedVersion{Major: major, Minor: minor, Patch: patch}
	if match[4] != "" {
		parsed.Prerelease = match[4]
	}
	if match[5] != "" {
		parsed.Build = match[5]
	}
	return parsed, nil
}

// ApplyBump returns version with level applied, dropping any prerelease or
// build metadata.
func ApplyBump(version string, level BumpLevel) (string, error) {
	parsed, err := ParseVersion(version)
	if err != nil {
		return "", err
	}

	switch level {
	case BumpMajor:
		return checkedVersion(fmt.Sprintf("%d.0.0", parsed.Major+1))
	case BumpMinor:
		return checkedVersion(fmt.Sprintf("%d.%d.0", parsed.Major, parsed.Minor+1))
	case BumpPatch:
		return checkedVersion(fmt.Sprintf("%d.%d.%d", parsed.Major, parsed.Minor, parsed.Patch+1))
	default:
		return "", fmt.Errorf("invalid bump level %q", level)
	}
}

// BaseVersion returns version without prerelease or build metadata.
func BaseVersion(version string) (string, error) {
	parsed, err := ParseVersion(version)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d", parsed.Major, parsed.Minor, parsed.Patch), nil
}

// PrereleaseOf returns the prerelease part of version, or "" when it has none
// or is not a valid version.
func PrereleaseOf(version string) string {
	parsed, err := ParseVersion(version)
	if err != nil {
		return ""
	}
	return parsed.Prerelease
}

// IsPrerelease reports whether version carries prerelease metadata.
func IsPrerelease(version string) bool {
	return PrereleaseOf(version) != ""
}

func checkedVersion(version string) (string, error) {
	if _, err := ParseVersion(version); err != nil {
		return "", err
	}
	return version, nil
}

// PrereleaseVersion tags base with channel and number, e.g. 1.2.3-alpha.1.
func PrereleaseVersion(base, channel string, number int) (string, error) {
	if !channelRe.MatchString(channel) || number < 1 || number > maxSafeInteger {
		return "", errors.New("invalid prerelease channel or number")
	}
	baseVersion, err := BaseVersion(base)
	if err != nil {
		return "", err
	}
	return checkedVersion(fmt.Sprintf("%s-%s.%d", baseVersion, channel, number))
}

// NextPrerelease returns the next prerelease for base and channel, incrementing
// the highest matching number found in versions and restarting at .1 otherwise.
func NextPrerelease(base, channel string, versions []string) (string, error) {
	target, err := BaseVersion(base)
	if err != nil {
		return "", err
	}
	highest := 0

	for _, version := range versions {
		prerelease := PrereleaseOf(version)
		if prerelease == "" {
			continue
		}
		versionBase, err := BaseVersion(version)
		if err != nil || versionBase != target {
			continue
		}
		match := prereleaseNumRe.FindStringSubmatch(prerelease)
		if match == nil || match[1] != channel {
			continue
		}
		if number, _ := strconv.Atoi(match[2]); number > highest {
			highest = number
		}
	}

	return PrereleaseVersion(target, channel, highest+1)
}

// CompareBase compares the core versions of a and b, ignoring prerelease and
// build metadata.
func CompareBase(a, b string) (int, error) {
	left, err := ParseVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := ParseVersion(b)
	if err != nil {
		return 0, err
	}
	if c := cmp.Compare(left.Major, right.Major); c != 0 {
		return c, nil
	}
	if c := cmp.Compare(left.Minor, right.Minor); c != 0 {
		return c, nil
	}
	return cmp.Compare(left.Patch, right.Patch), nil
}

// HighestBase returns the base version of the highest of versions, or 0.0.0
// when none are given.
func HighestBase(versions ...string) (string, error) {
	highest := ""
	for i, version := range versions {
		if i == 0 {
			highest = version
			continue
		}
		comparison, err := CompareBase(version, highest)
		if err != nil {
			return "", err
		}
		if comparison > 0 {
			highest = version
		}
	}
	if highest == "" {
		return "0.0.0", nil
	}
	return BaseVersion(highest)
}

// HighestBump returns the most significant level, or "" when levels is empty.
func HighestBump(levels []BumpLevel) BumpLevel {
	highest := BumpLevel("")
	for _, level := range levels {
		if highest == "" || bumpWeight[level] > bumpWeight[highest] {
			highest = level
		}
	}
	return highest
}

// NextVersion applies level to base, returning base unchanged for an empty
// level.
func NextVersion(base string, level BumpLevel) (string, error) {
	if level == "" {
		return base, nil
	}
	return ApplyBump(base, level)
}
