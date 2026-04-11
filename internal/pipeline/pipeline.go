package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/shouni/go-web-exact/v2/ports"

	"ap-chain/internal/config"
	"ap-chain/internal/domain"
)

// Pipeline はパイプラインの実行に必要な外部依存関係を保持するサービス構造体です。
type Pipeline struct {
	cfg       *config.Config
	fetcher   domain.FetchRunner
	cleaner   domain.CleanRunner
	publisher domain.PublishRunner
	notifier  domain.Notifier
}

// NewPipeline は、Pipeline を生成します。
func NewPipeline(
	cfg *config.Config,
	fetcher domain.FetchRunner,
	cleaner domain.CleanRunner,
	publisher domain.PublishRunner,
	notifier domain.Notifier,
) *Pipeline {
	return &Pipeline{
		cfg:       cfg,
		fetcher:   fetcher,
		cleaner:   cleaner,
		publisher: publisher,
		notifier:  notifier,
	}
}

// Execute は、すべての依存関係を構築し実行します。
func (p *Pipeline) Execute(ctx context.Context) (err error) {
	var urlResults []ports.URLResult
	defer func() {
		if err != nil && p.notifier != nil {
			_ = p.notifier.NotifyFailure(ctx, err)
		}
	}()

	// 1. Fetch
	urlResults, err = p.fetch(ctx)
	if err != nil {
		return err // defer で通知される
	}

	// 2. Clean (MapReduce)
	content, err := p.clean(ctx, urlResults)
	if err != nil {
		return err
	}

	if strings.TrimSpace(content) == "" {
		err = fmt.Errorf("AIモデルが空のコンテンツを返しました")
		return err
	}

	// 3. Publish
	result, err := p.publisher.Run(ctx, p.cfg.OutputFile, content)
	if err != nil {
		return err
	}

	// 4. Success Notification
	if p.notifier != nil {
		_ = p.notifier.NotifySuccess(ctx, result.HTML.StorageURI, result.HTML.PublicURL, len(urlResults))
	}

	return nil
}

// fetch は、コンテンツ取得を実行します。
func (p *Pipeline) fetch(ctx context.Context) ([]ports.URLResult, error) {
	results, err := p.fetcher.Run(ctx, p.cfg.InputFile)
	if err != nil {
		return nil, fmt.Errorf("スクリプトテキスト作成に失敗しました: %w", err)
	}

	return results, nil
}

// clean は、LLMマルチステップを実行します。
func (p *Pipeline) clean(ctx context.Context, result []ports.URLResult) (string, error) {
	content, err := p.cleaner.Run(ctx, result)
	if err != nil {
		return "", fmt.Errorf("結果テキスト作成に失敗しました: %w", err)
	}

	return content, nil
}

func (p *Pipeline) handleError(ctx context.Context, err error) error {
	if p.notifier != nil {
		_ = p.notifier.NotifyFailure(ctx, err)
	}
	return err
}
