package runner

import (
	"context"
	"errors"

	"github.com/shouni/go-web-exact/v2/ports"

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

// Run は、URLのコンテントリストに対してスクレイピング処理を実行者に委譲します。
func (f *CleanRunner) Run(ctx context.Context, urls []ports.URLResult) (string, error) {
	if len(urls) == 0 {
		return "", errors.New("urls is empty")
	}

	result, err := f.cleaner.CleanAndStructureText(ctx, urls)
	if err != nil {
		return "", err
	}

	return result, nil
}
