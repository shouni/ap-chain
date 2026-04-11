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
	fetcher   Fetcher
	cleaner   Cleaner
	publisher Publisher
	notifier  domain.Notifier
}

// New は、Pipeline を生成します。
func New(f Fetcher, c Cleaner, p Publisher, n domain.Notifier) *Pipeline {
	return &Pipeline{
		fetcher:   f,
		cleaner:   c,
		publisher: p,
		notifier:  n,
	}
}

// Execute は一連の処理を実行します。
func (p *Pipeline) Execute(ctx context.Context, req domain.Request) (err error) {
	// 0. バリデーション（早期リターン）
	if req.InputURI == "" || req.OutputURI == "" {
		return fmt.Errorf("InputURI and OutputURI are required")
	}

	// 1. エラー発生時の遅延通知（deferによる一元管理）
	defer func() {
		if err != nil {
			p.notifyFailure(ctx, err)
		}
	}()

	// 2. Fetch
	results, err := p.fetcher.Run(ctx, req.InputURI)
	if err != nil {
		return fmt.Errorf("fetch process failed: %w", err)
	}

	// 3. Clean (LLM MapReduce)
	content, err := p.cleaner.Run(ctx, results)
	if err != nil {
		return fmt.Errorf("clean process failed: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("cleaned content is empty")
	}

	// 4. Publish
	publishResult, err := p.publisher.Run(ctx, req.OutputURI, content)
	if err != nil {
		return fmt.Errorf("publish process failed: %w", err)
	}

	// 5. Success Notification
	p.notifySuccess(ctx, publishResult, len(results))

	return nil
}

// notifySuccess は、指定された公開結果とソース数を含む成功通知を通知機能を通じて送信します。
func (p *Pipeline) notifySuccess(ctx context.Context, res *domain.PublishResult, count int) {
	p.sendNotify(ctx, func(nCtx context.Context) error {
		return p.notifier.NotifySuccess(nCtx, res.HTML.StorageURI, res.HTML.PublicURL, count)
	}, "success")
}

// notifyFailure は、指定されたエラーコンテキストとともに、通知機能を通じて障害通知を送信します。
func (p *Pipeline) notifyFailure(ctx context.Context, err error) {
	p.sendNotify(ctx, func(nCtx context.Context) error {
		return p.notifier.NotifyFailure(nCtx, err)
	}, "failure")
}

// sendNotify は、指定された通知機能とラベルを使用して、通知機能を介して通知を送信します。
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
