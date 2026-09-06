# Releasing sqvue

Releases are created by the manual **Release** workflow. The publish job only
runs when the workflow is launched from a `release/*` branch. It tests the
project, creates and pushes the version tag, builds macOS, Linux, and Windows
archives for amd64 and arm64, publishes them to GitHub Releases, and uploads
SHA-256 checksums. Each release also uploads `install.sh` for the documented
macOS and Linux one-command installer.

Use semantic version tags with a leading `v`:

- `v0.1.0-beta.1` creates a GitHub prerelease.
- `v0.1.0` creates the corresponding stable release.

To publish, push the completed `release/*` branch, open **Actions → Release →
Run workflow**, select that branch, and enter the version tag. Do not create or
push the tag yourself. The workflow validates that the tag does not already
exist and supplies its version to `sqvue --version`; ordinary source builds
continue to report `dev`.
