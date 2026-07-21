# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

AP Chain is a Go CLI that scrapes a list of URLs, runs the content through a Gemini MapReduce pipeline to produce a deduplicated, source-attributed structured document, and publishes the result (Markdown + HTML) to local disk or GCS with a signed URL, notifying Slack on completion. Single command: `generate`.

## Commands

```bash
go build ./...                 # build
go vet ./...                   # static checks
go run ./main.go generate -i urls.txt -o ./output/output.md
```

There are currently no `_test.go` files in this repository — `go test ./...` runs but exercises nothing. If you add tests, `go test ./internal/<pkg>/... -run TestName -v` runs a single one.

Required environment for running `generate`: either `GEMINI_API_KEY` or `GCP_PROJECT_ID` (Vertex AI auth) must be set; `SLACK_WEBHOOK_URL` is optional and enables completion notifications. See `internal/config/config.go` for the full env var list (`GEMINI_MODEL`, `GEMINI_QUALITY_MODEL`, `MAX_CONCURRENCY`, `RATE_INTERVAL_SEC`).

No linter config or CI workflow is checked into this repo; `cloudbuild.yaml` builds the Docker image via Buildx and deploys it as a Cloud Run Job.

## Architecture

Layered, interface-driven DI structure — every cross-package dependency is an interface defined by the *consumer* package, satisfied by an adapter elsewhere. When changing a signature, grep for the interface definition (usually in `domain/`, `pipeline/`, or as an unexported interface next to the struct that uses it) rather than assuming the concrete type is the contract.

```
cmd/            Cobra commands (root.go wires global flags via clibase; generate.go is the only subcommand)
internal/config Config struct + env/flag defaults (single source of truth for defaults — see below)
internal/app    Container: the DI struct holding everything built at startup, plus its Close()
internal/builder Constructs the Container and wires runners/adapters into a pipeline.Pipeline (app.go, io.go, pipeline.go, runners.go)
internal/domain Shared plain types (Request, URLResult, Segment, PublishResult) and the top-level Pipeline/Notifier interfaces
internal/pipeline Orchestrator: Execute() runs Collect -> Compose -> Publish, then notifies Slack (success or failure) with a detached context so notification isn't cut short by request cancellation
internal/runner  Business logic per phase (CollectRunner, ComposeRunner, PublishRunner) — depends only on small interfaces (ContentReader, Composer, mdRunner, etc.), not on concrete adapters
internal/adapters Wraps external libraries behind those interfaces: ai.go (Gemini client), composer.go (MapReduce execution against the Gemini client), prompt.go (renders assets/prompts/*.md via go-prompt-kit), slack.go
assets/         Embedded prompt templates (prompt_map.md, prompt_reduce.md), loaded via go:embed + go-prompt-kit's resource.Load with the `prompt_` filename-prefix convention
```

Request flow (see `README.md` for the full mermaid sequence diagram):
1. `cmd.generateCommand` builds the `app.Container` via `builder.BuildContainer`, then calls `Pipeline.Execute`.
2. **Collect** (`runner.CollectRunner`): reads the input file/GCS object, extracts and validates URLs (`securenet.IsSafeURL`), scrapes them concurrently via `go-web-exact`, and filters out failed/empty results.
3. **Compose** (`runner.ComposeRunner`): splits each URL's content into segments (`segmentText`, character-count based — see note below), runs the **Map** phase in parallel per segment (`ComposerAdapter.RunMap`, bounded by `maxConcurrency`/`rateInterval`), then the **Reduce** phase (`RunReduce`) to merge intermediate summaries into one structured Markdown document.
4. **Publish** (`runner.PublishRunner`): writes Markdown and HTML (converted via `go-prompt-kit/md/builder`) to the output URI, generates signed URLs if a `remoteio.URLSigner` is configured.
5. `pipeline.Pipeline` sends a Slack success/failure notification regardless of which step failed.

### Notable design points

- **Defaults live in `internal/config`, not the consuming package.** `config.DefaultMaxConcurrency` / `config.DefaultRateInterval` are the canonical values; `adapters.ComposerAdapter` imports `config` rather than redefining its own constants. Keep new tunables following this pattern instead of duplicating a default in the adapter.
- **`segmentText` (`internal/runner/compose.go`) is deliberately character-count based, not token-based.** `go-gemini-client`'s `CountTokens`/`CountTokensWithParts` (available since v1.13.0) make a real network call with retries — using them per split-candidate would add significant latency/cost and compete with the same rate limiter used for Map/Reduce calls. This was evaluated and intentionally rejected; don't "fix" it without discussing the tradeoff.
- **This repo is one of a family of `github.com/shouni/*` Go modules** (`go-gemini-client`, `go-web-exact`, `go-web-reader`, `go-remote-io`, `go-notifier`, `go-prompt-kit`, `go-utils`, `netarmor`, `clibase`, `go-http-kit`) that are versioned and released independently. If a bug appears to originate in one of these (not in `internal/`), check whether the library has a local checkout as a sibling directory before assuming it must be patched by pinning `replace` in `go.mod`.
- Builder functions in `internal/builder/runners.go` only take `ctx context.Context` when they actually use it (`buildComposer`, for the Gemini client init). `buildCollector`/`buildPublisher` don't — don't add it back "for consistency" without a real caller.
- Init-failure errors in `internal/builder` are wrapped via the shared `wrapInitErr(name, err)` helper (`internal/builder/errors.go`) rather than ad hoc `fmt.Errorf` per call site.
