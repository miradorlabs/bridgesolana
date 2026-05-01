# CLAUDE.md

Project-specific guidance for Claude. Keep this file lean — it loads into
every conversation. Codebase architecture lives in the code; this file is
for conventions and workflow.

## Project

`bridgesolana` is a Go library that identifies cross-chain bridge events in
Solana transaction logs. Build a `BridgeDetector` once per chain, hand it
the raw program log strings, and get back either `BridgeDetails`
(self-contained, log mode) or `InstructionDetection` records that need RPC
follow-up to extract the correlation ID (instruction mode).

- Module path: `github.com/miradorlabs/bridgesolana` (matches repo URL).
- Single Go module at the repo root. No `cmd/` — pure library.
- Go 1.26.1 (see `go.mod`).
- License: MIT.
- Status: pre-1.0. Breaking changes are allowed in `0.x.0` minor bumps;
  `0.x.y` patch releases never break callers.

Bridge configs are embedded JSON under `config/<chain>/*.json` and loaded
once via `sync.Once`. Currently only `config/solana/cctp.json` exists —
Circle CCTP V1 and V2 source/destination legs.

## Detection modes

The detector handles two log shapes that occur in Solana transactions:

- **Log mode** — `Program data: <base64>` lines emitted by Anchor `emit!`
  macros. The detector matches the leading 8-byte Anchor discriminator and
  extracts the correlation ID directly from the decoded bytes.
- **Instruction mode** — `Program log: Instruction: <Name>` lines emitted
  by the runtime when a program enters an instruction. The detector matches
  on `(programID, instructionName)` while tracking the program invocation
  stack, so that nested CPI calls are scoped to the right program. The
  correlation ID lives off-log: callers must read the relevant `MessageSent`
  account (source) or `ReceiveMessage` instruction data (destination) via
  RPC, then call `ParseMessageSentAccount` /
  `ParseReceiveMessageInstructionData` followed by
  `ExtractCorrelationFields`.

If you add a new bridge that uses Anchor `emit!`, prefer log mode. If it
fires bridge intent through CPI without emitting an event, instruction
mode is the only option.

## Commit conventions

**Conventional Commits are required** — release-please parses commit
messages on `main` to compute the next SemVer bump and regenerate
`CHANGELOG.md`. The `commitlint` workflow validates PR commits.

Allowed types and how they appear in releases:

| Type        | Section                  | Bump (pre-1.0) |
|-------------|--------------------------|----------------|
| `feat`      | Features                 | minor          |
| `fix`       | Bug Fixes                | patch          |
| `perf`      | Performance Improvements | patch          |
| `refactor`  | Code Refactoring         | patch          |
| `deps`      | Dependencies             | patch          |
| `docs`      | Documentation            | patch          |
| `ci`        | (hidden)                 | none           |
| `build`     | (hidden)                 | none           |
| `test`      | (hidden)                 | none           |
| `chore`     | (hidden)                 | none           |
| `style`     | (hidden)                 | none           |
| `revert`    | (hidden)                 | none           |

Breaking changes use `type!:` or a `BREAKING CHANGE:` footer. While we're
pre-1.0, release-please bumps these as **minor**, not major (configured
via `bump-minor-pre-major: true`).

### Commit-message body rules

release-please v17 uses the spec-strict `@conventional-commits/parser`,
which tokenizes the entire message. A single bad body can prevent a
release PR from being generated at all. Rules:

- **No nested parens anywhere in the message.** `foo(bar(baz))` in a body
  line will fail. Rewrite as `foo of bar of baz`, or split across lines.
- **No unmatched parens.**
- **Don't start a body line with `<word>:` where `<word>` looks like a
  conventional-commit type** (`feat:`, `fix:`, etc.) — the parser may
  read it as a second header.
- **Squash-merge subjects must themselves be conventional.** Set the PR
  title to a clean `type: subject` before merging.

### Dependabot commits

For Dependabot bumps to trigger patch releases, `.github/dependabot.yml`
must use `commit-message.prefix: deps` for both `gomod` and
`github-actions` ecosystems.

### DCO sign-off

Every commit must be signed off (`Signed-off-by: ...` trailer). Always
commit with `git commit -s`. The `dco` GitHub workflow blocks PRs without
trailers on every commit.

### Authorship

**Solo author only.** Do not add `Co-Authored-By:` trailers (including
Claude). The DCO sign-off is the only trailer.

## Pre-push checks

Install hooks once: `make setup-hooks` (copies `.githooks/pre-push` and
`.githooks/commit-msg` into `.git/hooks/`).

Before pushing, run `make verify` — `go fmt`, `go vet`, `golangci-lint
run`, and `go test ./...`. CI runs the same set on every PR.

## Linter

`golangci-lint` is pinned to a v2 release in two places that **must stay
in sync**:

- `Makefile` → `GOLANGCI_LINT_VERSION` (used by `make tools`).
- `.github/workflows/ci.yml` → `version:` on `golangci-lint-action@v9`.

If CI fails with *"the Go language version (goX.Y) used to build
golangci-lint is lower than the targeted Go version"*, bump to a newer v2
release in **both** files.

`.golangci.yml` uses the v2 schema (`version: "2"`, top-level
`formatters:`, `linters: settings:`).

## Releases

Tagging and CHANGELOG generation are automated by **release-please**. The
action runs on every push to `main`:

1. Scans conventional-commit messages since the last tag.
2. Opens (or updates) a long-lived release PR — `chore: release vX.Y.Z`
   — that bumps `.release-please-manifest.json` and regenerates
   `CHANGELOG.md`.
3. Merging that PR tags `vX.Y.Z` and creates the GitHub Release.

**Do not hand-edit `CHANGELOG.md`** — release-please owns it.

The git tag is the source of truth for the module version. There is no
`version.go` constant.

## Testing

- `go test ./...` — unit tests.
- `go test -race ./...` — race detector. CI runs this.

Tests cover detector dispatch (`detector_test.go`) and payload extraction
(`extraction_test.go`). The keccak256 fixtures in
`TestExtractCorrelationFields_Keccak256` are real on-chain CCTP V2
messages from Solana → Base and Solana → Arbitrum transfers; the expected
hashes were verified independently against the EVM `MessageReceived`
topic.

## Adding a new bridge on Solana

1. Add the program ID and event/instruction definition to
   `config/solana/<bridge>.json`. The schema is documented in `config.go`.
2. Extend `detector_test.go` with a log fixture for the new bridge.
3. Update the coverage matrix in `README.md`.
4. Open a PR with `feat(config): add <bridge> on solana`.

## Adding a new chain

1. Create `config/<chain>/<bridge>.json` files.
2. The embed directive `//go:embed config/*/*.json` in `config.go` will
   pick them up automatically.
3. Update the coverage matrix in `README.md`.
