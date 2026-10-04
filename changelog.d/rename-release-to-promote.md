## Breaking Changes

- Replace the `release` command with `promote <channel>`. It always takes a target channel (`stable`, `alpha`, `beta` or `rc`) and moves prereleases forward along the alpha -> beta -> rc -> stable ladder. The `--alpha`, `--beta`, `--rc` and `--pre` flags and custom prerelease tags are removed.
