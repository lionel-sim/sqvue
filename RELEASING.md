# Releasing sqvue

Releases are created automatically when a version tag is pushed from a
`release/*` branch. The release workflow rejects a tag unless its commit is
contained by an existing remote `release/*` branch. It then tests the project,
builds macOS, Linux, and Windows archives for amd64 and arm64, publishes them
to GitHub Releases, and uploads SHA-256 checksums.

Use semantic version tags with a leading `v`:

- `v0.1.0-beta.1` creates a GitHub prerelease.
- `v0.1.0` creates the corresponding stable release.

Before pushing a tag, run the project checks locally and confirm the intended
tag points to the tested commit. Push the release branch before the tag so the
workflow can validate it. The release workflow supplies the tag version to
`sqvue --version`; ordinary source builds continue to report `dev`.
