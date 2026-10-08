package semfrag

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type GenerateOptions struct {
	Dir    string
	Output string
	Input  string
	Clear  bool
	DryRun bool
	Order  []string
	Bump   map[string]BumpLevel
	Types  SectionTypeMap
}

type GenerateResult struct {
	Entry     string
	Version   string
	Title     string
	Level     *BumpLevel
	Previous  *string
	Fragments []string
	Written   bool
	Cleared   []string
}

type PromoteChannel string

const (
	ChannelStable PromoteChannel = "stable"
	ChannelAlpha  PromoteChannel = "alpha"
	ChannelBeta   PromoteChannel = "beta"
	ChannelRC     PromoteChannel = "rc"
)

var PromoteChannels = []PromoteChannel{ChannelStable, ChannelAlpha, ChannelBeta, ChannelRC}

func IsPromoteChannel(value string) bool {
	for _, channel := range PromoteChannels {
		if string(channel) == value {
			return true
		}
	}
	return false
}

var promoteSources = map[PromoteChannel][]string{
	ChannelStable: {"unreleased", "alpha", "beta", "rc"},
	ChannelAlpha:  {"unreleased"},
	ChannelBeta:   {"unreleased", "alpha"},
	ChannelRC:     {"unreleased", "alpha", "beta"},
}

type PromoteOptions struct {
	Output  string
	Dir     string
	Channel PromoteChannel
	DryRun  bool
	Order   []string
	Types   SectionTypeMap
	Now     time.Time
}

type PromoteResult struct {
	Version string
	Written bool
}

type LatestOptions struct {
	Output string
}

type LatestResult struct {
	Version string
}

type NotesOptions struct {
	Output string
}

type NotesResult struct {
	Version string
	Notes   string
}

type InitOptions struct {
	Output  string
	Dir     string
	Version string
	Config  string
	DryRun  bool
}

type InitResult struct {
	Version       string
	Title         string
	Output        string
	Dir           string
	Config        string
	ConfigWritten bool
	Written       bool
}

const DefaultInitialVersion = "1.0.0"

var renameFile = os.Rename

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func listFragments(dir string) ([]string, error) {
	if !fileExists(dir) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".md") {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names, nil
}

func randomID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func atomicWrite(file, content string) error {
	destination := file
	if fileExists(file) {
		resolved, err := filepath.EvalSymlinks(file)
		if err != nil {
			return err
		}
		destination = resolved
	} else {
		absolute, err := filepath.Abs(file)
		if err != nil {
			return err
		}
		destination = absolute
	}

	temporary := filepath.Join(
		filepath.Dir(destination),
		fmt.Sprintf(".%s.%s.tmp", filepath.Base(destination), randomID()),
	)
	defer os.Remove(temporary)

	perm := fs.FileMode(0o666)
	if info, err := os.Stat(destination); err == nil {
		perm = info.Mode().Perm()
	}
	handle, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := handle.WriteString(content); err != nil {
		handle.Close()
		return err
	}
	if err := handle.Close(); err != nil {
		return err
	}
	return renameFile(temporary, destination)
}

func withPreamble(preamble, content string) string {
	if preamble == "" {
		return content
	}
	separator := "\n\n"
	if strings.HasSuffix(preamble, "\n\n") {
		separator = ""
	} else if strings.HasSuffix(preamble, "\n") {
		separator = "\n"
	}
	return preamble + separator + content
}

func ReadFragments(dir string) ([]Fragment, error) {
	names, err := listFragments(dir)
	if err != nil {
		return nil, err
	}
	fragments := make([]Fragment, len(names))
	for i, name := range names {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		fragments[i] = Fragment{Name: name, Content: string(content)}
	}
	return fragments, nil
}

