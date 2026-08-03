package builder

import (
	"context"

	"github.com/shouni/ap-chain/internal/app"
	"github.com/shouni/ap-chain/internal/domain"
	"github.com/shouni/ap-chain/internal/pipeline"
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

	p := pipeline.New(collector, composer, buildPublisher(appCtx), appCtx.Notifier)

	return p, nil
}
