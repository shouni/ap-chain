package runner

import (
	"context"
	"errors"

	"ap-chain/internal/domain"
)

type CleanRunner struct {
	cleaner domain.Cleaner
}

// NewCleanRunner は CleanRunner の新しいインスタンスを作成します。
func NewCleanRunner(cleaner domain.Cleaner) *CleanRunner {
	return &CleanRunner{
		cleaner: cleaner,
	}
}

// Run は、取得したURL結果のリストに対してクリーンアップと構造化処理を実行者に委譲します。
func (f *CleanRunner) Run(ctx context.Context, urls []domain.URLResult) (string, error) {
	if len(urls) == 0 {
		return "", errors.New("urls is empty")
	}

	result, err := f.cleaner.CleanAndStructureText(ctx, urls)
	if err != nil {
		return "", err
	}

	return result, nil
}
