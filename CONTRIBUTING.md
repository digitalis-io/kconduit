<p align="center">
  <a href="https://digitalis.io">
    <img src="https://digitalis-marketplace-assets.s3.us-east-1.amazonaws.com/DigitalisDigital_DigitalisFullLogoGradient+-+medium.png" alt="Digitalis.IO" width="300">
  </a>
</p>

<p align="center">
  <em>Built and maintained by <a href="https://digitalis.io">Digitalis.IO</a></em>
</p>

# Contributing to KConduit

We welcome contributions to KConduit. This document describes how to build, test, and submit changes to the project.

## Prerequisites

- **Go 1.27.0** or later (see `go.mod` for version requirements)
- **Docker** and **docker compose** v2 (for running test Kafka clusters)
- **golangci-lint** v2.13 or later (for linting)

Install golangci-lint from [golangci-lint.run](https://golangci-lint.run/usage/install/).

## Building

Build the binary for your current platform:

```bash
make build
```

The binary is written to `./kconduit`.

To build for multiple platforms (Linux, macOS, Windows):

```bash
make build-all
```

Binaries are written to the `build/` directory.

## Running Locally

Connect to a local default Kafka broker at `localhost:9092`:

```bash
make run
```

To run with debug logging:

```bash
make run-debug
```

## Testing Against Local Kafka Clusters

KConduit ships two Docker Compose test environments. They use overlapping ports, so only one can run at a time.

### Plaintext Cluster

Start a three-broker KRaft cluster with no authentication:

```bash
make kafka-up
```

This cluster listens on `localhost:19094`, `localhost:29094`, `localhost:39094`.

Connect KConduit:

```bash
make run-plain
```

View cluster logs:

```bash
make kafka-logs
```

Stop the cluster:

```bash
make kafka-down
```

Clean up volumes:

```bash
make kafka-clean
```

### SASL Cluster

Start a three-broker KRaft cluster with three independent controllers and SASL/PLAIN authentication enabled:

```bash
make kafka-acls-up
```

This cluster listens on `localhost:29092`, `localhost:39092`, `localhost:49092` (credentials: `admin` / `admin-secret`).

Connect KConduit with SASL:

```bash
make run-acls
```

View cluster logs:

```bash
make kafka-acls-logs
```

Stop the cluster:

```bash
make kafka-acls-down
```

Clean up volumes:

```bash
make kafka-acls-clean
```

## Running Tests

Run all tests with race detection and coverage:

```bash
make test
```

Run short tests only:

```bash
make test-short
```

Generate a coverage report:

```bash
make test-coverage
```

This creates `coverage.html` in the project root.

Run benchmarks:

```bash
make benchmark
```

## Code Quality

### Formatting

Format code with `go fmt`:

```bash
make fmt
```

### Linting

Run the linter (required before committing):

```bash
make lint
```

The linter configuration is in `.golangci.yml`. Fix any reported issues before committing.

### Vetting

Run `go vet` to check for suspicious constructs:

```bash
make vet
```

### Dependency Management

Tidy and verify module dependencies:

```bash
make tidy
```

## Continuous Integration

All pull requests are checked by CI, which runs:

- `go vet` on `./cmd/...` and `./pkg/...`
- `go test -v -race` with coverage (on Ubuntu and macOS)
- `golangci-lint` (v2.13)
- `govulncheck` (vulnerability scanning)

You can run these locally before pushing:

```bash
make vet
make test
make lint
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

## Git Workflow

1. Create a feature branch from an up-to-date `main`:

```bash
git switch main && git pull --ff-only
git switch -c <type>/<short-kebab-description>
```

Branch names follow the pattern `<type>/<description>` where `type` is one of: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `ci`, `perf`, `build`.

2. Commit your changes with signed commits and sign-off:

```bash
git commit -S -s -m "type(scope): your message"
```

Commits must be:
- **Signed**: Use `-S` (GPG or SSH key)
- **Signed-off**: Use `-s` (Developer Certificate of Origin)
- **Conventional Commits**: `type(scope): summary` format

3. Push and open a pull request:

```bash
git push origin <type>/<short-kebab-description>
```

4. Ensure CI passes and code review is complete before merging.

## Updating CHANGELOG

Every pull request must update `CHANGELOG.md`. Add an entry under the `## [Unreleased]` section in the appropriate subsection (`Added`, `Changed`, `Fixed`, `Removed`, `Security`, etc.).

Use [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.

Example:

```markdown
## [Unreleased]

### Added

- New feature description (#123)

### Fixed

- Bug fix description (#124)
```

When releasing, the maintainers will move the `[Unreleased]` block under a new `## [X.Y.Z] - YYYY-MM-DD` header.

Trivial commits (typos, documentation nits with no code changes) may skip CHANGELOG updates.

## Release Process

Maintainers create releases as follows:

1. Update `CHANGELOG.md`: move `[Unreleased]` to `[X.Y.Z] - YYYY-MM-DD]`
2. Tag the release: `git tag vX.Y.Z`
3. Build release artifacts: `make release`
4. Push the tag: `git push origin vX.Y.Z`

The version is extracted from git tags; see `make version` to display the current version.

## Code Style

- Follow idiomatic Go conventions
- Use `go fmt` (checked by CI)
- Avoid magic numbers; use named constants
- Write meaningful error messages
- Document exported functions and types

## Testing Guidelines

- Write unit tests for new features
- Use `t.Helper()` for test helpers
- Aim for >80% code coverage on new code
- Test both happy paths and error cases

## Reporting Issues

If you find a bug or have a feature request, open a GitHub issue with:

- A clear title and description
- Reproduction steps (for bugs)
- Expected vs. actual behavior
- Environment (OS, Go version, Kafka version)

## Questions?

Reach out to the Digitalis.IO team at [digitalis.io/contact](https://digitalis.io/contact).

## 💬 Support

This project is maintained by [Digitalis.io](https://digitalis.io). For support, visit [digitalis.io/contact](https://digitalis.io/contact).