func Init(options InitOptions) (InitResult, error) {
	requested := options.Version
	if requested == "" {
		requested = DefaultInitialVersion
	}
	parsed, err := ParseVersion(requested)
	if err != nil {
		return InitResult{}, err
	}
	if parsed.Prerelease != "" || parsed.Build != "" {
		return InitResult{}, fmt.Errorf(`Invalid initial version: %q. Expected a plain MAJOR.MINOR.PATCH version.`, requested)
	}
	if fileExists(options.Output) {
		return InitResult{}, fmt.Errorf("%s already exists. Remove it first or choose another output.", options.Output)
	}

	version, err := BaseVersion(requested)
	if err != nil {
		return InitResult{}, err
	}
	title := fmt.Sprintf("%s - %s", version, UnreleasedMarker)
	entry := RenderChangelog(nil, title)
	configPath := options.Config
	if configPath == "" {
		configPath = DefaultConfigFile
	}
	configWritten := !fileExists(configPath)

	if !options.DryRun {
		if err := os.MkdirAll(options.Dir, 0o777); err != nil {
			return InitResult{}, err
		}
		if err := atomicWrite(options.Output, entry); err != nil {
			return InitResult{}, err
		}
		if configWritten {
			config, err := DefaultConfig(version)
			if err != nil {
				return InitResult{}, err
			}
			serialized, err := SerializeConfig(config)
			if err != nil {
				return InitResult{}, err
			}
			if err := atomicWrite(configPath, serialized); err != nil {
				return InitResult{}, err
			}
		}
	}

	return InitResult{
		Version:       version,
		Title:         title,
		Output:        options.Output,
		Dir:           options.Dir,
		Config:        configPath,
		ConfigWritten: configWritten,
		Written:       !options.DryRun,
	}, nil
}

func SelectBump(sections []Section, bump map[string]BumpLevel) (BumpLevel, bool) {
	levels := []BumpLevel{}
	for _, section := range sections {
		if level, ok := bump[section.Title]; ok && level != "" {
			levels = append(levels, level)
		}
	}
	return HighestBump(levels)
}

func resolveNextVersion(blocks []VersionBlock, sections []Section, bump map[string]BumpLevel) (string, *BumpLevel, *string, error) {
	level, hasLevel := SelectBump(sections, bump)
	var levelPtr *BumpLevel
	if hasLevel {
		value := level
		levelPtr = &value
	}

	lastReleased := ""
	foundReleased := false
	for _, block := range blocks {
		if !block.Unreleased && !IsPrerelease(block.Version) {
			lastReleased = block.Version
			foundReleased = true
			break
		}
	}

	prereleaseBases := []string{}
	for _, block := range blocks {
		if IsPrerelease(block.Version) {
			base, err := BaseVersion(block.Version)
			if err != nil {
				return "", nil, nil, err
			}
			prereleaseBases = append(prereleaseBases, base)
		}
	}

	if !foundReleased {
		base := DefaultInitialVersion
		for _, block := range blocks {
			if block.Unreleased {
				base = block.Version
				break
			}
		}
		version, err := HighestBase(append([]string{base}, prereleaseBases...)...)
		if err != nil {
			return "", nil, nil, err
		}
		return version, levelPtr, nil, nil
	}

	bumped, err := NextVersion(lastReleased, levelPtr)
	if err != nil {
		return "", nil, nil, err
	}
	version, err := HighestBase(append([]string{bumped}, prereleaseBases...)...)
	if err != nil {
		return "", nil, nil, err
	}
	previous := lastReleased
	return version, levelPtr, &previous, nil
}

