package builder

import (
	"context"
	"fmt"

	mdBuilder "github.com/shouni/go-prompt-kit/md/builder"
	"github.com/shouni/go-remote-io/remoteio"
	scraperBuilder "github.com/shouni/go-web-exact/v2/builder"
	"github.com/shouni/go-web-exact/v2/scraper"
	"github.com/shouni/go-web-reader/pkg/reader"

	"ap-chain/internal/adapters"
	"ap-chain/internal/app"
	"ap-chain/internal/runner"
)

// buildCollector は、CollectRunner のインスタンスを構築して返します。
func buildCollector(ctx context.Context, appCtx *app.Container) (*runner.CollectRunner, error) {
	contentReader, err := reader.New(
		reader.WithGCSFactory(func(ctx context.Context) (remoteio.ReadWriteFactory, error) {
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
		return nil, fmt.Errorf("Scraperの初期化に失敗しました: %w", err)
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
		return nil, fmt.Errorf("AIAdapterの初期化に失敗しました: %w", err)
	}
	promptBuilder, err := adapters.NewPromptAdapter()
	if err != nil {
		return nil, fmt.Errorf("PromptAdapterの初期化に失敗しました: %w", err)
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
		return nil, fmt.Errorf("Composer Adapterの初期化に失敗しました: %w", err)
	}

	composer, err := runner.NewComposeRunner(appCtx.Config, composerAdapter)
	if err != nil {
		return nil, fmt.Errorf("Composerの初期化に失敗しました: %w", err)
	}

	return composer, nil
}

// buildPublisher は、Publisher のインスタンスを構築して返します。
func buildPublisher(ctx context.Context, appCtx *app.Container) (*runner.PublishRunner, error) {
	b, err := mdBuilder.New(
		mdBuilder.WithEnableUnsafeHTML(false),
	)
	if err != nil {
		return nil, fmt.Errorf("Markdown Builderの初期化に失敗: %w", err)
	}
	md, err := b.BuildRunner()
	if err != nil {
		return nil, fmt.Errorf("MarkdownToHtmlRunnerの構築に失敗: %w", err)
	}

	return runner.NewPublishRunner(
		appCtx.RemoteIO.Writer,
		appCtx.RemoteIO.Signer,
		md,
	), nil
}
