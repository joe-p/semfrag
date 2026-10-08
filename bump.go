package semfrag

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type BumpLevel string

const (
	BumpPatch BumpLevel = "PATCH"
	BumpMinor BumpLevel = "MINOR"
	BumpMajor BumpLevel = "MAJOR"
)

var BumpLevels = []BumpLevel{BumpPatch, BumpMinor, BumpMajor}

var bumpWeight = map[BumpLevel]int{BumpPatch: 1, BumpMinor: 2, BumpMajor: 3}

func IsBumpLevel(value string) bool {
	for _, level := range BumpLevels {
		if string(level) == value {
			return true
		}
	}
	return false
}

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
	return fmt.Errorf(`Invalid semantic version: %q. Expected MAJOR.MINOR.PATCH.`, version)
}

func safeInteger(part string) (int, bool) {
	value, err := strconv.Atoi(part)
	if err != nil || value > maxSafeInteger {
		return 0, false
	}
	return value, true
}

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
		return "", fmt.Errorf("Invalid bump level: %q.", level)
	}
}

func BaseVersion(version string) (string, error) {
	parsed, err := ParseVersion(version)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d", parsed.Major, parsed.Minor, parsed.Patch), nil
}

func PrereleaseOf(version string) string {
	parsed, err := ParseVersion(version)
	if err != nil {
		return ""
	}
	return parsed.Prerelease
}

func IsPrerelease(version string) bool {
	return PrereleaseOf(version) != ""
}

func checkedVersion(version string) (string, error) {
	if _, err := ParseVersion(version); err != nil {
		return "", err
	}
	return version, nil
}

func PrereleaseVersion(base, channel string, number int) (string, error) {
	if !channelRe.MatchString(channel) || number < 1 || number > maxSafeInteger {
		return "", fmt.Errorf("Invalid prerelease channel or number.")
	}
	baseVersion, err := BaseVersion(base)
	if err != nil {
		return "", err
	}
	return checkedVersion(fmt.Sprintf("%s-%s.%d", baseVersion, channel, number))
}

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
		number, _ := strconv.Atoi(match[2])
		if number > highest {
			highest = number
		}
	}

	return PrereleaseVersion(target, channel, highest+1)
}

func CompareBase(a, b string) (int, error) {
	left, err := ParseVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := ParseVersion(b)
	if err != nil {
		return 0, err
	}
	if left.Major != right.Major {
		return left.Major - right.Major, nil
	}
	if left.Minor != right.Minor {
		return left.Minor - right.Minor, nil
	}
	return left.Patch - right.Patch, nil
}

func HighestBase(versions ...string) (string, error) {
	highest := ""
	found := false
	for _, version := range versions {
		if !found {
			highest = version
			found = true
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
	if !found {
		return "0.0.0", nil
	}
	return BaseVersion(highest)
}

func HighestBump(levels []BumpLevel) (BumpLevel, bool) {
	var highest BumpLevel
	found := false
	for _, level := range levels {
		if !found || bumpWeight[level] > bumpWeight[highest] {
			highest = level
			found = true
		}
	}
	return highest, found
}

func NextVersion(base string, level *BumpLevel) (string, error) {
	if level == nil {
		return base, nil
	}
	return ApplyBump(base, *level)
}
