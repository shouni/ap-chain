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
}

// NewPipeline は、Pipeline を生成します。
func NewPipeline(
	cfg *config.Config,
	fetcher domain.FetchRunner,
	cleaner domain.CleanRunner,
	publisher domain.PublishRunner,
) *Pipeline {
	return &Pipeline{
		cfg:       cfg,
		fetcher:   fetcher,
		cleaner:   cleaner,
		publisher: publisher,
	}
}

// Execute は、すべての依存関係を構築し実行します。
func (p *Pipeline) Execute(ctx context.Context) error {
	URLResult, err := p.fetch(ctx)
	if err != nil {
		return err
	}
	content, err := p.clean(ctx, URLResult)
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("AIモデルが空のスクリプトを返しました。プロンプトや入力コンテンツに問題がないか確認してください")
	}
	err = p.publish(ctx, content)
	if err != nil {
		return err
	}

	return nil
}

// fetch は、コンテンツ取得を実行します。
func (p *Pipeline) fetch(ctx context.Context) ([]ports.URLResult, error) {
	// TODO::テスト用
	var urls []string
	urls = append(urls, "https://github.com/shouni/go-http-kit")
	urls = append(urls, "https://github.com/shouni/go-remote-io")

	results, err := p.fetcher.Run(ctx, urls)
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

// publish は、パブリッシュを実行します。
func (p *Pipeline) publish(
	ctx context.Context,
	content string,
) error {
	err := p.publisher.Run(ctx, p.cfg.OutputSource, content)
	if err != nil {
		return fmt.Errorf("公開処理の実行に失敗しました: %w", err)
	}

	return nil
}
