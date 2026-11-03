## Fixes

- `rollback` now undoes the release commit by pushing a revert commit with `[skip ci]` instead of force pushing the branch back. The force push's head commit had no `[skip ci]`, so a release workflow triggered by pushes started again, and a release that kept failing looped forever. Set the revert's message with the new `commit-message` input.
