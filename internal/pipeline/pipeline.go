package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ap-chain/internal/domain"
)

// Pipeline は各コンポーネントを統合し、実行を管理するオーケストレーターです。
type Pipeline struct {
	collector Collector
	composer  Composer
	publisher Publisher
	notifier  domain.Notifier
}

// New は、依存関係を注入して Pipeline を生成します。
func New(col Collector, com Composer, pub Publisher, n domain.Notifier) *Pipeline {
	return &Pipeline{
		collector: col,
		composer:  com,
		publisher: pub,
		notifier:  n,
	}
}

// Execute は一連の処理（収集・構成・公開）を順次実行します。
func (p *Pipeline) Execute(ctx context.Context, req domain.Request) (err error) {
	// 0. バリデーション
	if req.InputURI == "" || req.OutputURI == "" {
		return errors.New("InputURI and OutputURI are required")
	}

	// 1. エラー発生時の遅延通知
	defer func() {
		if err != nil {
			p.notifyFailure(ctx, err)
		}
	}()

	// 2. Collect (Webコンテンツの並列収集)
	results, err := p.collector.Run(ctx, req.InputURI)
	if err != nil {
		return fmt.Errorf("collection process failed: %w", err)
	}

	// 3. Compose (LLM MapReduce によるレポート執筆)
	content, err := p.composer.Run(ctx, results)
	if err != nil {
		return fmt.Errorf("composition process failed: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("composed content is empty")
	}

	// 4. Publish (永続化と公開)
	publishResult, err := p.publisher.Run(ctx, req.OutputURI, content)
	if err != nil {
		return fmt.Errorf("publish process failed: %w", err)
	}

	// 5. Success Notification
	p.notifySuccess(ctx, publishResult, len(results))

	return nil
}

// notifySuccess は成功通知を送信します。
func (p *Pipeline) notifySuccess(ctx context.Context, res *domain.PublishResult, count int) {
	p.sendNotify(ctx, func(nCtx context.Context) error {
		return p.notifier.NotifySuccess(nCtx, res.HTML.StorageURI, res.HTML.PublicURL, count)
	}, "success")
}

// notifyFailure は失敗通知を送信します。
func (p *Pipeline) notifyFailure(ctx context.Context, err error) {
	p.sendNotify(ctx, func(nCtx context.Context) error {
		return p.notifier.NotifyFailure(nCtx, err)
	}, "failure")
}

// sendNotify は共通の通知送信ロジックです。
func (p *Pipeline) sendNotify(ctx context.Context, notifyFn func(context.Context) error, label string) {
	if p.notifier == nil {
		return
	}

	// 親コンテキストがキャンセルされていても、通知のために独立した時間を確保します。
	nCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if err := notifyFn(nCtx); err != nil {
		slog.ErrorContext(nCtx, "failed to send notification",
			slog.String("type", label),
			slog.Any("error", err))
	}
}
