# RELEASE

## Process
1. Ensure `make check` is green.
2. Update `docs/CHANGELOG.md`.
3. Tag a release `vX.Y.Z`.
4. Build artifacts (`make build`) and publish GitHub Release.

## v0.1.0
- `git tag v0.1.0`
- `make build`
- `gh release create v0.1.0 --notes-file docs/CHANGELOG.md`
