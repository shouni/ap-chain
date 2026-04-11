package builder

import (
	"context"
	"fmt"

	"ap-chain/internal/app"
	"ap-chain/internal/domain"
	"ap-chain/internal/pipeline"
)

// buildPipeline は、提供されたランナーを使用して新しいパイプラインを初期化して返します。
func buildPipeline(ctx context.Context, appCtx *app.Container) (domain.Pipeline, error) {
	fetchRunner, err := buildFetchRunner(ctx, appCtx)
	if err != nil {
		return nil, fmt.Errorf("生成ランナーの初期化に失敗しました: %w", err)
	}
	cleanRunner, err := buildCleanRunner(ctx, appCtx)
	if err != nil {
		return nil, fmt.Errorf("LLMランナーの初期化に失敗しました: %w", err)
	}
	publisherRunner, err := buildPublishRunner(ctx, appCtx)
	if err != nil {
		return nil, fmt.Errorf("パブリッシャーランナーの初期化に失敗しました: %w", err)
	}

	p := pipeline.New(fetchRunner, cleanRunner, publisherRunner, appCtx.Notifier)

	return p, nil
}
