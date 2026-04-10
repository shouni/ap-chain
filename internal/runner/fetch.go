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

type FetchRunner struct {
	reader domain.ContentReader
	scrape ports.ScrapeRunner
}

// NewFetchRunner は FetchRunner の新しいインスタンスを作成します。
func NewFetchRunner(reader domain.ContentReader, scrape ports.ScrapeRunner) *FetchRunner {
	return &FetchRunner{
		reader: reader,
		scrape: scrape,
	}
}

// Run は、ソースURLからURLリストを取得し、スクレイピングを実行します。
func (r *FetchRunner) Run(ctx context.Context, sourceURI string) ([]ports.URLResult, error) {
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
	results := r.scrape.Run(ctx, urls)
	if len(results) == 0 {
		return nil, fmt.Errorf("スクレイピング結果が空です")
	}

	var hasValidContent bool
	for _, res := range results {
		if res.Error == nil && res.Content != "" {
			hasValidContent = true
			break
		}
	}

	if !hasValidContent {
		return nil, fmt.Errorf("処理可能なWebコンテンツを一件も取得できませんでした")
	}

	return results, nil
}

// readContent は、指定されたソースURLからコンテンツを取得します。
func (r *FetchRunner) readContent(ctx context.Context, sourceURL string) (string, error) {
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
func (r *FetchRunner) parseURLs(ctx context.Context, content string) ([]string, error) {
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

	// Scannerの内部エラーをチェックし、呼び出し元に伝える
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("スキャン中にエラーが発生しました: %w", err)
	}

	return urls, nil
}