func Generate(options GenerateOptions) (GenerateResult, error) {
	fragments, err := ReadFragments(options.Dir)
	if err != nil {
		return GenerateResult{}, err
	}
	names := make([]string, len(fragments))
	for i, fragment := range fragments {
		names[i] = fragment.Name
	}

	toStdout := options.Output == "-"
	input := options.Input
	if input == "" {
		if toStdout {
			input = "CHANGELOG.md"
		} else {
			input = options.Output
		}
	}
	existing := ""
	if fileExists(input) {
		data, err := os.ReadFile(input)
		if err != nil {
			return GenerateResult{}, err
		}
		existing = string(data)
	}

	preamble, blocks, err := ParseChangelogDocument(existing, options.Types)
	if err != nil {
		return GenerateResult{}, err
	}
	for i := 1; i < len(blocks); i++ {
		if blocks[i].Unreleased {
			return GenerateResult{}, fmt.Errorf("An UNRELEASED section must appear only at the top of the changelog.")
		}
	}

	existingUnreleased := []Section{}
	if len(blocks) > 0 && blocks[0].Unreleased {
		existingUnreleased = blocks[0].Sections
	}
	fragmentSections, err := MergeFragments(fragments, options.Order, options.Types)
	if err != nil {
		return GenerateResult{}, err
	}
	sections, err := MergeSections([][]Section{existingUnreleased, fragmentSections}, options.Order, options.Types)
	if err != nil {
		return GenerateResult{}, err
	}

	if len(sections) == 0 {
		var previous *string
		for _, block := range blocks {
			if !block.Unreleased {
				value := block.Version
				previous = &value
				break
			}
		}
		return GenerateResult{
			Entry:     "",
			Version:   "",
			Title:     "",
			Level:     nil,
			Previous:  previous,
			Fragments: names,
			Written:   false,
			Cleared:   []string{},
		}, nil
	}

	version, level, previous, err := resolveNextVersion(blocks, sections, options.Bump)
	if err != nil {
		return GenerateResult{}, err
	}
	title := fmt.Sprintf("%s - %s", version, UnreleasedMarker)
	entry := RenderChangelog(sections, title)

	remainderParts := []string{}
	for _, block := range blocks {
		if !block.Unreleased {
			remainderParts = append(remainderParts, block.Raw)
		}
	}
	remainder := strings.Join(remainderParts, "\n\n")

	written := !options.DryRun && !toStdout
	if written {
		if err := atomicWrite(options.Output, withPreamble(preamble, PrependChangelog(remainder, entry))); err != nil {
			return GenerateResult{}, err
		}
	}

	cleared := []string{}
	if options.Clear && written && len(names) > 0 {
		for _, fragment := range fragments {
			file := filepath.Join(options.Dir, fragment.Name)
			current, err := os.ReadFile(file)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return GenerateResult{}, err
			}
			if string(current) != fragment.Content {
				continue
			}
			if err := os.Remove(file); err != nil {
				return GenerateResult{}, err
			}
			cleared = append(cleared, fragment.Name)
		}
	}

	return GenerateResult{
		Entry:     entry,
		Version:   version,
		Title:     title,
		Level:     level,
		Previous:  previous,
		Fragments: names,
		Written:   written,
		Cleared:   cleared,
	}, nil
}

func currentChannel(block VersionBlock) string {
	if block.Unreleased {
		return "unreleased"
	}
	prerelease := PrereleaseOf(block.Version)
	if prerelease == "" {
		return "stable"
	}
	match := prereleaseNumRe.FindStringSubmatch(prerelease)
	if match != nil {
		return match[1]
	}
	return prerelease
}

func Promote(options PromoteOptions) (PromoteResult, error) {
	data, err := os.ReadFile(options.Output)
	if err != nil {
		return PromoteResult{}, err
	}
	preamble, blocks, err := ParseChangelogDocument(string(data), options.Types)
	if err != nil {
		return PromoteResult{}, err
	}

	dir := options.Dir
	if dir == "" {
		dir = "changelog.d"
	}
	pending, err := listFragments(dir)
	if err != nil {
		return PromoteResult{}, err
	}
	if len(pending) > 0 {
		return PromoteResult{}, fmt.Errorf("Cannot promote: %s still contains %d pending fragment(s). Run generate first.", dir, len(pending))
	}

	if len(blocks) == 0 {
		return PromoteResult{}, fmt.Errorf("No version to promote in %s.", options.Output)
	}
	top := blocks[0]
	current := currentChannel(top)
	if !contains(promoteSources[options.Channel], current) {
		return PromoteResult{}, fmt.Errorf("Cannot promote %s to %s in %s.", current, options.Channel, options.Output)
	}

	if options.Channel == ChannelStable {
		return promoteStable(options, blocks, preamble)
	}
	return promotePrerelease(options, blocks, options.Channel, preamble)
}

