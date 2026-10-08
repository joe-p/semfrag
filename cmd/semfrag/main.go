package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	semfrag "github.com/joe-p/semfrag"
)

// version is injected at build time by scripts/build-binaries.sh. Binaries
// installed with `go install ...@vX.Y.Z` leave it empty and are resolved from
// the module build info instead; local builds fall back to 0.0.0.
var version = ""

func resolveVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	return versionFromBuildInfo(info, ok)
}

func versionFromBuildInfo(info *debug.BuildInfo, ok bool) string {
	if ok && info != nil {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return "0.0.0"
}

const help = `semfrag - merge changelog.d fragments into a changelog

Usage:
  semfrag [generate] [options]
  semfrag promote <channel> [options]
  semfrag latest [options]
  semfrag notes [options]
  semfrag init [options]

Commands:
  generate              Merge pending fragments into an unreleased section and
                        prepend it to the changelog. This is the default command.
  promote <channel>     Promote the top section to a release. <channel> is one of
                        stable, alpha, beta or rc. "stable" finalizes the top
                        unreleased or prerelease section, merging same-version
                        prereleases. The other channels tag the top section as a
                        prerelease. Fails if fragments are pending.
  latest                Print the latest released version from the changelog.
  notes                 Print the changelog body of the latest release, without
                        the version heading. Useful for release notes.
  init                  Create an empty changelog with an unreleased heading, the
                        fragments directory and a config file. Fails if the
                        changelog exists. A 0.y.z start makes "Breaking Changes"
                        bump MINOR.

Options:
  -d, --dir <path>      Directory containing changelog fragments (default: changelog.d)
  -o, --output <path>   Changelog file, or "-" for stdout (default: CHANGELOG.md)
      --input <path>    Existing changelog to read (default: output, or CHANGELOG.md for stdout)
  -c, --config <path>   Config file to read or, for init, write (default: semfrag.json)
      --initial <ver>   init only: starting version, e.g. 0.1.0 or 1.0.0 (default: 1.0.0)
      --dry-run         Print the result without writing or clearing
      --no-clear        Keep the fragment files after generating
  -h, --help            Show this help
  -v, --version         Show the version

Versions are read from the changelog itself. The next version is the highest
bump level among the pending sections applied to the last released version. While
the top section is "1.0.0 - UNRELEASED" and 1.0.0 has not been released, the
version stays 1.0.0 regardless of bump level.

Promotion:
  Prereleases move forward along the alpha -> beta -> rc -> stable ladder.
  "promote alpha" requires an unreleased top section, "promote beta" accepts an
  unreleased or alpha top, "promote rc" accepts an unreleased, alpha or beta top,
  and "promote stable" accepts any of them. For example, "promote alpha" renames
  the top "1.0.1 - UNRELEASED" section to "1.0.1-alpha.1 - January 1st, 2026".
  Repeating it for the same version increments the number; a different channel
  restarts at .1. New fragments still generate a plain "1.0.1 - UNRELEASED" on
  top. "promote stable" then merges the unreleased and all "1.0.1-*" prerelease
  sections into "1.0.1 - January 1st, 2026" and removes them.

Config file:
  A JSON object with a "sections" array listing the allowed sections in the order
  they should appear. Each section has a "title", an optional semantic version
  "bump" level (MAJOR, MINOR or PATCH) and an optional "type" of "list" (the
  default) or "raw", e.g.

    {
      "sections": [
        { "title": "Breaking Changes", "bump": "MAJOR" },
        { "title": "Features", "bump": "MINOR" },
        { "title": "Fixes", "bump": "PATCH" },
        { "title": "Upgrade Guide", "type": "raw" }
      ]
    }

  A "list" section preserves multiline items and drops duplicate items. A "raw"
  section preserves its markdown verbatim, so it may contain nested markdown
  such as blank lines, code blocks and "###" headings. It may not contain "#" or
  "##" headings outside fenced code. Sections found in fragments that are not
  listed cause an error.
`

type cliArgs struct {
	dir         string
	output      string
	input       string
	inputSet    bool
	config      string
	initial     string
	initialSet  bool
	dryRun      bool
	noClear     bool
	help        bool
	version     bool
	positionals []string
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "semfrag: %s\n", message)
	os.Exit(1)
}

