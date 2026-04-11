package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/shouni/go-web-exact/v2/ports"
	"github.com/shouni/netarmor/securenet"

	"ap-chain/internal/config"
	"ap-chain/internal/domain"
)

// ContentReader は、指定されたURIからコンテンツを取得するためのインターフェースです。
type ContentReader interface {
	Open(ctx context.Context, uri string) (io.ReadCloser, error)
}

type CollectRunner struct {
	reader  ContentReader
	scraper ports.ScrapeRunner
}

// NewCollectRunner は CollectRunner の新しいインスタンスを作成します。
func NewCollectRunner(reader ContentReader, scraper ports.ScrapeRunner) *CollectRunner {
	return &CollectRunner{
		reader:  reader,
		scraper: scraper,
	}
}

// Run は、ソースURLからURLリストを取得し、スクレイピングを実行して domain.URLResult のスライスを返します。
func (r *CollectRunner) Run(ctx context.Context, sourceURI string) ([]domain.URLResult, error) {
	if sourceURI == "" {
		return nil, fmt.Errorf("入力ソース(--input)が指定されていません")
	}

	// 1. ソースの読み込み
	content, err := r.readContent(ctx, sourceURI)
	if err != nil {
		return nil, err
	}

	// 2. 有効なURLの抽出
	urls, err := r.parseURLs(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("URLリストの解析に失敗しました: %w", err)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("ソースファイルから有効なURLが一件も抽出されませんでした")
	}

	// 3. スクレイピング実行
	rawResults := r.scraper.Run(ctx, urls)
	if len(rawResults) == 0 {
		return nil, fmt.Errorf("スクレイピング結果が空です")
	}

	// 4. domain.URLResult (自身の型) への詰め替えとフィルタリング
	// rawResults の要素数が既知であるため、キャパシティを事前に割り当ててアロケーションを最適化
	validResults := make([]domain.URLResult, 0, len(rawResults))
	for _, res := range rawResults {
		// エラーがなく、コンテンツが空でないものだけを採用
		if res.Error == nil && res.Content != "" {
			validResults = append(validResults, domain.URLResult{
				URL:     res.URL,
				Content: res.Content,
			})
		} else if res.Error != nil {
			slog.WarnContext(ctx, "スクレイピングに失敗したURLをスキップしました", "url", res.URL, "error", res.Error)
		} else {
			// エラーはないがコンテンツが空の場合もログに残し、トラブルシューティングを容易にする
			slog.WarnContext(ctx, "コンテンツが空のためURLをスキップしました", "url", res.URL)
		}
	}

	if len(validResults) == 0 {
		return nil, fmt.Errorf("処理可能なWebコンテンツを一件も取得できませんでした")
	}

	slog.InfoContext(ctx, "Fetching completed", "total", len(urls), "valid", len(validResults))
	return validResults, nil
}

// readContent は、指定されたソースURLからコンテンツを取得します。
func (r *CollectRunner) readContent(ctx context.Context, sourceURL string) (string, error) {
	stream, err := r.reader.Open(ctx, sourceURL)
	if err != nil {
		return "", fmt.Errorf("failed to read source: %w", err)
	}
	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			slog.WarnContext(ctx, "ストリームのクローズに失敗しました", "error", closeErr)
		}
	}()

	// Limit+1 バイト読み込み、切り捨てを検知する
	limit := int64(config.MaxInputSize)
	body, err := io.ReadAll(io.LimitReader(stream, limit+1))
	if err != nil {
		return "", fmt.Errorf("コンテンツの読み取りに失敗しました: %w", err)
	}

	if int64(len(body)) > limit {
		return "", fmt.Errorf("入力ファイルがサイズ上限 (%d MB) を超えています", limit/1024/1024)
	}

	trimmedContent := strings.TrimSpace(string(body))
	if len(trimmedContent) < config.MinInputContentLength {
		return "", fmt.Errorf("入力されたコンテンツが短すぎます")
	}
	return trimmedContent, nil
}

// parseURLs は content を行単位で分割し、有効なURLのみを抽出します。
func (r *CollectRunner) parseURLs(ctx context.Context, content string) ([]string, error) {
	var urls []string
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ok, err := securenet.IsSafeURL(line)
		if !ok || err != nil {
			slog.WarnContext(ctx, "無効なURL形式をスキップしました", "line", line, "error", err)
			continue
		}

		urls = append(urls, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("スキャン中にエラーが発生しました: %w", err)
	}

	return urls, nil
}
