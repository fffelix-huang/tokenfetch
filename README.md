# tokenfetch

<div align="center">
  <a href="https://github.com/fffelix-huang/tokenfetch/actions"><img src="https://github.com/fffelix-huang/tokenfetch/actions/workflows/ci.yml/badge.svg?branch=master" alt="Build Status"></a>
  <a href="https://github.com/fffelix-huang/tokenfetch/releases"><img src="https://img.shields.io/github/v/tag/fffelix-huang/tokenfetch" alt="Version"></a>
  <a href="https://github.com/fffelix-huang/tokenfetch?tab=MIT-1-ov-file#readme"><img src="https://img.shields.io/github/license/fffelix-huang/tokenfetch" alt="License"></a>
  <a href="https://github.com/fffelix-huang/tokenfetch/graphs/contributors"><img src="https://img.shields.io/github/contributors/fffelix-huang/tokenfetch" alt="Contributors"></a>
  <a href="https://github.com/fffelix-huang/tokenfetch/stargazers"><img src="https://img.shields.io/github/stars/fffelix-huang/tokenfetch?style=flat" alt="Stars"></a>
</div>

Tokenfetch gives summary of Claude Code token usage and estimated API cost.

## Features

- Token totals split into uncached input, cache read, cache write and output
- Estimated cost at Anthropic API list prices (fast mode, 5m/1h cache writes and data residency included)
- Breakdown by model, project, skill and plugin
- Activity heatmap, per-day bars, calendar, and hour-of-day charts
- History survives Claude Code's transcript cleanup
- `--json` output for scripts and other tools

## Install

Homebrew (macOS, Linux):

```sh
brew install fffelix-huang/tap/tokenfetch
```

Go:

```sh
go install github.com/fffelix-huang/tokenfetch/cmd/tokenfetch@latest
```

Prebuilt binaries are on the [releases page](https://github.com/fffelix-huang/tokenfetch/releases).

## Usage

```sh
tokenfetch            # everything recorded, activity over the past year
tokenfetch --month    # last 30 days, calendar view
tokenfetch --week     # last 7 days, per-day bars
tokenfetch --today    # since local midnight, per-hour bars
tokenfetch --json     # machine-readable report
tokenfetch --help
```

## How it works

Tokenfetch reads Claude Code session transcripts (`~/.claude/projects/**/*.jsonl`), deduplicates streamed responses, and stores one record per model response in a local SQLite database. Each run only reads what was appended since the last run.

**Cost is an estimate.** It applies [Anthropic API list prices](https://platform.claude.com/docs/en/about-claude/pricing) to recorded tokens. On a Pro or Max subscription you are not billed per token; the figure shows what the same usage would cost on the API.

**Keep your history.** Claude Code deletes transcripts that haven't been modified for `cleanupPeriodDays` (default 30). tokenfetch keeps everything it has already read, but it can't recover transcripts deleted before it ran. Run it regularly, or keep transcripts longer in `~/.claude/settings.json`:

```json
{
  "cleanupPeriodDays": 365
}
```

## Development

```sh
go build -o bin/tokenfetch ./cmd/tokenfetch
go test ./...
TOKENFETCH_DATA_DIR=/tmp/tokenfetch go run ./cmd/tokenfetch   # use a throwaway database
```

### Release

1. Bump `version` in `cmd/tokenfetch/main.go` and commit.
2. `git tag vX.Y.Z && git push origin vX.Y.Z`

The release workflow checks the tag matches `version`, then [GoReleaser](https://goreleaser.com) builds macOS and Linux binaries and publishes the GitHub release, and the Homebrew formula in [fffelix-huang/homebrew-tap](https://github.com/fffelix-huang/homebrew-tap) is updated to the new tag.
