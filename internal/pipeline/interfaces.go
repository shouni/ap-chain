package pipeline

import (
	"ap-chain/internal/domain"
	"context"
)

type (
	Fetcher interface {
		Run(ctx context.Context, sourceURI string) ([]domain.URLResult, error)
	}
	Cleaner interface {
		Run(ctx context.Context, results []domain.URLResult) (string, error)
	}
	Publisher interface {
		Run(ctx context.Context, outputURI, content string) (*domain.PublishResult, error)
	}
)
