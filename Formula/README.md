<p align="center">
  <a href="https://digitalis.io">
    <img src="https://digitalis-marketplace-assets.s3.us-east-1.amazonaws.com/DigitalisDigital_DigitalisFullLogoGradient+-+medium.png" alt="Digitalis.IO" width="300">
  </a>
</p>

<p align="center">
  <em>Built and maintained by <a href="https://digitalis.io">Digitalis.IO</a></em>
</p>

# Homebrew Formula for KConduit

This directory contains the Homebrew formula for KConduit.

## Installation via Homebrew Tap

> **Note:** the `digitalis-io/homebrew-tap` repository does not exist yet, so
> this installation path is not available today. Use "Manual Formula
> Installation" below until the tap is published.

Once the tap is set up, users will be able to install KConduit using:

```bash
# Add the tap (only needed once)
brew tap digitalis-io/tap

# Install kconduit
brew install kconduit
```

## Manual Formula Installation

> **Note:** `Formula/kconduit.rb` is currently a template. It pins
> `version "0.0.1"` while the latest release is v0.0.4, and every `sha256` is
> still an empty string waiting for GoReleaser to fill it in. Installing from it
> as-is will fail. Update the version and checksums first — see "Manual Formula
> Update" below.

For development or testing, once the formula has real values in it:

```bash
brew install --build-from-source Formula/kconduit.rb
```

## Updating the Formula

> **Note:** automatic formula updates are not yet enabled. The `brews:` block
> in [`.goreleaser.yml`](../.goreleaser.yml) that would perform this is
> currently commented out, and the `digitalis-io/homebrew-tap` repository
> referenced below does not exist yet. Until both are in place, update
> `Formula/kconduit.rb` manually — see "Manual Formula Update" below.

Once enabled, GoReleaser will, on every tagged release:

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

This needs the version and checksums filled in first, for the reason given under
"Manual Formula Installation".

## Release Process

1. Tag a new version: `git tag v0.1.0`
2. Push the tag: `git push origin v0.1.0`
3. Run GoReleaser: `goreleaser release --clean`
4. Once the `brews:` block is enabled (see "Updating the Formula" above),
   GoReleaser updates the Homebrew tap automatically. Until then, follow
   "Manual Formula Update" below.

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

## 💬 Support

This project is maintained by [Digitalis.io](https://digitalis.io). For support,
visit [digitalis.io/contact](https://digitalis.io/contact).