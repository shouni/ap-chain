package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
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

// Run は、URLのコンテントリストに対してスクレイピング処理を実行者に委譲します。
func (r *FetchRunner) Run(ctx context.Context, sourceURL string) ([]ports.URLResult, error) {
	if sourceURL == "" {
		return nil, fmt.Errorf("入力ソース(--url)が指定されていません")
	}
	content, err := r.readContent(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	urls := r.parseURLs(content)
	if len(urls) == 0 {
		return nil, fmt.Errorf("ソースファイルからURLを抽出できませんでした")
	}

	results := r.scrape.Run(ctx, urls)
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

	body, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("コンテンツの読み込みに失敗しました: %w", err)
	}

	trimmedContent := strings.TrimSpace(string(body))
	if len(trimmedContent) < config.MinInputContentLength {
		return "", fmt.Errorf("入力されたコンテンツが短すぎます")
	}
	return trimmedContent, nil
}

// parseURLs は content を行単位で分割し、空行やコメントを除外してURLリストを返します。
func (r *FetchRunner) parseURLs(content string) []string {
	var urls []string
	// string 用の scanner を使う（strings.NewReader）
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	return urls
}
