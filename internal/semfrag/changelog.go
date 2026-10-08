package semfrag

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

// GenerateOptions configures Generate.
type GenerateOptions struct {
	Dir    string
	Output string
	Input  string
	Clear  bool
	DryRun bool
	Order  []string
	Bump   map[string]BumpLevel
	Types  SectionTypes
}

// GenerateResult reports what Generate produced. Level and Previous are empty
// when there is no bump or no previous release.
type GenerateResult struct {
	Entry     string
	Version   string
	Title     string
	Level     BumpLevel
	Previous  string
	Fragments []string
	Written   bool
	Cleared   []string
}

// PromoteChannel is a release channel a section can be promoted to.
type PromoteChannel string

const (
	ChannelStable PromoteChannel = "stable"
	ChannelAlpha  PromoteChannel = "alpha"
	ChannelBeta   PromoteChannel = "beta"
	ChannelRC     PromoteChannel = "rc"
)

// PromoteChannels returns the supported channels in display order.
func PromoteChannels() []PromoteChannel {
	return []PromoteChannel{ChannelStable, ChannelAlpha, ChannelBeta, ChannelRC}
}

// IsPromoteChannel reports whether value is a supported channel.
func IsPromoteChannel(value string) bool {
	switch PromoteChannel(value) {
	case ChannelStable, ChannelAlpha, ChannelBeta, ChannelRC:
		return true
	default:
		return false
	}
}

// promoteSources lists the release states each channel accepts.
var promoteSources = map[PromoteChannel][]string{
	ChannelStable: {"unreleased", "alpha", "beta", "rc"},
	ChannelAlpha:  {"unreleased"},
	ChannelBeta:   {"unreleased", "alpha"},
	ChannelRC:     {"unreleased", "alpha", "beta"},
}

// PromoteOptions configures Promote.
type PromoteOptions struct {
	Output  string
	Dir     string
	Channel PromoteChannel
	DryRun  bool
	Order   []string
	Types   SectionTypes
	Now     time.Time
}

// PromoteResult reports the promoted version.
type PromoteResult struct {
	Version string
	Written bool
}

// LatestOptions configures Latest.
type LatestOptions struct {
	Output string
}

// LatestResult reports the latest released version.
type LatestResult struct {
	Version string
}

// NotesOptions configures Notes.
type NotesOptions struct {
	Output string
}

// NotesResult reports the latest release's notes.
type NotesResult struct {
	Version string
	Notes   string
}

// InitOptions configures Init.
type InitOptions struct {
	Output  string
	Dir     string
	Version string
	Config  string
	DryRun  bool
}

// InitResult reports what Init created.
type InitResult struct {
	Version       string
	Title         string
	Output        string
	Dir           string
	Config        string
	ConfigWritten bool
	Written       bool
}

// DefaultInitialVersion is used when Init is not given a version.
const DefaultInitialVersion = "1.0.0"

// renameFile is a test seam for atomicWrite.
var renameFile = os.Rename

func listFragments(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
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
	slices.Sort(names)
	return names, nil
}

