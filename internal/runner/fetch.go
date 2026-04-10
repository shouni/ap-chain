package runner

import (
	"context"
	"fmt"

	"github.com/shouni/go-web-exact/v2/ports"
)

type FetchRunner struct {
	scrape ports.ScrapeRunner
}

// NewFetchRunner は FetchRunner の新しいインスタンスを作成します。
// ここで注入されるのは、リトライ機能を持つ runner.ReliableScraper です。
func NewFetchRunner(scrape ports.ScrapeRunner) *FetchRunner {
	return &FetchRunner{
		scrape: scrape,
	}
}

// Run は、URLリストに対してスクレイピング処理を実行者に委譲します。
func (f *FetchRunner) Run(ctx context.Context, urls []string) ([]ports.URLResult, error) {
	results := f.scrape.Run(ctx, urls)
	if len(results) == 0 {
		return nil, fmt.Errorf("取得された結果が空です。URLリストを確認してください。")
	}

	var hasValidContent bool
	for _, r := range results {
		if r.Error == nil && r.Content != "" {
			hasValidContent = true
			break
		}
	}

	if !hasValidContent {
		return nil, fmt.Errorf("処理可能なWebコンテンツを一件も取得できませんでした。")
	}

	return results, nil
}
