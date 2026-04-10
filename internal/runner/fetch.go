package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/shouni/go-web-exact/v2/ports"

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
func (r *FetchRunner) Run(ctx context.Context, sourceURL string) ([]ports.URLResult, error) {
	if sourceURL == "" {
		return nil, fmt.Errorf("入力ソース(--input)が指定されていません")
	}

	// 1. ソースコンテンツの読み込み (バリデーション済み)
	content, err := r.readContent(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	// 2. 有効なURLのみを抽出
	urls := r.parseURLs(ctx, content)

	// 早期リターンにより無駄な処理を防止
	if len(urls) == 0 {
		return nil, fmt.Errorf("ソースファイルから有効なURLが一件も抽出されませんでした")
	}

	// 3. スクレイピング実行
	results := r.scrape.Run(ctx, urls)
	if len(results) == 0 {
		return nil, fmt.Errorf("スクレイピング結果が空です。対象URLへのアクセスを確認してください。")
	}

	// 4. コンテンツの存在確認
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

	// LimitReader による OOM 防止
	body, err := io.ReadAll(io.LimitReader(stream, config.MaxInputSize))
	if err != nil {
		return "", fmt.Errorf("コンテンツの読み取りに失敗しました (上限10MB): %w", err)
	}

	trimmedContent := strings.TrimSpace(string(body))
	if len(trimmedContent) < config.MinInputContentLength {
		return "", fmt.Errorf("入力されたコンテンツが短すぎます")
	}
	return trimmedContent, nil
}

// parseURLs は content を行単位で分割し、有効なURLのみを抽出します。
func (r *FetchRunner) parseURLs(ctx context.Context, content string) []string {
	var urls []string
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 空行やコメントをスキップ
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// URL形式のバリデーション
		if _, err := url.ParseRequestURI(line); err != nil {
			slog.WarnContext(ctx, "無効なURL形式をスキップしました", "line", line, "error", err)
			continue
		}

		urls = append(urls, line)
	}

	return urls
}
