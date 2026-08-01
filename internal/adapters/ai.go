// Package adapters は、外部ライブラリを内部インターフェースの背後に隠すアダプタ群です。
package adapters

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"ap-chain/internal/config"
)

const (
	// defaultLocationID はデフォルトのロケーションIDです。
	defaultLocationID = "global"

	// defaultInitialDelay リトライのデフォルトの遅延期間を指定します。
	defaultInitialDelay = 30 * time.Second
)

// NewAIAdapter は aiClientを初期化します。
func NewAIAdapter(ctx context.Context, cfg *config.Config) (*gemini.Client, error) {
	clientConfig := gemini.Config{
		InitialDelay: defaultInitialDelay,
	}

	// GeminiAPIKeyが設定されている場合は優先して使用し、
	// 設定されていない場合はGCPのProjectIDを使用したVertex AI経由の認証を試みる。
	switch {
	case cfg.GeminiAPIKey != "":
		clientConfig.APIKey = cfg.GeminiAPIKey
	case cfg.ProjectID != "":
		clientConfig.ProjectID = cfg.ProjectID
		clientConfig.LocationID = defaultLocationID
	default:
		return nil, errors.New("GEMINI_API_KEY or GCP_PROJECT_ID is not set")
	}

	aiClient, err := gemini.NewClient(ctx, clientConfig)

	if err != nil {
		return nil, fmt.Errorf("AIクライアントの初期化に失敗しました: %w", err)
	}
	return aiClient, nil
}
