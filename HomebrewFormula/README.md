# HomebrewFormula

This directory is karya's Homebrew tap. `karya.rb` is a **plain Homebrew formula**
(deliberately not a Cask) so `brew install` works on **both macOS and Linux**.

Install karya via Homebrew:

```sh
brew tap drjzlyan/karya https://github.com/drjzlyan/karya
brew trust drjzlyan/karya   # Homebrew 6.0+ requires trusting non-official taps
brew install karya
```

Notes:
- The repository is not named `homebrew-karya`, so the tap URL is required.
- Since Homebrew 6.0, non-official taps must be trusted before a formula will
  load (a tap can run its own Ruby code). `brew trust drjzlyan/karya` is a
  one-time step; to trust only this formula, use
  `brew trust --formula drjzlyan/karya/karya`.

## Maintenance

`karya.rb` is **generated** by [`scripts/update-formula.sh`](../scripts/update-formula.sh)
from the release's `checksums.txt` and committed to `main` automatically by the
release workflow (`.github/workflows/release.yml`) on each tag — **do not edit it
by hand**. GoReleaser's built-in `brews`/`homebrew_casks` are intentionally not
used (casks are macOS-only; `brews` is deprecated), so the formula stays
cross-platform and future-proof.

To regenerate locally against a release's checksums:

```sh
make formula TAG=v0.1.0 SUMS=dist/checksums.txt
```