// nextValue consumes the token at index i as the value of option. A token that
// looks like another option is rejected so a missing value cannot swallow a
// flag such as --dry-run. Explicit forms (`--output=--weird`, `-o--weird`) and
// the stdout value `-` remain valid.
func nextValue(argv []string, i int, option string) (string, int, error) {
	if i >= len(argv) {
		return "", i, fmt.Errorf("Option '%s' argument is missing.", option)
	}
	value := argv[i]
	if strings.HasPrefix(value, "-") && value != "-" {
		return "", i, fmt.Errorf("Option '%s' argument is ambiguous.", option)
	}
	return value, i + 1, nil
}

func parseArgs(argv []string) (cliArgs, error) {
	args := cliArgs{}
	i := 0
	for i < len(argv) {
		arg := argv[i]
		i++

		if arg == "--" {
			args.positionals = append(args.positionals, argv[i:]...)
			break
		}

		if strings.HasPrefix(arg, "--") {
			name := arg[2:]
			value := ""
			hasValue := false
			if equals := strings.IndexByte(name, '='); equals >= 0 {
				value = name[equals+1:]
				name = name[:equals]
				hasValue = true
			}
			switch name {
			case "dir", "output", "input", "config", "initial":
				if !hasValue {
					v, next, err := nextValue(argv, i, "--"+name)
					if err != nil {
						return args, err
					}
					value = v
					i = next
				}
				switch name {
				case "dir":
					args.dir = value
				case "output":
					args.output = value
				case "input":
					args.input = value
					args.inputSet = true
				case "config":
					args.config = value
				case "initial":
					args.initial = value
					args.initialSet = true
				}
			case "dry-run":
				args.dryRun = true
			case "no-clear":
				args.noClear = true
			case "help":
				args.help = true
			case "version":
				args.version = true
			default:
				return args, fmt.Errorf("Unknown option '--%s'.", name)
			}
			continue
		}

		if len(arg) > 1 && arg[0] == '-' {
			name := arg[1]
			value := ""
			hasValue := false
			if len(arg) > 2 {
				value = strings.TrimPrefix(arg[2:], "=")
				hasValue = true
			}
			switch name {
			case 'd', 'o', 'c':
				if !hasValue {
					v, next, err := nextValue(argv, i, fmt.Sprintf("-%c", name))
					if err != nil {
						return args, err
					}
					value = v
					i = next
				}
				switch name {
				case 'd':
					args.dir = value
				case 'o':
					args.output = value
				case 'c':
					args.config = value
				}
			case 'h':
				args.help = true
			case 'v':
				args.version = true
			default:
				return args, fmt.Errorf("Unknown option '-%c'.", name)
			}
			continue
		}

		args.positionals = append(args.positionals, arg)
	}
	return args, nil
}

