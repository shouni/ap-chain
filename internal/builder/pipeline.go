package builder

import (
	"context"

	"ap-chain/internal/app"
	"ap-chain/internal/domain"
	"ap-chain/internal/pipeline"
)

// buildPipeline は、各コンポーネントを構築し、新しいパイプラインを初期化して返します。
func buildPipeline(ctx context.Context, appCtx *app.Container) (domain.Pipeline, error) {
	collector, err := buildCollector(appCtx)
	if err != nil {
		return nil, wrapInitErr("Collector", err)
	}

	composer, err := buildComposer(ctx, appCtx)
	if err != nil {
		return nil, wrapInitErr("Composer", err)
	}

	publisher, err := buildPublisher(appCtx)
	if err != nil {
		return nil, wrapInitErr("Publisher", err)
	}

	p := pipeline.New(collector, composer, publisher, appCtx.Notifier)

	return p, nil
}
