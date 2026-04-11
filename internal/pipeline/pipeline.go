package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ap-chain/internal/domain"
)

// Pipeline はパイプラインの実行に必要な外部依存関係を保持するサービス構造体です。
type Pipeline struct {
	fetcher   domain.FetchRunner
	cleaner   domain.CleanRunner
	publisher domain.PublishRunner
	notifier  domain.Notifier
}

// NewPipeline は、Pipeline を生成します。
func NewPipeline(
	fetcher domain.FetchRunner,
	cleaner domain.CleanRunner,
	publisher domain.PublishRunner,
	notifier domain.Notifier,
) *Pipeline {
	return &Pipeline{
		fetcher:   fetcher,
		cleaner:   cleaner,
		publisher: publisher,
		notifier:  notifier,
	}
}

// Execute は、パイプラインの各ステップ（取得、クリーンアップ、公開）を順次実行し、結果を通知します。
func (p *Pipeline) Execute(ctx context.Context, req domain.Request) (err error) {
	if req.InputURI == "" || req.OutputURI == "" {
		return fmt.Errorf("InputURI および OutputURI は必須です")
	}

	var urlResults []domain.URLResult

	// 1. エラー発生時の遅延通知
	defer func() {
		if err != nil {
			p.sendNotify(ctx, func(nCtx context.Context) error {
				return p.notifier.NotifyFailure(nCtx, err)
			}, "failure")
		}
	}()

	// 2. Fetch
	if urlResults, err = p.fetch(ctx, req.InputURI); err != nil {
		return err
	}

	// 3. Clean (MapReduce)
	var content string
	if content, err = p.clean(ctx, urlResults); err != nil {
		return err
	}

	if strings.TrimSpace(content) == "" {
		err = fmt.Errorf("AIモデルが空のコンテンツを返しました")
		return err
	}

	// 4. Publish
	var result *domain.PublishResult
	if result, err = p.publisher.Run(ctx, req.OutputURI, content); err != nil {
		return err
	}

	// 5. Success Notification (ここも確実に行う)
	p.sendNotify(ctx, func(nCtx context.Context) error {
		return p.notifier.NotifySuccess(nCtx, result.HTML.StorageURI, result.HTML.PublicURL, len(urlResults))
	}, "success")

	return nil
}

// sendNotify は、親コンテキストの状態に関わらず通知を試行する共通ヘルパーです。
func (p *Pipeline) sendNotify(ctx context.Context, notifyFn func(context.Context) error, label string) {
	if p.notifier == nil {
		return
	}

	// 親がキャンセルされていても10秒間は通知のために粘る
	nCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if err := notifyFn(nCtx); err != nil {
		slog.Error("failed to send notification", "type", label, "error", err)
	}
}

// fetch は、コンテンツ取得を実行します。
func (p *Pipeline) fetch(ctx context.Context, sourceURI string) ([]domain.URLResult, error) {
	results, err := p.fetcher.Run(ctx, sourceURI)
	if err != nil {
		return nil, fmt.Errorf("スクリプトテキスト作成に失敗しました: %w", err)
	}

	return results, nil
}

// clean は、LLMマルチステップを実行します。
func (p *Pipeline) clean(ctx context.Context, result []domain.URLResult) (string, error) {
	content, err := p.cleaner.Run(ctx, result)
	if err != nil {
		return "", fmt.Errorf("結果テキスト作成に失敗しました: %w", err)
	}

	return content, nil
}
