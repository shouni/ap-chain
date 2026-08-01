# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

AP Chain is a Go CLI that scrapes a list of URLs, runs the content through a Gemini MapReduce pipeline to produce a deduplicated, source-attributed structured document, and publishes the result (Markdown, deterministically rendered from the AI's structured JSON output) to local disk or GCS with a signed URL, notifying Slack on completion. Single command: `generate`.

## Commands

```bash
go build ./...                 # build
go vet ./...                   # static checks
go test -race ./...            # what CI runs
golangci-lint run              # config in .golangci.yml; CI pins v2.12.2
go test ./internal/config/ -run TestLoadConfig -v   # single test
go run ./main.go generate -i urls.txt -o ./output/output.md
```

Required environment for running `generate`: either `GEMINI_API_KEY` or `GCP_PROJECT_ID` (Vertex AI auth) must be set; `SLACK_WEBHOOK_URL` is optional and enables completion notifications. See `internal/config/config.go` for the full env var list (`GEMINI_MODEL`, `GEMINI_QUALITY_MODEL`, `MAX_CONCURRENCY`, `RATE_INTERVAL_SEC`).

CI (`.github/workflows/ci.yml`) runs build/vet/gofmt/race-test, golangci-lint, and govulncheck on pushes and PRs to `main` and `develop`. `cloudbuild.yaml` builds the Docker image via Buildx and deploys it as a Cloud Run Job.

Coverage is uneven by design: `internal/config` and `internal/pipeline` are at 100%, `internal/runner` and `internal/adapters` cover the pure logic (segmentation, Markdown rendering, prompt building), and `internal/builder` / `internal/app` are untested because they only wire real GCS and Gemini clients together.

## Architecture

Layered, interface-driven DI structure — every cross-package dependency is an interface defined by the *consumer* package, satisfied by an adapter elsewhere. When changing a signature, grep for the interface definition (usually in `domain/`, `pipeline/`, or as an unexported interface next to the struct that uses it) rather than assuming the concrete type is the contract.

```
cmd/            Cobra commands (root.go wires global flags via clibase; generate.go is the only subcommand)
internal/config Config struct + env/flag defaults (single source of truth for defaults — see below)
internal/app    Container: the DI struct holding everything built at startup, plus its Close()
internal/builder Constructs the Container and wires runners/adapters into a pipeline.Pipeline (app.go, io.go, pipeline.go, runners.go)
internal/domain Shared plain types (Request, URLResult, Segment, PublishResult) and the top-level Pipeline/Notifier interfaces
internal/pipeline Orchestrator: Execute() runs Collect -> Compose -> Publish, then notifies Slack (success or failure) with a detached context so notification isn't cut short by request cancellation
internal/runner  Business logic per phase (CollectRunner, ComposeRunner, PublishRunner) — depends only on small interfaces (ContentReader, Composer, converter, etc.), not on concrete adapters
internal/adapters Wraps external libraries behind those interfaces: ai.go (Gemini client), composer.go (MapReduce execution, ResponseSchema-constrained JSON calls against the Gemini client), schema.go (map/reduce output schemas), markdown.go (deterministic JSON→Markdown rendering), prompt.go (renders assets/prompts/*.md via go-prompt-kit), slack.go
assets/         Embedded prompt templates (prompt_map.md, prompt_reduce.md), loaded via go:embed + go-prompt-kit's resource.Load with the `prompt_` filename-prefix convention
```

Request flow (see `README.md` for the full mermaid sequence diagram):
1. `cmd.generateCommand` builds the `app.Container` via `builder.BuildContainer`, then calls `Pipeline.Execute`.
2. **Collect** (`runner.CollectRunner`): reads the input file/GCS object, extracts and validates URLs (`securenet.IsSafeURL`), scrapes them concurrently via `go-web-exact`, and filters out failed/empty results.
3. **Compose** (`runner.ComposeRunner`): splits each URL's content into segments (`segmentText`, character-count based — see note below), runs the **Map** phase in parallel per segment (`ComposerAdapter.RunMap`, bounded by `maxConcurrency`/`rateInterval`) — each call is `ResponseSchema`-constrained to `{cleaned_text}` (see `internal/adapters/schema.go`), with the source URL kept from the original `domain.Segment` rather than asked of the model — then the **Reduce** phase (`RunReduce`) merges the cleaned segments (passed as a JSON array, not string-joined) into one `{title, sections: [{heading, body, source_urls}]}` JSON document.
4. **Publish** (`runner.PublishRunner`): renders the Reduce phase's JSON deterministically into Markdown (`adapters.MarkdownConverter`, no HTML/goldmark involved) and writes it to the output URI, generating a signed URL if a `remoteio.URLSigner` is configured.
5. `pipeline.Pipeline` sends a Slack success/failure notification regardless of which step failed.

### Notable design points

- **Defaults live in `internal/config`, not the consuming package.** `config.DefaultMaxConcurrency` / `config.DefaultRateInterval` are the canonical values; `adapters.ComposerAdapter` imports `config` rather than redefining its own constants. Keep new tunables following this pattern instead of duplicating a default in the adapter.
- **`segmentText` (`internal/runner/compose.go`) is deliberately character-count based, not token-based.** `go-gemini-client`'s `CountTokens`/`CountTokensWithParts` (available since v1.13.0) make a real network call with retries — using them per split-candidate would add significant latency/cost and compete with the same rate limiter used for Map/Reduce calls. This was evaluated and intentionally rejected; don't "fix" it without discussing the tradeoff.
- **This repo is one of a family of `github.com/shouni/*` Go modules** (`go-gemini-client`, `go-web-exact`, `go-web-reader`, `go-remote-io`, `go-notifier`, `go-prompt-kit`, `go-utils`, `netarmor`, `clibase`, `go-http-kit`) that are versioned and released independently. If a bug appears to originate in one of these (not in `internal/`), check whether the library has a local checkout as a sibling directory before assuming it must be patched by pinning `replace` in `go.mod`.
- Builder functions in `internal/builder/runners.go` only take `ctx context.Context` when they actually use it (`buildComposer`, for the Gemini client init). `buildCollector`/`buildPublisher` don't — don't add it back "for consistency" without a real caller.
- Init-failure errors in `internal/builder` are wrapped via the shared `wrapInitErr(name, err)` helper (`internal/builder/errors.go`) rather than ad hoc `fmt.Errorf` per call site. The one remaining `fmt.Errorf` (`io.go`, the nil-factory check) is a precondition, not an init failure.
- **`internal/adapters` deliberately does not import `google.golang.org/genai`.** `gemini.Schema` is an alias for `genai.Schema` and `MultimodalGenerator.GenerateWithAttachments` is the genai-free entry point for text generation, so schemas and calls are written against `go-gemini-client` alone. That also keeps the client mock to a single method. Don't reintroduce `genai.Part` unless multimodal input actually needs Part-level control.
- **`pipeline.Execute` registers its failure-notification defer before validating the request.** Running as a Cloud Run Job means nobody reads stdout, so an input-validation failure has to reach Slack too.
- **`config.envNonEmpty` treats an env var that is set but empty as unset.** `envutil.GetEnv` uses `os.LookupEnv` and returns the empty value; `cloudbuild.yaml` passes `GEMINI_MODEL=${_GEMINI_MODEL}` unconditionally, so an empty substitution would otherwise wipe out the default model name.
- **`gemini.GenerateOptions.ResponseSchema` requires `ResponseMIMEType: "application/json"` to be set explicitly alongside it** — Gemini rejects the call with `Error 400: Response_schema with a response mime type 'text/plain' is unsupported` otherwise. Both `RunMap` and `RunReduce` in `internal/adapters/composer.go` set both fields together; don't drop one when touching this code.
- The `title`/`sections[].heading`/`.body` fields from the Reduce phase are rendered straight into Markdown by `adapters.MarkdownConverter` (`# `/`## ` headings, plain paragraphs). `prompt_reduce.md` explicitly tells the model `body` must be plain text with no Markdown syntax — the model's Markdown decorations would otherwise show up literally (e.g. a literal `**word**`) rather than being interpreted.
