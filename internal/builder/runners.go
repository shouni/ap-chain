package builder

import (
	"context"
	"fmt"

	"github.com/shouni/go-remote-io/remoteio"
	scraperBuilder "github.com/shouni/go-web-exact/v2/builder"
	"github.com/shouni/go-web-exact/v2/scraper"
	"github.com/shouni/go-web-reader/pkg/reader"

	"ap-chain/internal/adapters"
	"ap-chain/internal/app"
	"ap-chain/internal/runner"
)

// buildCollector は、CollectRunner のインスタンスを構築して返します。
func buildCollector(appCtx *app.Container) (*runner.CollectRunner, error) {
	contentReader, err := reader.New(
		reader.WithGCSFactory(func(_ context.Context) (remoteio.IOFactory, error) {
			return appCtx.RemoteIO.Factory, nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize content reader: %w", err)
	}

	opts := []scraper.Option{
		scraper.WithMaxConcurrency(appCtx.Config.MaxScraperParallel),
	}
	sb, err := scraperBuilder.New(appCtx.HTTPClient, opts)
	if err != nil {
		return nil, wrapInitErr("Scraper", err)
	}

	return runner.NewCollectRunner(
		contentReader,
		sb.ScrapeRunner(),
	), nil
}

// buildComposer は、Composer のインスタンスを構築して返します。
func buildComposer(ctx context.Context, appCtx *app.Container) (*runner.ComposeRunner, error) {
	ai, err := adapters.NewAIAdapter(ctx, appCtx.Config)
	if err != nil {
		return nil, wrapInitErr("AIAdapter", err)
	}
	promptBuilder, err := adapters.NewPromptAdapter()
	if err != nil {
		return nil, wrapInitErr("PromptAdapter", err)
	}
	opts := []adapters.ComposerOption{
		adapters.WithMaxConcurrency(appCtx.Config.MaxConcurrency),
		adapters.WithRateInterval(appCtx.Config.RateInterval),
	}
	composerAdapter, err := adapters.NewComposerAdapter(
		ai,
		promptBuilder,
		opts...,
	)
	if err != nil {
		return nil, wrapInitErr("Composer Adapter", err)
	}

	models := runner.Models{
		MapModel:    appCtx.Config.MapModel,
		ReduceModel: appCtx.Config.ReduceModel,
	}
	composer, err := runner.NewComposeRunner(composerAdapter, models)
	if err != nil {
		return nil, wrapInitErr("Composer", err)
	}

	return composer, nil
}

// buildPublisher は、Publisher のインスタンスを構築して返します。
func buildPublisher(appCtx *app.Container) (*runner.PublishRunner, error) {
	return runner.NewPublishRunner(
		appCtx.RemoteIO.Writer,
		appCtx.RemoteIO.Signer,
		adapters.NewMarkdownConverter(),
	), nil
}