// atomicWrite writes content to path via a temporary file and rename, following
// symlinks and preserving the destination's permissions.
func atomicWrite(path, content string) error {
	destination := path
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		destination = resolved
	} else if errors.Is(err, fs.ErrNotExist) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		destination = absolute
	} else {
		return err
	}

	info, err := os.Stat(destination)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// OpenFile applies the process umask, whereas Chmod does not. CreateTemp
	// always uses 0600, so create an exclusive, randomly named file ourselves.
	perm := os.FileMode(0o666)
	if info != nil {
		perm = 0o600
	}
	var handle *os.File
	for {
		temporary := filepath.Join(filepath.Dir(destination), "."+filepath.Base(destination)+"."+rand.Text()+".tmp")
		handle, err = os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	temporary := handle.Name()
	defer os.Remove(temporary)

	if info != nil {
		if err := handle.Chmod(info.Mode().Perm()); err != nil {
			handle.Close()
			return err
		}
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

// withPreamble places preamble before content, normalizing the separator.
func withPreamble(preamble, content string) string {
	if preamble == "" {
		return content
	}
	separator := "\n\n"
	switch {
	case strings.HasSuffix(preamble, "\n\n"):
		separator = ""
	case strings.HasSuffix(preamble, "\n"):
		separator = "\n"
	}
	return preamble + separator + content
}

// ReadFragments reads the fragment files in dir, sorted by name.
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

// Init creates an unreleased changelog, the fragments directory and, unless it
// already exists, a default config.
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
		return InitResult{}, fmt.Errorf("invalid initial version %q, expected a plain MAJOR.MINOR.PATCH version", requested)
	}
	if _, err := os.Stat(options.Output); err == nil {
		return InitResult{}, fmt.Errorf("%s already exists, remove it first or choose another output", options.Output)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return InitResult{}, err
	}

	version, err := BaseVersion(requested)
	if err != nil {
		return InitResult{}, err
	}
	title := version + " - " + UnreleasedMarker
	configPath := options.Config
	if configPath == "" {
		configPath = DefaultConfigFile
	}
	_, err = os.Stat(configPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return InitResult{}, err
	}
	configWritten := errors.Is(err, fs.ErrNotExist)

	if !options.DryRun {
		if err := os.MkdirAll(options.Dir, 0o777); err != nil {
			return InitResult{}, err
		}
		if err := atomicWrite(options.Output, RenderChangelog(nil, title)); err != nil {
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

// SelectBump returns the most significant bump among the given sections, or ""
// when none of them bump.
func SelectBump(sections []Section, bump map[string]BumpLevel) BumpLevel {
	levels := make([]BumpLevel, 0, len(sections))
	for _, section := range sections {
		if level := bump[section.Title]; level != "" {
			levels = append(levels, level)
		}
	}
	return HighestBump(levels)
}

// resolveNextVersion computes the next version, its bump level and the previous
// release from the parsed blocks and pending sections.
func resolveNextVersion(blocks []VersionBlock, sections []Section, bump map[string]BumpLevel) (string, BumpLevel, string, error) {
	level := SelectBump(sections, bump)

	lastReleased := ""
	for _, block := range blocks {
		if !block.Unreleased && !IsPrerelease(block.Version) {
			lastReleased = block.Version
			break
		}
	}

	prereleaseBases := []string{}
	for _, block := range blocks {
		if IsPrerelease(block.Version) {
			base, err := BaseVersion(block.Version)
			if err != nil {
				return "", "", "", err
			}
			prereleaseBases = append(prereleaseBases, base)
		}
	}

	if lastReleased == "" {
		// The initial version stays fixed until it is released.
		base := DefaultInitialVersion
		for _, block := range blocks {
			if block.Unreleased {
				base = block.Version
				break
			}
		}
		version, err := HighestBase(append([]string{base}, prereleaseBases...)...)
		if err != nil {
			return "", "", "", err
		}
		return version, level, "", nil
	}

	bumped, err := NextVersion(lastReleased, level)
	if err != nil {
		return "", "", "", err
	}
	version, err := HighestBase(append([]string{bumped}, prereleaseBases...)...)
	if err != nil {
		return "", "", "", err
	}
	return version, level, lastReleased, nil
}

// Generate merges pending fragments into an unreleased section and prepends it
// to the changelog.
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
		input = options.Output
		if toStdout {
			input = "CHANGELOG.md"
		}
	}
	data, err := os.ReadFile(input)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return GenerateResult{}, err
	}

	document, err := ParseChangelogDocument(string(data), options.Types)
	if err != nil {
		return GenerateResult{}, err
	}
	blocks := document.Blocks
	for i := 1; i < len(blocks); i++ {
		if blocks[i].Unreleased {
			return GenerateResult{}, fmt.Errorf("an UNRELEASED section must appear only at the top of the changelog")
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
		previous := ""
		for _, block := range blocks {
			if !block.Unreleased {
				previous = block.Version
				break
			}
		}
		return GenerateResult{Fragments: names, Previous: previous, Cleared: []string{}}, nil
	}

	version, level, previous, err := resolveNextVersion(blocks, sections, options.Bump)
	if err != nil {
		return GenerateResult{}, err
	}
	title := version + " - " + UnreleasedMarker
	entry := RenderChangelog(sections, title)

	remainder := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if !block.Unreleased {
			remainder = append(remainder, block.Raw)
		}
	}

	written := !options.DryRun && !toStdout
	if written {
		content := withPreamble(document.Preamble, PrependChangelog(strings.Join(remainder, "\n\n"), entry))
		if err := atomicWrite(options.Output, content); err != nil {
			return GenerateResult{}, err
		}
	}

	cleared := []string{}
	if options.Clear && written {
		for _, fragment := range fragments {
			path := filepath.Join(options.Dir, fragment.Name)
			current, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return GenerateResult{}, err
			}
			// Keep fragments edited since the snapshot; a later generate can
			// consume them.
			if string(current) != fragment.Content {
				continue
			}
			if err := os.Remove(path); err != nil {
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

// currentChannel names the release state of a block for promotion checks.
func currentChannel(block VersionBlock) string {
	if block.Unreleased {
		return "unreleased"
	}
	prerelease := PrereleaseOf(block.Version)
	if prerelease == "" {
		return "stable"
	}
	if match := prereleaseNumRe.FindStringSubmatch(prerelease); match != nil {
		return match[1]
	}
	return prerelease
}

// Promote moves the top section to a release on the given channel.
func Promote(options PromoteOptions) (PromoteResult, error) {
	data, err := os.ReadFile(options.Output)
	if err != nil {
		return PromoteResult{}, err
	}
	document, err := ParseChangelogDocument(string(data), options.Types)
	if err != nil {
		return PromoteResult{}, err
	}
	blocks := document.Blocks

	dir := options.Dir
	if dir == "" {
		dir = "changelog.d"
	}
	pending, err := listFragments(dir)
	if err != nil {
		return PromoteResult{}, err
	}
	if len(pending) > 0 {
		return PromoteResult{}, fmt.Errorf("cannot promote: %s still contains %d pending fragment(s), run generate first", dir, len(pending))
	}

	if len(blocks) == 0 {
		return PromoteResult{}, fmt.Errorf("no version to promote in %s", options.Output)
	}
	current := currentChannel(blocks[0])
	if !slices.Contains(promoteSources[options.Channel], current) {
		return PromoteResult{}, fmt.Errorf("cannot promote %s to %s in %s", current, options.Channel, options.Output)
	}

	if options.Channel == ChannelStable {
		return promoteStable(options, document)
	}
	return promotePrerelease(options, document, options.Channel)
}

func latestReleased(blocks []VersionBlock, output string) (VersionBlock, error) {
	for _, block := range blocks {
		if !block.Unreleased {
			return block, nil
		}
	}
	return VersionBlock{}, fmt.Errorf("no released version found in %s", output)
}

// Latest returns the most recent released version in the changelog.
func Latest(options LatestOptions) (LatestResult, error) {
	data, err := os.ReadFile(options.Output)
	if err != nil {
		return LatestResult{}, err
	}
	document, err := ParseChangelogDocument(string(data), nil)
	if err != nil {
		return LatestResult{}, err
	}
	released, err := latestReleased(document.Blocks, options.Output)
	if err != nil {
		return LatestResult{}, err
	}
	return LatestResult{Version: released.Version}, nil
}

// Notes returns the body of the most recent release, without its heading.
func Notes(options NotesOptions) (NotesResult, error) {
	data, err := os.ReadFile(options.Output)
	if err != nil {
		return NotesResult{}, err
	}
	document, err := ParseChangelogDocument(string(data), nil)
	if err != nil {
		return NotesResult{}, err
	}
	released, err := latestReleased(document.Blocks, options.Output)
	if err != nil {
		return NotesResult{}, err
	}
	lines := strings.Split(released.Raw, "\n")
	notes := strings.TrimLeft(strings.Join(lines[1:], "\n"), "\n")
	notes = strings.TrimRightFunc(notes, unicode.IsSpace)
	return NotesResult{Version: released.Version, Notes: notes}, nil
}

func promotePrerelease(options PromoteOptions, document ChangelogDocument, channel PromoteChannel) (PromoteResult, error) {
	top := document.Blocks[0]
	if _, err := ParseVersion(top.Version); err != nil {
		return PromoteResult{}, err
	}
	versions := make([]string, len(document.Blocks))
	for i, block := range document.Blocks {
		versions[i] = block.Version
	}
	version, err := NextPrerelease(top.Version, string(channel), versions)
	if err != nil {
		return PromoteResult{}, err
	}
	title := version + " - " + FormatReleaseDate(releaseTime(options.Now))
	entry := RenderChangelog(top.Sections, title)
	remainder := make([]string, 0, len(document.Blocks)-1)
	for _, block := range document.Blocks[1:] {
		remainder = append(remainder, block.Raw)
	}

	if !options.DryRun {
		content := withPreamble(document.Preamble, PrependChangelog(strings.Join(remainder, "\n\n"), entry))
		if err := atomicWrite(options.Output, content); err != nil {
			return PromoteResult{}, err
		}
	}
	return PromoteResult{Version: version, Written: !options.DryRun}, nil
}

func promoteStable(options PromoteOptions, document ChangelogDocument) (PromoteResult, error) {
	blocks := document.Blocks
	base, err := BaseVersion(blocks[0].Version)
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

	title := base + " - " + FormatReleaseDate(releaseTime(options.Now))
	entry := RenderChangelog(sections, title)
	remainder := make([]string, 0, len(blocks)-index)
	for _, block := range blocks[index:] {
		remainder = append(remainder, block.Raw)
	}

	if !options.DryRun {
		content := withPreamble(document.Preamble, PrependChangelog(strings.Join(remainder, "\n\n"), entry))
		if err := atomicWrite(options.Output, content); err != nil {
			return PromoteResult{}, err
		}
	}
	return PromoteResult{Version: base, Written: !options.DryRun}, nil
}

func releaseTime(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}
