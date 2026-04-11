package pipeline

import (
	"context"

	"ap-chain/internal/domain"
)

type (
	Collector interface {
		Run(ctx context.Context, sourceURI string) ([]domain.URLResult, error)
	}
	Composer interface {
		Run(ctx context.Context, results []domain.URLResult) (string, error)
	}
	Publisher interface {
		Run(ctx context.Context, outputURI, content string) (*domain.PublishResult, error)
	}
)