func main() {
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		fail(err.Error())
	}

	if args.help {
		fmt.Fprint(os.Stdout, help)
		return
	}
	if args.version {
		fmt.Fprintf(os.Stdout, "%s\n", resolveVersion())
		return
	}

	command := "generate"
	if len(args.positionals) > 0 {
		command = args.positionals[0]
	}
	switch command {
	case "generate", "promote", "latest", "notes", "init":
	default:
		fail(fmt.Sprintf("unknown command: %s", command))
	}

	var channel semfrag.PromoteChannel
	if command == "promote" {
		if len(args.positionals) < 2 {
			fail(fmt.Sprintf("promote requires a channel: %s", joinChannels()))
		}
		if len(args.positionals) > 2 {
			fail(fmt.Sprintf("unexpected argument: %s", args.positionals[2]))
		}
		requested := args.positionals[1]
		if !semfrag.IsPromoteChannel(requested) {
			fail(fmt.Sprintf("unknown channel: %s. Expected one of: %s", requested, joinChannels()))
		}
		channel = semfrag.PromoteChannel(requested)
	} else if len(args.positionals) > 1 {
		fail(fmt.Sprintf("unexpected argument: %s", args.positionals[1]))
	}

	dir := args.dir
	if dir == "" {
		dir = "changelog.d"
	}
	output := args.output
	if output == "" {
		output = "CHANGELOG.md"
	}

	if command != "generate" && args.inputSet {
		fail("--input can only be used with generate")
	}
	if command != "init" && args.initialSet {
		fail("--initial can only be used with init")
	}

	if command == "init" {
		if output == "-" {
			fail("init cannot write to stdout")
		}
		result, err := semfrag.Init(semfrag.InitOptions{
			Output:  output,
			Dir:     dir,
			Version: args.initial,
			Config:  args.config,
			DryRun:  args.dryRun,
		})
		if err != nil {
			fail(err.Error())
		}
		configNote := ""
		if result.ConfigWritten {
			if args.dryRun {
				configNote = fmt.Sprintf(" and write %s", result.Config)
			} else {
				configNote = fmt.Sprintf(" and wrote %s", result.Config)
			}
		}
		if args.dryRun {
			fmt.Fprintf(os.Stdout, "Would initialize %s at %s%s.\n", output, result.Title, configNote)
			return
		}
		fmt.Fprintf(os.Stdout, "Initialized %s at %s%s.\n", output, result.Title, configNote)
		return
	}

	if command == "latest" {
		result, err := semfrag.Latest(semfrag.LatestOptions{Output: output})
		if err != nil {
			fail(err.Error())
		}
		fmt.Fprintf(os.Stdout, "%s\n", result.Version)
		return
	}

	if command == "notes" {
		result, err := semfrag.Notes(semfrag.NotesOptions{Output: output})
		if err != nil {
			fail(err.Error())
		}
		fmt.Fprintf(os.Stdout, "%s\n", result.Notes)
		return
	}

	config, err := semfrag.LoadConfig(args.config)
	if err != nil {
		fail(err.Error())
	}
	var order []string
	var bumps map[string]semfrag.BumpLevel
	var types semfrag.SectionTypeMap
	if config != nil {
		order = semfrag.SectionOrder(*config)
		bumps = semfrag.SectionBumps(*config)
		types = semfrag.SectionTypes(*config)
	}

	if command == "promote" {
		result, err := semfrag.Promote(semfrag.PromoteOptions{
			Output:  output,
			Dir:     dir,
			DryRun:  args.dryRun,
			Channel: channel,
			Order:   order,
			Types:   types,
		})
		if err != nil {
			fail(err.Error())
		}
		if args.dryRun {
			fmt.Fprintf(os.Stdout, "Would promote to %s in %s.\n", result.Version, output)
			return
		}
		fmt.Fprintf(os.Stdout, "Promoted to %s in %s.\n", result.Version, output)
		return
	}

	result, err := semfrag.Generate(semfrag.GenerateOptions{
		Dir:    dir,
		Output: output,
		Input:  args.input,
		Clear:  !args.noClear,
		DryRun: args.dryRun,
		Order:  order,
		Bump:   bumps,
		Types:  types,
	})
	if err != nil {
		fail(err.Error())
	}

	if result.Entry == "" {
		fmt.Fprint(os.Stdout, "No changelog fragments found.\n")
		return
	}

	if args.dryRun || output == "-" {
		fmt.Fprint(os.Stdout, result.Entry)
		return
	}

	from := "initial"
	if result.Previous != nil {
		from = *result.Previous
	}
	bump := ""
	if result.Level != nil && result.Previous != nil {
		bump = fmt.Sprintf(" (%s)", *result.Level)
	}
	fmt.Fprintf(os.Stderr, "semfrag: %s -> %s%s\n", from, result.Version, bump)

	tail := ".\n"
	if len(result.Cleared) > 0 {
		tail = fmt.Sprintf(" and cleared %d file(s).\n", len(result.Cleared))
	}
	fmt.Fprintf(os.Stdout, "Generated %s from %d fragment(s)%s", output, len(result.Fragments), tail)
}

func joinChannels() string {
	channels := make([]string, len(semfrag.PromoteChannels))
	for i, channel := range semfrag.PromoteChannels {
		channels[i] = string(channel)
	}
	return strings.Join(channels, ", ")
}
