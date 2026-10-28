# Releasing

Push a tag `vX.Y.Z` on `main`:

```sh
git tag -a v0.2.0 -m v0.2.0
git push origin v0.2.0
```

The `Release` workflow (`.github/workflows/release.yml`) then:

1. runs `go vet` and the tests;
2. builds `lockin` for linux, darwin and windows on amd64 and arm64, with `-X main.version=vX.Y.Z`;
3. packs each as `lockin_X.Y.Z_<os>_<arch>.tar.gz` (with `LICENSE` and `README.md`) and writes `checksums.txt`
   (SHA-256);
4. checks that the Linux binary runs;
5. creates a GitHub release with the tarballs, the checksums and generated notes.

The GitHub Action downloads these files, so the asset names are part of its interface. Do not rename them.

Users pin the action with `uses: moveinfra/cloud-lock-in-scanner@vX.Y.Z`, and the action then downloads that same
release. Used at a branch or a commit, it downloads the latest release.

## Versioning

- Patch (`0.1.1`): fixes, new rules, rule corrections. A rule change can move scores; say so in the release notes.
- Minor (`0.2.0`): new detectors, flags, outputs or action inputs.
- Major: a flag, output field or action input removed or renamed, or a change to the score formula.
