// Package pipeline は、収集・構成・公開の各フェーズを統合するオーケストレーターです。
package pipeline

import (
	"context"

	"ap-chain/internal/domain"
)

type (
	// Collector は、入力ソースからWebコンテンツを収集する責務です。
	Collector interface {
		Run(ctx context.Context, sourceURI string) ([]domain.URLResult, error)
	}
	// Composer は、収集結果を構造化レポートのJSONへまとめる責務です。
	Composer interface {
		Run(ctx context.Context, results []domain.URLResult) (string, error)
	}
	// Publisher は、成果物を出力先へ書き出す責務です。
	Publisher interface {
		Run(ctx context.Context, outputURI, content string) (*domain.PublishResult, error)
	}
)
