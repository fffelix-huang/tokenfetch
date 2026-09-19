# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`tokenfetch`: neofetch/fastfetch-style CLI summarizing AI token usage + estimated cost. Module `github.com/fffelix-huang/tokenfetch`.

- v1: Claude Code only. Keep provider-agnostic — new agents (Codex, Gemini CLI, …) = new `usage.Source`, no changes to store/report/render.
- Breakdowns: local hour (hour-of-day, weekday×hour heatmap), model, project, skill, plugin, subagent.
- Planned: macOS menubar as thin client over `tokenfetch --json` (no usage logic outside Go); Homebrew via `git@github.com:fffelix-huang/homebrew-tap.git`.

## Commands

```sh
make build    # bin/tokenfetch, --version shows the commit
make test
make lint     # gofmt + go vet; CI runs make lint/test/build
make clean
go run ./cmd/tokenfetch [--today|--week|--month|--all] [--json] [--rebuild]
go test ./...
go test ./internal/ingest -run TestIncrementalDedup   # single test
go vet ./...
TOKENFETCH_DATA_DIR=/tmp/tf go run ./cmd/tokenfetch   # isolated database; never test against the real one
```

## Architecture

Pipeline: `usage.Source → ingest → store (SQLite) → report → render`

- `cmd/tokenfetch` — dir name = binary name for `go install …/cmd/tokenfetch@latest`.
- `internal/usage` — `Event` (normalized record) + `Source` interface. Only `internal/source/<provider>` knows log formats.
- `internal/ingest` — incremental: per-file offset/size/mtime in store; only newline-terminated lines consumed (half-written tail picked up next run). Events from deleted logs stay in the DB — Claude Code prunes transcripts (`cleanupPeriodDays`, default 30), so the DB is the only long-term history.
- `internal/store` — `modernc.org/sqlite` (no cgo), WAL, at `~/Library/Application Support/tokenfetch/tokenfetch.db` (XDG data dir on Linux, `%AppData%` on Windows; `TOKENFETCH_DATA_DIR` overrides). **Not a cache: never drop/wipe `events`.** Schema changes = append to `migrations` (user_version). `--rebuild` only clears `files` offsets. Old `~/Library/Caches/tokenfetch` DB is auto-moved. Upsert on `(provider, message_id)` keeping max `output`, refreshing all columns. Queries group into 15-min slots (+5:30/+5:45 zones need it to map to local hours).
- `internal/pricing` — hardcoded list prices + `Normalize` for id variants (date suffix, `[1m]`, Bedrock/Vertex forms). Cost computed at report time, never stored. Unknown model → excluded from cost and listed in `unpriced_models`. Update table from https://platform.claude.com/docs/en/about-claude/pricing (fast mode, 1h vs 5m cache write, 0.025x Fable 5.1 cache read, 1.1x `inference_geo: us` on 4.6+, web search $10/1k).
- `internal/report` — builds `Report`; its JSON is the menubar contract, keep field names stable.
- `cmd/tokenfetch` `usageText` — hand-written `--help`; when adding flags also update `internal/completion/completion.{bash,zsh,fish}` (embedded, printed by `--bash/--zsh/--fish`; Homebrew formula installs them via `generate_completions_from_executable(..., shell_parameter_format: :flag)`). bash script must work on macOS bash 3.2 (no split on `=` in `COMP_WORDS`).
- `--color=auto|always|never`: auto honors `NO_COLOR`; always forces ANSI256/TrueColor even when piped.
- `internal/render` — lipgloss. Stats column left, top models/projects/skills/plugins right (rows shared round-robin to match stats height; stacks if terminal too narrow). Main chart per range: `--today` hour bars, `--week` day bars, `--month` Sun-first calendar, `--all` (default) past-year Sun-first grid; non-today ranges also get hour-of-day bars. Hour chart is mirrored: am bars grow up, pm bars grow down, shared `12,1..11` axis, one peak for both halves. All bar charts use `labeledBars`: cost then tokens hug each bar's end, inside its own 6-wide column. `fitTokens`/`fitMoney` use as many decimals as fit the column (tokens ≤3, cost ≤2; cost switches to K/M only when the plain amount can't fit). Month + all use fixed daily tiers (0, <10M, <100M, <1B, ≥1B). Heat ramp is orange-only (user rejected red), on xterm-256 cube colors so steps stay distinct (`TestHeatLevelsDistinct`); solid `██` on 256/truecolor, shade glyphs otherwise.

## Claude Code log format (verified CC 2.1.273, undocumented — parse leniently)

- `$CLAUDE_CONFIG_DIR` (comma-separated) or `~/.claude` + `~/.config/claude`, then `projects/<cwd, / → ->/<sessionId>.jsonl`; subagents in `<sessionId>/subagents/agent-<id>.jsonl`. Dir names start with `-` — beware in shell (`find ... -print0`, `--`).
- Only `type=="assistant"` lines with `message.usage` count; skip model `<synthetic>`.
- One response = several lines sharing `message.id`, early ones with partial `output_tokens` → dedup keep max. No message id spans files.
- Project = line's `cwd` (dir-name encoding is lossy). Attribution fields top-level: `attributionSkill`, `attributionPlugin`, `attributionAgent` (null = main thread).
- `usage.cache_creation.ephemeral_{5m,1h}_input_tokens` split; older lines lack it (remainder billed as 5m) and lack `speed`/`iterations`. Don't add `iterations[]` on top of top-level counts. `inference_geo: "not_available"` = none.
- No cost field in logs. `~/.claude/stats-cache.json` has lifetime `modelUsage` + `dailyModelTokens` (covers pruned transcripts) but sums every streamed line without dedup (~1.9x overcount) and buckets by UTC date — not a reliable source.
- Cross-check totals: jq over all files, `group_by(.message.id) | map(max_by(.message.usage.output_tokens))`, sum — must equal `tokenfetch --all --json`.

## Release

- Manual: bump `var version` in `cmd/tokenfetch/main.go`, commit, `git tag -s vX.Y.Z -m vX.Y.Z`, `git push origin master vX.Y.Z` → `.github/workflows/release.yml`:
  1. `release` job: fails if tag ≠ `var version`, tests, GoReleaser (`.goreleaser.yml`) → GitHub Release with darwin/linux × amd64/arm64 tarballs. No Homebrew config in GoReleaser.
  2. `homebrew` job: renders `.github/homebrew/tokenfetch.rb` (`@VERSION@`, `@SHA256@` of the tag's source tarball) and pushes `Formula/tokenfetch.rb` to `fffelix-huang/homebrew-tap` (secret `HOMEBREW_TAP_TOKEN`). Formula builds from source, not a cask — no quarantine/xattr handling needed.
- Version (fzf style): `var version = "X.Y.Z"`, `var revision = "devel"`. GoReleaser sets `-X main.revision={{.ShortCommit}}`; the formula sets `-X main.revision=#{tap.user}` (fzf style: `fffelix-huang` from our tap). `--version` prints `0.1.0 (devel|abc1234|fffelix-huang)`.
- Dry run: Actions → Release → Run workflow (snapshot, publishes nothing, skips homebrew job).
- v2+ requires module path `/v2` suffix.
