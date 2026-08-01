// Package builder は、外部サービスとの接続を確立し Container を組み立てます。
package builder

import (
	"context"
	"io"
	"log/slog"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-remote-io/remoteio/gcs"

	"ap-chain/internal/adapters"
	"ap-chain/internal/app"
	"ap-chain/internal/config"
)

// BuildContainer は外部サービスとの接続を確立し、依存関係を組み立てた app.Container を返します。
func BuildContainer(ctx context.Context, cfg *config.Config) (container *app.Container, err error) {
	var resources []io.Closer
	defer func() {
		if err != nil {
			for _, r := range resources {
				if closeErr := r.Close(); closeErr != nil {
					slog.Warn("failed to close resource during cleanup", "error", closeErr)
				}
			}
		}
	}()

	// 1. I/O Infrastructure (GCS)
	storage, err := gcs.New(ctx)
	if err != nil {
		return nil, wrapInitErr("GCSファクトリ", err)
	}
	resources = append(resources, storage)
	rio, err := buildRemoteIO(storage)
	if err != nil {
		return nil, wrapInitErr("IOコンポーネント", err)
	}

	httpClient := httpkit.New(cfg.HTTPTimeout)

	// 2. Slack Adapter
	slack, err := adapters.NewSlackAdapter(httpClient, cfg.SlackWebhookURL)
	if err != nil {
		return nil, wrapInitErr("Slackアダプタ", err)
	}

	appCtx := &app.Container{
		Config:     cfg,
		RemoteIO:   rio,
		HTTPClient: httpClient,
		Notifier:   slack,
	}

	// 3. Pipeline (Core Logic)
	p, err := buildPipeline(ctx, appCtx)
	if err != nil {
		return nil, wrapInitErr("パイプライン", err)
	}
	appCtx.Pipeline = p

	return appCtx, nil
}
