## Features

- Add composite GitHub Actions for language-agnostic releases: `setup` installs the standalone binary, `prepare` merges fragments and finalizes the release, `publish` commits, tags and creates the GitHub release, `rollback` undoes a failed release (including failures in your own publish steps), and `check` validates fragments on pull requests.
