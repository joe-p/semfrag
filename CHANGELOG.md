# 0.10.0 - October 8th, 2026

## Breaking Changes

- Rewrite `semfrag` in Go, replacing the TypeScript/Bun implementation. JavaScript/TypeScript projects can no longer import the programmatic API; the npm package now installs the prebuilt Go binary for your platform with `npm install -g semfrag` instead of shipping JavaScript. The CLI, config file and changelog formats are unchanged.

# 0.9.0 - October 4th, 2026

## Breaking Changes

- Replace the `release` command with `promote <channel>`. It always takes a target channel (`stable`, `alpha`, `beta` or `rc`) and moves prereleases forward along the alpha -> beta -> rc -> stable ladder. The `--alpha`, `--beta`, `--rc` and `--pre` flags and custom prerelease tags are removed.

# 0.8.0 - October 4th, 2026

## Features

- Add composite GitHub Actions for language-agnostic releases: `setup` installs the standalone binary, `prepare` merges fragments and finalizes the release, `publish` commits, tags and creates the GitHub release, `rollback` undoes a failed release (including failures in your own publish steps), and `check` validates fragments on pull requests.

# 0.7.0 - October 3rd, 2026

## Features

- Publish standalone `semfrag` binaries for Linux, macOS and Windows on x64 and arm64 (including musl) with every GitHub release, built via `bun build --compile`. They require no runtime to be installed.

# 0.6.0 - October 3rd, 2026

## Breaking Changes

- Change the default section order to `Breaking Changes`, `Features`, `Fixes`, so features are listed before fixes.

# 0.5.0 - October 3rd, 2026

## Features

- Stamp the release date next to the version when running `release`, e.g. `1.1.0 - January 1st, 2026`.

# 0.4.0

## Breaking Changes

- Rename the package and CLI from `changelog-d` to `semfrag`. The default config file is now `semfrag.json` and the `changelog-d` binary is now `semfrag`.

# 0.3.0

## Features

- Add `semfrag notes` to print the changelog body of the latest release, and use it to generate GitHub release notes.

# 0.2.0

## Features

- Add `semfrag latest` to print the most recent released version from the changelog.
- Add a GitHub Actions workflow that releases and tags on every merge to `main`.

# 0.1.0

## Features

- Add `semfrag init` to create an empty changelog with a `1.0.0` or `0.1.0` unreleased heading, a default `semfrag.json`, and the fragments directory.
- Support `0.y.z` versions: the initial version stays fixed until the first release, and a `0.y.z` config makes `Breaking Changes` bump `MINOR`.
