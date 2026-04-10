package builder

import (
	"context"
	"fmt"

	mdBuilder "github.com/shouni/go-prompt-kit/md/builder"
	scraperBuilder "github.com/shouni/go-web-exact/v2/builder"
	"github.com/shouni/go-web-exact/v2/scraper"

	"ap-chain/internal/adapters"
	"ap-chain/internal/app"
	"ap-chain/internal/cleaner"
	"ap-chain/internal/domain"
	"ap-chain/internal/runner"
)

// buildFetchRunner は、FetchRunner のインスタンスを返します。
func buildFetchRunner(ctx context.Context, appCtx *app.Container) (domain.FetchRunner, error) {
	opts := []scraper.Option{
		scraper.WithMaxConcurrency(appCtx.Config.Concurrency),
	}
	sb, err := scraperBuilder.New(appCtx.HTTPClient, opts)
	if err != nil {
		// 失敗時はFactoryを閉じる
		return nil, fmt.Errorf("ScrapeRunnerの初期化に失敗しました: %w", err)
	}

	return runner.NewFetchRunner(
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
	// LLMExecutor の構築
	executor, err := cleaner.NewLLMConcurrentExecutor(appCtx.Config, ai, promptBuilder)
	if err != nil {
		return nil, fmt.Errorf("cleanerの初期化に失敗しました: %w", err)
	}
	cleanerRunner, err := cleaner.NewCleaner(promptBuilder, executor)
	if err != nil {
		return nil, fmt.Errorf("cleanerの初期化に失敗しました: %w", err)
	}

	return runner.NewCleanRunner(
		cleanerRunner,
	), nil
}

// buildPublishRunner は、PublishRunner のインスタンスを返します。
func buildPublishRunner(ctx context.Context, appCtx *app.Container) (domain.PublishRunner, error) {
	mdOpts := []mdBuilder.Option{
		mdBuilder.WithEnableUnsafeHTML(false),
	}
	b, err := mdBuilder.New(mdOpts...)
	if err != nil {
		return nil, fmt.Errorf("markdown Builderの初期化に失敗: %w", err)
	}
	md2htmlRunner, err := b.BuildRunner()
	if err != nil {
		return nil, fmt.Errorf("MarkdownToHtmlRunnerの構築に失敗: %w", err)
	}

	return runner.NewPublisherRunner(
		appCtx.RemoteIO.Writer,
		md2htmlRunner,
	), nil
}
