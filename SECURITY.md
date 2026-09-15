<p align="center">
  <a href="https://digitalis.io">
    <img src="https://digitalis-marketplace-assets.s3.us-east-1.amazonaws.com/DigitalisDigital_DigitalisFullLogoGradient+-+medium.png" alt="Digitalis.IO" width="300">
  </a>
</p>

<p align="center">
  <em>Built and maintained by <a href="https://digitalis.io">Digitalis.IO</a></em>
</p>

# Security Policy

## Supported Versions

KConduit is pre-1.0 and in beta. Security fixes are made on the latest release
only; there are no maintained release branches, and versions before the current
one are not patched.

Check the [releases page](https://github.com/digitalis-io/kconduit/releases) for
the latest version, and upgrade to it before reporting a problem.

## Reporting a Vulnerability

If you discover a security vulnerability in KConduit, please report it responsibly by contacting the Digitalis.IO security team at [digitalis.io/contact](https://digitalis.io/contact). Do not open a public GitHub issue.

Please include:

- A description of the vulnerability
- Steps to reproduce (if applicable)
- Affected versions
- Potential impact
- Any suggested fixes (if available)

## Response Process

When a security vulnerability is reported:

1. We acknowledge the report.
2. We investigate and assess the severity and scope.
3. We develop and test a fix.
4. We release a patched version.
5. We publish an advisory describing the issue and the fix.

We do not offer a response-time guarantee for this project. If you need one,
Digitalis.io offers commercial support — see the contact link above.

Severity is classified as:

- **Critical**: Allows remote code execution, credential theft, or unauthorized cluster access
- **High**: Allows privilege escalation, data exfiltration, or denial of service
- **Medium**: Reduces security posture or requires specific conditions to exploit
- **Low**: Minor information disclosure or requires user action to trigger

## Security Considerations for Operators

When running KConduit, consider:

- **Network Access**: Restrict access to the KConduit CLI to trusted users only
- **Kafka Authentication**: Use SASL and TLS when connecting to production Kafka clusters
- **Credentials**: Never commit credentials to version control; use environment variables or secrets management
- **Logging**: Be aware that debug logs may contain sensitive information; disable in production or store securely
- **Dependencies**: Keep Go and all dependencies up to date; use `go mod tidy` and `govulncheck ./...`

## Dependency Security

KConduit uses `govulncheck` in CI to detect known vulnerabilities in dependencies. To check locally:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

See `go.mod` for the current dependency versions.

## Code Review

All changes to KConduit go through code review before merging, including:

- Security-focused review of authentication, encryption, and external input handling
- Linting and static analysis via golangci-lint
- Automated testing with race detection
- Dependency verification

## Scope of Support

KConduit is a Kafka management tool. It does not provide:

- Direct cluster security: Kafka's security model (SASL, ACLs, TLS) is Kafka's responsibility
- Encrypted data storage: KConduit does not persist sensitive data
- Key management: Credentials should be managed externally (environment variables, secrets managers)

## Questions?

For security questions or concerns, contact the Digitalis.IO team at [digitalis.io/contact](https://digitalis.io/contact).

## 💬 Support

This project is maintained by [Digitalis.io](https://digitalis.io). For support, visit [digitalis.io/contact](https://digitalis.io/contact).
