# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Reporting a Vulnerability

The Fern Platform team takes security bugs seriously. We appreciate your efforts to responsibly disclose your findings, and will make every effort to acknowledge your contributions.

To report a security vulnerability, please use the GitHub Security Advisories feature:

1. Go to https://github.com/guidewire-oss/fern-platform/security/advisories
2. Click "New draft advisory"
3. Fill in the details of your finding

Alternatively, you can email us at: security@guidewire.com

Please include the following information:
- Type of issue (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

## Response Timeline

We take security issues seriously and will make every effort to respond and remediate as soon as possible. Our team strives to:
- Acknowledge receipt of security reports promptly
- Investigate and validate reported issues thoroughly
- Develop and test fixes with appropriate urgency based on severity
- Coordinate responsible disclosure with reporters

## Security Best Practices

When deploying Fern Platform:

1. **Authentication**: Always enable OAuth/OIDC authentication in production
2. **TLS**: Use TLS for all communications
3. **Database**: Use strong passwords and restrict network access
4. **Updates**: Keep Fern Platform updated to the latest version
5. **Secrets**: Never commit secrets to version control

## Security Features

Fern Platform includes several security features:

- OAuth 2.0/OIDC authentication support
- Role-based access control (RBAC)
- Non-root container execution
- Regular dependency updates
- Security scanning in CI/CD pipeline

## Vulnerability Disclosure

When we receive a security report, we will:

1. Confirm the problem and determine affected versions
2. Audit code to find any similar problems
3. Prepare fixes for all supported versions
4. Release new versions and announce the vulnerability

## Attribution

We will credit reporters who responsibly disclose vulnerabilities in our release notes and security advisories.

Thank you for helping keep Fern Platform and our users safe!