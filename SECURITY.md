# Security Policy

## Supported versions

karya is pre-1.0 and under active development. Security fixes are applied to the
latest `main` and the most recent tagged release.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, use GitHub's private vulnerability reporting:

1. Go to the repository's **Security** tab.
2. Click **Report a vulnerability** to open a private advisory.

Include as much detail as you can: affected version (`karya version`), OS/arch,
a description of the issue, and reproduction steps or a proof of concept.

We aim to acknowledge reports within a few days and to provide a remediation
timeline after triage.

## Scope notes

karya is an orchestrator: it launches Neovim, tmux, coding-agent CLIs, and other
tools. Vulnerabilities in those upstream tools should be reported to their
respective projects. Issues in how karya invokes them, handles paths/env, or
manages its own isolated directories are in scope here.
