package builder

import (
	"context"
	"fmt"

	"ap-chain/internal/app"
	"ap-chain/internal/domain"
	"ap-chain/internal/pipeline"
)

// buildPipeline は、各コンポーネントを構築し、新しいパイプラインを初期化して返します。
func buildPipeline(ctx context.Context, appCtx *app.Container) (domain.Pipeline, error) {
	collector, err := buildCollector(ctx, appCtx)
	if err != nil {
		return nil, fmt.Errorf("Collectorの初期化に失敗しました: %w", err)
	}

	composer, err := buildComposer(ctx, appCtx)
	if err != nil {
		return nil, fmt.Errorf("Composerの初期化に失敗しました: %w", err)
	}

	publisher, err := buildPublisher(ctx, appCtx)
	if err != nil {
		return nil, fmt.Errorf("Publisherの初期化に失敗しました: %w", err)
	}

	p := pipeline.New(collector, composer, publisher, appCtx.Notifier)

	return p, nil
}
