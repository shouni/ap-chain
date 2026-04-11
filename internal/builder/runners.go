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
	"ap-chain/internal/cleaner"
	"ap-chain/internal/domain"
	"ap-chain/internal/runner"
)

// buildFetchRunner は、FetchRunner のインスタンスを返します。
func buildFetchRunner(ctx context.Context, appCtx *app.Container) (domain.FetchRunner, error) {
	contentReader, err := reader.New(
		reader.WithGCSFactory(func(ctx context.Context) (remoteio.ReadWriteFactory, error) {
			return appCtx.RemoteIO.Factory, nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize content reader: %w", err)
	}

	opts := []scraper.Option{
		scraper.WithMaxConcurrency(appCtx.Config.Concurrency),
	}
	sb, err := scraperBuilder.New(appCtx.HTTPClient, opts)
	if err != nil {
		return nil, fmt.Errorf("ScrapeRunnerの初期化に失敗しました: %w", err)
	}

	return runner.NewFetchRunner(
		contentReader,
		sb.ScrapeRunner(),
	), nil
}

// buildCleanRunner は、CleanRunner のインスタンスを返します。
func buildCleanRunner(ctx context.Context, appCtx *app.Container) (domain.CleanRunner, error) {
	ai, err := adapters.NewAIAdapter(ctx, appCtx.Config)
	if err != nil {
		return nil, fmt.Errorf("AIAdapterの初期化に失敗しました: %w", err)
	}
	promptBuilder, err := adapters.NewPromptAdapter()
	if err != nil {
		return nil, fmt.Errorf("PromptAdapterの初期化に失敗しました: %w", err)
	}
	executor, err := cleaner.NewLLMConcurrentExecutor(ai, promptBuilder, appCtx.Config.Concurrency)
	if err != nil {
		return nil, fmt.Errorf("cleanerの初期化に失敗しました: %w", err)
	}

	models := cleaner.LLMModels{
		Map:    appCtx.Config.MapModel,
		Reduce: appCtx.Config.ReduceModel,
	}
	llmCleaner, err := cleaner.NewCleaner(executor, models)
	if err != nil {
		return nil, fmt.Errorf("cleanerの初期化に失敗しました: %w", err)
	}

	return runner.NewCleanRunner(
		llmCleaner,
	), nil
}

// buildPublishRunner は、PublishRunner のインスタンスを返します。
func buildPublishRunner(ctx context.Context, appCtx *app.Container) (domain.PublishRunner, error) {
	b, err := mdBuilder.New(
		mdBuilder.WithEnableUnsafeHTML(false),
	)
	if err != nil {
		return nil, fmt.Errorf("markdown Builderの初期化に失敗: %w", err)
	}
	md, err := b.BuildRunner()
	if err != nil {
		return nil, fmt.Errorf("MarkdownToHtmlRunnerの構築に失敗: %w", err)
	}

	return runner.NewPublisherRunner(
		appCtx.RemoteIO.Writer,
		appCtx.RemoteIO.Signer,
		md,
	), nil
}
