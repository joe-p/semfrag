## Breaking Changes

- Rewrite `semfrag` in Go, replacing the TypeScript/Bun implementation. JavaScript/TypeScript projects can no longer import the programmatic API; the npm package now installs the prebuilt Go binary for your platform with `npm install -g semfrag` instead of shipping JavaScript. The CLI, config file and changelog formats are unchanged.
