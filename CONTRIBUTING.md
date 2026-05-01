# Contributing to bridgesolana

Thanks for considering a contribution.

## Prerequisites

- Go ≥ 1.24
- `make`
- `golangci-lint` — run `make tools` to install a pinned version into
  `./bin/`, or install system-wide with
  `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`.

## Local development

```sh
make test         # run all tests
make lint         # run golangci-lint
make fmt          # gofmt + goimports
make build        # compile the package
make verify       # fmt + vet + lint + test (run before pushing)
```

To install the recommended pre-push and commit-msg git hooks:

```sh
make setup-hooks
```

These run `make verify` before every push and validate commit messages
against our conventions. CI runs the same checks, so the hooks are
optional, but they shorten the feedback loop.

## Commit conventions

We use [Conventional Commits 1.0](https://www.conventionalcommits.org/)
with **mandatory** type and **encouraged** scope:

```
<type>(<scope>): <short description>

<optional body>

Signed-off-by: Your Name <you@example.com>
```

### Allowed types

| Type       | When to use                                         |
|------------|-----------------------------------------------------|
| `feat`     | A new user-facing capability (new bridge, new API)  |
| `fix`      | A bug fix                                           |
| `refactor` | Internal change, no behaviour change                |
| `perf`     | Performance improvement                             |
| `test`     | Tests only                                          |
| `docs`     | README, CONTRIBUTING, godoc                         |
| `build`    | `go.mod`, build scripts                             |
| `ci`       | GitHub Actions, hooks                               |
| `chore`    | Repo plumbing that doesn't fit elsewhere            |
| `revert`   | Reverts a previous commit                           |

### Suggested scopes

- `config` — bridge config additions or updates
- `detector` — `BridgeDetector`, `NewBridgeDetector`, `DetectFromLogs`, `DetectInstructionBridges`
- `extraction` — correlation-ID extraction logic
- `deps` — dependency updates

### Examples

```
feat(config): add CCTP V2 destination on Solana
fix(extraction): handle short MessageSent account data
refactor(detector): collapse program-stack tracking helpers
chore(deps): bump go-ethereum to v1.18.0
```

### Breaking changes

Append `!` to the type and add a `BREAKING CHANGE:` footer:

```
feat(detector)!: rename BridgeDetails.CorrelationID to .CorrelationKey

BREAKING CHANGE: callers must update field references.
```

## Developer Certificate of Origin

All commits must be signed off, certifying that you wrote the change
or have the right to submit it under the project's license:

```sh
git commit -s -m "feat(config): add new bridge"
```

This appends a `Signed-off-by` trailer matching your `git config user.name`
and `user.email`. See https://developercertificate.org for the full text.

The CI will reject pull requests with unsigned commits.

## Pull requests

- Branch from `main`.
- Keep PRs focused — one logical change per PR.
- Update or add tests for behaviour changes.
- Update the README coverage matrix when adding bridges or chains.
- Make sure `make verify` is green locally before opening the PR.

## Adding a new bridge

1. Add the program ID and event/instruction definition to a JSON file
   under `config/<chain>/<bridge>.json`. The schema is documented in
   `config.go`.
2. Add a unit test (or extend an existing one in `detector_test.go` /
   `extraction_test.go`) that exercises the new bridge.
3. Update the coverage matrix in `README.md`.
4. Open a PR with `feat(config): add <bridge> on <chain>`.
