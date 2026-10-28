# Security policy

## Reporting a vulnerability

Please report security issues privately, not in public issues:

- GitHub: **Security → Report a vulnerability** on this repository, or
- email **hello@moveinfra.eu** with "security" in the subject.

Include the version (`lockin --version`), how to reproduce, and the impact you see. We reply within 3 working days
and keep you updated until a fix is released. We credit reporters in the release notes unless you ask us not to.

## Supported versions

Fixes go into the latest release. Upgrade the binary, or the version pinned in `uses:`, to get them.

## What the scanner can and cannot do

Useful when you assess its risk in your repository or CI:

- **Reads:** files below the path you give it, up to 1 MB each, and `.lockinignore`. It does not follow the content
  of the files anywhere: no includes, no module downloads, no code execution.
- **Writes:** only the files you name with `--out`, `--json`, `--markdown` and `--badge`.
- **Network:** none. There is no telemetry and no update check. Rules are compiled into the binary.
- **Credentials:** it does not read cloud credentials or environment variables, and needs none.
- **Reports:** contain file paths, line numbers and service names from your repository. A public badge shows only
  the score. Think before posting a full report from a private repository somewhere public.
- **GitHub Action:** downloads the release tarball and `checksums.txt` from this repository's GitHub releases over
  HTTPS and checks the SHA-256 before running the binary. Inputs are passed to the scripts as environment variables.
  Pin a version tag (`@v0.1.0`). If you pin a commit SHA, also set the `version` input, so the binary is pinned too.
