# semfrag

Merge `changelog.d` fragments into a changelog, with automatic semantic
versioning. Think of it as a combination of [scriv](https://github.com/nedbat/scriv)
and [semantic-release](https://github.com/semantic-release/semantic-release): the
fragment workflow of the former with the automatic versioning of the latter.

It is intentionally agnostic: it does not care about your VCS, your programming
language, or your commit convention. Anything that can write a markdown file can
use it.

Drop small markdown files into a `changelog.d` directory as you work, then run
`semfrag generate` to merge them into a single unreleased section that is
prepended to `CHANGELOG.md`. Fragments are cleared afterwards so the next release
starts clean. Run `semfrag promote stable` when you are ready to cut the release.

Versions are read from the changelog itself: there is no version to pass by hand.
The next version is the highest bump level among the pending sections applied to
the last released version. Until it is released, a section is titled
`X.Y.Z - UNRELEASED`; releasing it stamps the date, for example
`1.1.0 - January 1st, 2026`.

A changelog title and introduction before the first version heading are preserved.
Fragments must start with a `##` section heading and contain at least one entry;
invalid fragments are reported by filename before the changelog is written or
any fragments are cleared.

## Install

As a Go tool:

```sh
go install github.com/joe-p/semfrag/cmd/semfrag@latest
```

Or as a standalone binary that needs no runtime at all. Every
[release](https://github.com/joe-p/semfrag/releases) ships self-contained
executables cross-compiled from Go for Linux, macOS and Windows on x64 and
arm64:

```sh
# Linux x64
curl -fsSL -o semfrag https://github.com/joe-p/semfrag/releases/latest/download/semfrag-linux-x64
chmod +x semfrag
./semfrag --version
```

The available assets are:

| Target             | Asset                       |
| ------------------ | --------------------------- |
| Linux x64          | `semfrag-linux-x64`         |
| Linux x64 (musl)   | `semfrag-linux-x64-musl`    |
| Linux arm64        | `semfrag-linux-arm64`       |
| Linux arm64 (musl) | `semfrag-linux-arm64-musl`  |
| macOS x64          | `semfrag-darwin-x64`        |
| macOS arm64        | `semfrag-darwin-arm64`      |
| Windows x64        | `semfrag-windows-x64.exe`   |
| Windows arm64      | `semfrag-windows-arm64.exe` |

## Usage

Create a fragment for each change. A fragment is a markdown file with one or more
section headings:

```md
## Fixes

- Fix a crash when the config file is missing
```

```sh
semfrag init [options]
semfrag generate [options]
semfrag promote <channel> [options]
semfrag latest [options]
semfrag notes [options]
```

`generate` is the default command, so `semfrag` on its own is equivalent to
`semfrag generate`.

### Initialize a changelog

```sh
semfrag init
```

creates `CHANGELOG.md` containing `# 1.0.0 - UNRELEASED`, the `changelog.d`
directory, and a `semfrag.json` with the default sections. Pass
`--initial 0.1.0` to start from a pre-1.0 version instead:

```sh
semfrag init --initial 0.1.0
```

```md
# 0.1.0 - UNRELEASED
```

A `0.y.z` start writes the config with `Breaking Changes` set to `MINOR`, since a
pre-1.0 release may make breaking changes in a minor bump:

```json
{
  "sections": [
    { "title": "Breaking Changes", "bump": "MINOR" },
    { "title": "Fixes", "bump": "PATCH" },
    { "title": "Features", "bump": "MINOR" }
  ]
}
```

`init` fails if the changelog already exists, leaves an existing config file
untouched, and requires `--initial` to be a plain `MAJOR.MINOR.PATCH` version (no
prerelease or build metadata).

### Generate an unreleased section

Given a changelog whose latest release is `1.0.0`:

```md
# 1.0.0

## Features

- Released 1.0!
```

and a fragment `changelog.d/fix.md`:

```md
## Fixes

- Some fix
```

running:

```sh
semfrag generate
```

prepends a new unreleased section and clears the fragment:

```md
# 1.0.1 - UNRELEASED

## Fixes

- Some fix

# 1.0.0

## Features

- Released 1.0!
```

Add another fragment `changelog.d/feat.md`:

```md
## Features

- A new feature!
```

and run `semfrag generate` again. The existing unreleased section is merged
with the new fragment and the version is recomputed from the last released
version (`1.0.0`), so the minor bump wins:

```md
# 1.1.0 - UNRELEASED

## Fixes

- Some fix

## Features

- A new feature!

# 1.0.0

## Features

- Released 1.0!
```

### Promote a release

When you are ready to ship, run:

```sh
semfrag promote stable
```

`promote` takes the channel to promote to as an argument: `stable`, `alpha`,
`beta` or `rc`. `stable` finalizes the top section, replacing the ` - UNRELEASED`
suffix (or a prerelease suffix) with the release date:

```md
# 1.1.0 - January 1st, 2026

## Fixes

- Some fix

## Features

- A new feature!

# 1.0.0

## Features

- Released 1.0!
```

`promote` fails if the top section cannot move to the requested channel, or if
`changelog.d` still contains pending fragments (run `generate` first).

### Latest released version

```sh
semfrag latest
```

prints the most recent released version from the changelog (default
`CHANGELOG.md`), skipping the top section while it is still ` - UNRELEASED`:

```sh
$ semfrag latest
1.1.0
```

This is useful in scripts that need the released version, for example to tag a
release. Prereleases count as released, so a top `1.0.1-alpha.1` prints
`1.0.1-alpha.1`. The command fails if the changelog has no released version yet.

### Release notes

```sh
semfrag notes
```

prints the changelog body of the most recent released version, without the
`# X.Y.Z - <date>` heading, so it can be piped straight into a release:

```sh
gh release create "v$(semfrag latest)" --notes "$(semfrag notes)"
```

Like `latest`, it skips a top ` - UNRELEASED` section, counts prereleases as
released, and fails if the changelog has no released version yet.

### Pre-releases

Promotions move forward along the `alpha` -> `beta` -> `rc` -> `stable` ladder,
so each channel only accepts certain top sections:

| Command          | Accepted top section            |
| ---------------- | ------------------------------- |
| `promote alpha`  | `UNRELEASED`                    |
| `promote beta`   | `UNRELEASED` or an alpha        |
| `promote rc`     | `UNRELEASED`, alpha or beta     |
| `promote stable` | `UNRELEASED`, alpha, beta or rc |

Use `promote alpha` to tag the top unreleased section as a prerelease instead of
finalizing it:

```sh
semfrag promote alpha
```

```md
# 1.0.1-alpha.1 - January 1st, 2026

## Fixes

- Fixed a bug

# 1.0.0

## Features

- Released 1.0!
```

The number increments for the same version and channel (`1.0.1-alpha.1` becomes
`1.0.1-alpha.2`), and a different channel restarts at `.1`. New fragments still
generate a plain `1.0.1 - UNRELEASED` on top of the prerelease:

```md
# 1.0.1 - UNRELEASED

## Fixes

- Some new fix

# 1.0.1-alpha.1

## Fixes

- Fixed a bug

# 1.0.0

## Features

- Released 1.0!
```

`promote stable` finalizes the version by merging the unreleased section and all
same-version prerelease sections into `1.0.1` and removing the prerelease blocks:

```md
# 1.0.1 - January 1st, 2026

## Fixes

- Some new fix
- Fixed a bug

# 1.0.0

## Features

- Released 1.0!
```

If there is no unreleased section, `promote stable` promotes the top prerelease
to a final release. Custom prerelease tags are not supported.

### The initial release

When there is no released version yet, the first `generate` creates
`# 1.0.0 - UNRELEASED`, or `# 0.1.0 - UNRELEASED` if you initialized with
`--initial 0.1.0`. While the initial version is still unreleased it stays fixed
regardless of the bump levels of the pending sections, so your first release is
exactly the version you started with. Once a release has been promoted, later
changes bump normally from that version.

### Preview without writing

```sh
semfrag generate --dry-run
semfrag promote stable --dry-run
```

`--dry-run` prints what would happen without writing to the changelog or clearing
fragments. Use `--no-clear` to write the changelog but keep the fragments, or
`-o -` to write the generated section to stdout. Stdout generation reads the
version and existing unreleased content from `CHANGELOG.md`; use `--input <path>`
to read a different changelog. Stdout generation never clears fragments.

### Options

`promote` additionally takes the target channel as a positional argument
(`stable`, `alpha`, `beta` or `rc`).

| Option                | Description                                                                                      |
| --------------------- | ------------------------------------------------------------------------------------------------ |
| `-d, --dir <path>`    | Directory containing fragments (default: `changelog.d`)                                          |
| `-o, --output <path>` | Changelog file, or `-` for stdout (default: `CHANGELOG.md`)                                      |
| `--input <path>`      | `generate` only: existing changelog to read (default: output path, or `CHANGELOG.md` for stdout) |
| `-c, --config <path>` | Config file to read or, for `init`, write (default: `semfrag.json`)                              |
| `--initial <version>` | `init` only: starting version, e.g. `0.1.0` or `1.0.0` (default: `1.0.0`)                        |
| `--dry-run`           | Print the result without writing or clearing                                                     |
| `--no-clear`          | Keep fragment files after generating                                                             |
| `-h, --help`          | Show help                                                                                        |
| `-v, --version`       | Show the package version                                                                         |

## CI/CD

The commands are designed to run unattended. For GitHub Actions, this repository
ships composite actions that handle the semfrag side of a release, leaving the
language-specific steps (bumping a manifest, publishing a package) to you. They
install the standalone binary, so no language runtime is needed.

| Action                           | Description                                                                                                   |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `joe-p/semfrag/actions/setup`    | Install the standalone `semfrag` binary and add it to `PATH`                                                  |
| `joe-p/semfrag/actions/check`    | Validate fragments with `generate --dry-run`, and optionally require a fragment when matching paths change    |
| `joe-p/semfrag/actions/prepare`  | Run `generate` and `promote`, and output `released`, `version`, `notes` and `notes-file`                      |
| `joe-p/semfrag/actions/publish`  | Commit the release, push it, tag it and create a GitHub release with the notes, optionally uploading `assets` |
| `joe-p/semfrag/actions/rollback` | Delete the GitHub release and tag created by this run and restore the branch                                  |

The actions are not versioned separately yet, so reference them by the full
commit SHA of a semfrag release, with the version as a comment. Every release
tag points at its release commit, so the SHA for a version is:

```sh
git ls-remote https://github.com/joe-p/semfrag refs/tags/v0.8.0
```

Dependabot's `github-actions` ecosystem understands this form and updates both
the SHA and the comment. The binary the actions install follows the same pin: it
defaults to the version in `VERSION` at that commit. Pass `version` to
override it, for example `version: latest`.

### Release on every push

```yaml
name: Release

on:
  push:
    branches: [main]

concurrency:
  group: release
  cancel-in-progress: false

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: main # the branch tip, so a queued run releases everything merged since
          fetch-depth: 0

      - uses: joe-p/semfrag/actions/prepare@<sha> # v0.8.0
        id: semfrag

      # Your language-specific bump, e.g. npm, cargo set-version, poetry version...
      - if: steps.semfrag.outputs.released == 'true'
        run: npm pkg set "version=${{ steps.semfrag.outputs.version }}"

      - if: steps.semfrag.outputs.released == 'true'
        uses: joe-p/semfrag/actions/publish@<sha> # v0.8.0

      # Your language-specific publish.
      - if: steps.semfrag.outputs.released == 'true'
        run: npm publish

      - if: failure() || cancelled()
        uses: joe-p/semfrag/actions/rollback@<sha> # v0.8.0
```

`prepare`, `publish` and `rollback` share state through the job environment, so
there is nothing to wire between them. `rollback` must be the **last** step: it
then also undoes the release when one of your own steps fails, such as the
publish above. A rollback cannot unpublish a package that already reached a
registry, so make publish steps skip versions that already exist if you publish
to more than one place.

`publish` stages modified and deleted tracked files with `git add -u`, along
with the configured changelog. Other new files are not committed. See each
`action.yml` for the full list of inputs, such as `tag-prefix`, `commit-message`,
`assets` and `prerelease`.

### Check pull requests

```yaml
on: pull_request

jobs:
  changelog:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: joe-p/semfrag/actions/check@<sha> # v0.8.0
        with:
          require-fragment-for: ^src/
```

This repository's own
[`pr.yml`](https://github.com/joe-p/semfrag/blob/main/.github/workflows/pr.yml)
and
[`release.yml`](https://github.com/joe-p/semfrag/blob/main/.github/workflows/release.yml)
use these actions, adding standalone binaries around them.

Outside GitHub Actions, the same release takes four commands:

```sh
semfrag generate          # merge fragments into the unreleased section
semfrag promote stable    # drop the UNRELEASED suffix
tag="v$(semfrag latest)"  # tag the released version
gh release create "$tag" --notes "$(semfrag notes)"
```

## Configuration

By default `semfrag` looks for `semfrag.json`. It lists the allowed
sections in the order they should appear, along with the semantic version bump
each section implies:

```json
{
  "sections": [
    { "title": "Breaking Changes", "bump": "MAJOR" },
    { "title": "Fixes", "bump": "PATCH" },
    { "title": "Features", "bump": "MINOR" },
    { "title": "Upgrade Guide", "type": "raw" }
  ]
}
```

- `title` is required and must be unique.
- `bump` is optional and must be `MAJOR`, `MINOR`, or `PATCH` (case-insensitive).
- `type` is optional and must be `list` (the default) or `raw` (case-insensitive).
- A fragment section that is not listed causes an error.

The highest bump level among the pending sections is applied to the last released
version. For example, `Breaking Changes` and `Features` fragments on top of
`1.2.3` produce `2.0.0`.

### Section types

A `list` section (the default) holds markdown list items. Multiline items retain
their continuation lines, nested lists, code blocks, and internal blank lines.
Identical complete items across fragments are collapsed, so `- Fix a bug` only
ever appears once; repeated lines within different items are preserved.

A `raw` section preserves the fragment markdown verbatim, including blank lines,
indentation, code blocks and nested headings. This is useful for prose or a
migration guide. When several fragments contribute to the same raw section their
bodies are concatenated with a blank line between them; an identical body is
never added twice.

Outside fenced code blocks, a raw section may not contain a level-1 (`#`) or
level-2 (`##`) heading, because those delimit versions and sections. Use `###`
or deeper for nested headings:

````md
## Upgrade Guide

Run the migration before starting the server:

```sh
migrate up
```

### From 1.x

Replace `oldThing()` with `newThing()`.
````

## Programmatic API

```go
package main

import (
	"fmt"

	semfrag "github.com/joe-p/semfrag"
)

func main() {
	_, _ = semfrag.Init(semfrag.InitOptions{
		Output:  "CHANGELOG.md",
		Dir:     "changelog.d",
		Config:  "semfrag.json",
		Version: "0.1.0",
	})

	_, _ = semfrag.Generate(semfrag.GenerateOptions{
		Dir:    "changelog.d",
		Output: "CHANGELOG.md",
		Clear:  true,
		Order:  []string{"Breaking Changes", "Fixes", "Features", "Upgrade Guide"},
		Bump: map[string]semfrag.BumpLevel{
			"Breaking Changes": semfrag.BumpMajor,
			"Fixes":            semfrag.BumpPatch,
			"Features":         semfrag.BumpMinor,
		},
		Types: semfrag.SectionTypeMap{"Upgrade Guide": semfrag.SectionTypeRaw},
	})

	_, _ = semfrag.Promote(semfrag.PromoteOptions{Output: "CHANGELOG.md", Dir: "changelog.d", Channel: semfrag.ChannelStable})
	_, _ = semfrag.Promote(semfrag.PromoteOptions{Output: "CHANGELOG.md", Dir: "changelog.d", Channel: semfrag.ChannelAlpha})

	latest, _ := semfrag.Latest(semfrag.LatestOptions{Output: "CHANGELOG.md"})
	notes, _ := semfrag.Notes(semfrag.NotesOptions{Output: "CHANGELOG.md"})
	fmt.Println(latest.Version, notes.Notes)

	blocks, _ := semfrag.ParseChangelog("# 1.0.0\n\n## Features\n\n- hello\n", semfrag.SectionTypeMap{"Features": semfrag.SectionTypeList})
	_ = blocks
}
```

## License

MIT
