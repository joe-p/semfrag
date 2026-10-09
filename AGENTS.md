# AGENTS.md

## Project

`semfrag` is a Go CLI that merges `changelog.d` fragments into a changelog.

## Changelog fragments

Every code change that affects behavior, fixes a bug, or alters user-facing
output must include a fragment in `changelog.d/`. Do not edit `CHANGELOG.md`
directly; it is generated from fragments and released by CI.

A fragment is a markdown file named after the change in kebab-case (for example
`config-search.md`) containing a section heading and one or more bullet items:

```md
## Features

- Describe the change.
```

The section must be one of the titles listed in `semfrag.json`. Use the section that
matches the highest semantic version bump of the change. Fragments are consumed
by `semfrag generate` at release time, so write them from the user's point of
view.

## Verification

Run `go test ./...` after making changes.
