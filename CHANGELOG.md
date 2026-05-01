# Changelog

## [0.2.0](https://github.com/miradorlabs/bridgesolana/compare/v0.1.0...v0.2.0) (2026-05-01)


### ⚠ BREAKING CHANGES

* callers using the v0.1.0 helpers must switch to det.Resolution.Resolve(data). Resolution.MessageVersion and Resolution.Correlation are no longer exported.

### Features

* collapse parse and extract into Resolution.Resolve ([#5](https://github.com/miradorlabs/bridgesolana/issues/5)) ([9de525c](https://github.com/miradorlabs/bridgesolana/commit/9de525c207b09b0fb04d6f06d90c4e620acbf555))

## [0.1.0](https://github.com/miradorlabs/bridgesolana/compare/v0.1.0...v0.1.0) (2026-05-01)


### ⚠ BREAKING CHANGES

* minimal Detect API and drop chainName argument

### Features

* minimal Detect API and drop chainName argument ([7bf0ff5](https://github.com/miradorlabs/bridgesolana/commit/7bf0ff5e40676ee7e4980e18815e9ab18066254a))


### Dependencies

* **deps:** Bump go.uber.org/zap from 1.27.0 to 1.28.0 ([#1](https://github.com/miradorlabs/bridgesolana/issues/1)) ([fba11dc](https://github.com/miradorlabs/bridgesolana/commit/fba11dc9799a8cc42af30b0727a1438155881935))

## Changelog

All notable changes to this project are managed by
[release-please](https://github.com/googleapis/release-please) and generated
from conventional-commit messages on `main`.

While the major version is `0`, minor releases (`0.x.0`) may include
breaking API changes; patch releases (`0.x.y`) will not.
