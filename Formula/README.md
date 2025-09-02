# Homebrew Formula for KConduit

This directory contains the Homebrew formula for KConduit.

## Installation via Homebrew Tap

Once the tap is set up, users can install KConduit using:

```bash
# Add the tap (only needed once)
brew tap digitalis-io/tap

# Install kconduit
brew install kconduit
```

## Manual Formula Installation

For development or testing, you can install directly from the formula file:

```bash
brew install --build-from-source Formula/kconduit.rb
```

## Updating the Formula

The formula is automatically updated by GoReleaser when creating new releases.
The GoReleaser configuration will:

1. Calculate SHA256 checksums for each platform
2. Update the formula with the correct download URLs
3. Push the updated formula to the homebrew-tap repository

## Formula Structure

The formula supports:
- macOS (Intel and Apple Silicon)
- Linux (x86_64 and ARM64)
- Automatic version detection
- Shell completion installation (bash, zsh, fish)
- Service management (for running as a daemon if needed)

## Testing the Formula

To test the formula locally:

```bash
brew install --verbose --debug Formula/kconduit.rb
brew test kconduit
```

## Release Process

1. Tag a new version: `git tag v0.1.0`
2. Push the tag: `git push origin v0.1.0`
3. Run GoReleaser: `goreleaser release --clean`
4. GoReleaser will automatically update the Homebrew tap

## Requirements for Homebrew Tap

To enable automatic Homebrew formula updates:

1. Create a repository named `homebrew-tap` under the digitalis-io organization
2. Set the `HOMEBREW_TAP_GITHUB_TOKEN` environment variable with a GitHub token that has write access to the tap repository
3. Run GoReleaser with the token available

## Manual Formula Update

If you need to manually update the formula after a release:

1. Download the release artifacts
2. Calculate SHA256: `shasum -a 256 kconduit_*.tar.gz`
3. Update the formula with the new version and checksums
4. Test the formula locally
5. Commit and push to the homebrew-tap repository