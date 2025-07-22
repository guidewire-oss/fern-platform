# Security Policy

## Supported Versions

The following versions of Fern Platform are currently being supported with security updates:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of Fern Platform seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please do NOT:
- Open a public GitHub issue
- Discuss the vulnerability publicly before it's fixed

### Please DO:
- Email security details to: security@guidewire.com
- Include the following information:
  - Type of vulnerability
  - Full paths of source file(s) related to the vulnerability
  - Location of the affected source code (tag/branch/commit)
  - Step-by-step instructions to reproduce the issue
  - Proof-of-concept or exploit code (if possible)
  - Impact of the vulnerability

### What to expect:
- Acknowledgment of your report within 48 hours
- Regular updates on our progress (at least every 72 hours)
- Credit for responsible disclosure (if desired)
- A security advisory once the issue is resolved

### Security Update Process:
1. Security patches are prioritized above all other work
2. Patches are released as soon as possible
3. All supported versions receive security updates
4. Security advisories are published via GitHub Security Advisories

## Security Best Practices

When using Fern Platform:
- Always use the latest stable version
- Enable GitHub's Dependabot alerts on your fork
- Regularly update dependencies
- Review security advisories
- Use environment variables for sensitive configuration
- Never commit credentials or secrets

## Security Features

Fern Platform includes several security features:
- OAuth 2.0 authentication
- Role-based access control (RBAC)
- Encrypted session management
- SQL injection prevention via parameterized queries
- XSS protection in web interfaces
- CSRF protection on state-changing operations

Thank you for helping keep Fern Platform and its users safe!