func latestReleased(blocks []VersionBlock, output string) (VersionBlock, error) {
	for _, block := range blocks {
		if !block.Unreleased {
			return block, nil
		}
	}
	return VersionBlock{}, fmt.Errorf("No released version found in %s.", output)
}

func Latest(options LatestOptions) (LatestResult, error) {
	data, err := os.ReadFile(options.Output)
	if err != nil {
		return LatestResult{}, err
	}
	_, blocks, err := ParseChangelogDocument(string(data), nil)
	if err != nil {
		return LatestResult{}, err
	}
	released, err := latestReleased(blocks, options.Output)
	if err != nil {
		return LatestResult{}, err
	}
	return LatestResult{Version: released.Version}, nil
}

func Notes(options NotesOptions) (NotesResult, error) {
	data, err := os.ReadFile(options.Output)
	if err != nil {
		return NotesResult{}, err
	}
	_, blocks, err := ParseChangelogDocument(string(data), nil)
	if err != nil {
		return NotesResult{}, err
	}
	released, err := latestReleased(blocks, options.Output)
	if err != nil {
		return NotesResult{}, err
	}
	lines := strings.Split(released.Raw, "\n")
	notes := strings.Join(lines[1:], "\n")
	notes = strings.TrimLeft(notes, "\n")
	notes = strings.TrimRightFunc(notes, unicode.IsSpace)
	return NotesResult{Version: released.Version, Notes: notes}, nil
}

func promotePrerelease(options PromoteOptions, blocks []VersionBlock, channel PromoteChannel, preamble string) (PromoteResult, error) {
	top := blocks[0]
	if _, err := ParseVersion(top.Version); err != nil {
		return PromoteResult{}, err
	}
	versions := make([]string, len(blocks))
	for i, block := range blocks {
		versions[i] = block.Version
	}
	version, err := NextPrerelease(top.Version, string(channel), versions)
	if err != nil {
		return PromoteResult{}, err
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	title := fmt.Sprintf("%s - %s", version, FormatReleaseDate(now))
	entry := RenderChangelog(top.Sections, title)
	remainderParts := []string{}
	for i := 1; i < len(blocks); i++ {
		remainderParts = append(remainderParts, blocks[i].Raw)
	}
	remainder := strings.Join(remainderParts, "\n\n")

	if !options.DryRun {
		if err := atomicWrite(options.Output, withPreamble(preamble, PrependChangelog(remainder, entry))); err != nil {
			return PromoteResult{}, err
		}
	}
	return PromoteResult{Version: version, Written: !options.DryRun}, nil
}

func promoteStable(options PromoteOptions, blocks []VersionBlock, preamble string) (PromoteResult, error) {
	top := blocks[0]
	base, err := BaseVersion(top.Version)
	if err != nil {
		return PromoteResult{}, err
	}

	consumed := []VersionBlock{}
	index := 0
	for index < len(blocks) {
		block := blocks[index]
		if !block.Unreleased && PrereleaseOf(block.Version) == "" {
			break
		}
		blockBase, err := BaseVersion(block.Version)
		if err != nil {
			return PromoteResult{}, err
		}
		if blockBase != base {
			break
		}
		consumed = append(consumed, block)
		index++
	}

	groups := make([][]Section, len(consumed))
	for i, block := range consumed {
		groups[i] = block.Sections
	}
	sections, err := MergeSections(groups, options.Order, options.Types)
	if err != nil {
		return PromoteResult{}, err
	}

	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	entry := RenderChangelog(sections, fmt.Sprintf("%s - %s", base, FormatReleaseDate(now)))
	remainderParts := []string{}
	for _, block := range blocks[index:] {
		remainderParts = append(remainderParts, block.Raw)
	}
	remainder := strings.Join(remainderParts, "\n\n")

	if !options.DryRun {
		if err := atomicWrite(options.Output, withPreamble(preamble, PrependChangelog(remainder, entry))); err != nil {
			return PromoteResult{}, err
		}
	}
	return PromoteResult{Version: base, Written: !options.DryRun}, nil
}
