# Releasing sqvue

Releases are created by the manual **Release** workflow. The publish job only
runs when the workflow is launched from a `release/*` branch. It tests the
project, creates and pushes the version tag, builds macOS, Linux, and Windows
archives for amd64 and arm64, uploads them to a GitHub release draft with
SHA-256 checksums, and includes `install.sh` for the documented macOS and Linux
one-command installer.

Use semantic version tags with a leading `v`:

- `v0.1.0-beta.1` creates a GitHub prerelease.
- `v0.1.0` creates the corresponding stable release.

To publish, push the completed `release/*` branch, open **Actions → Release →
Run workflow**, select that branch, and enter the version tag. Do not create or
push the tag yourself. The workflow validates that the tag does not already
exist and supplies its version to `sqvue --version`; ordinary source builds
continue to report `dev`.

When the workflow finishes, open the GitHub release draft, edit its title and
notes, and select **Publish release**. The archives and installer remain
unavailable to users until you publish the draft.
