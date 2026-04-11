package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"ap-chain/internal/config"
	"ap-chain/internal/domain"
)

const (
	// defaultSeparator は、一般的な段落区切りに使用される標準的な区切り文字です。
	defaultSeparator = "\n\n"
	// maxSegmentChars は、MapフェーズでLLMに一度に渡す安全な最大文字数。
	maxSegmentChars = 20000
)

// Composer は、LLMを用いて複数の情報ソースを統合・構成するインターフェースです。
// MapReduceモデルを採用しており、個別のコンテンツ要約（Map）と
// それらを統合した最終レポートの構築（Reduce）を担います。
type Composer interface {
	// RunMap は、分割された各セグメントに対して並列に中間処理（要約等）を実行します。
	// 各セグメントから抽出された中間情報のスライスを返します。
	RunMap(ctx context.Context, model string, allSegments []domain.Segment) ([]string, error)

	// RunReduce は、RunMap で生成された中間情報を統合し、
	// 指定されたモデルを使用して最終的な構造化文書（レポート）を書き上げます。
	RunReduce(ctx context.Context, model, combinedText string) (string, error)
}

type ComposeRunner struct {
	cfg      *config.Config
	composer Composer
}

// NewComposeRunner は ComposeRunner の新しいインスタンスを作成します。
func NewComposeRunner(cfg *config.Config, composer Composer) (*ComposeRunner, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if composer == nil {
		return nil, errors.New("composer cannot be nil")
	}
	return &ComposeRunner{
		cfg:      cfg,
		composer: composer,
	}, nil
}

// Run は、取得したURL結果のリストに対してクリーンアップと構造化処理を実行者に委譲します。
func (r *ComposeRunner) Run(ctx context.Context, urls []domain.URLResult) (string, error) {
	if len(urls) == 0 {
		return "", errors.New("urls is empty")
	}
	return r.composeAndStructureText(ctx, urls)
}

// ComposeAndStructureText は、MapReduce処理を実行し、最終的な構成と構造化を行います。
func (r *ComposeRunner) composeAndStructureText(ctx context.Context, results []domain.URLResult) (string, error) {
	// 1. MapフェーズのためのURL単位のテキスト分割
	allSegments := make([]domain.Segment, 0, len(results)*2)
	for _, res := range results {
		segments := segmentText(ctx, res.Content, maxSegmentChars)
		for _, segText := range segments {
			allSegments = append(allSegments, domain.Segment{Text: segText, URL: res.URL})
		}
	}

	slog.InfoContext(ctx, "Starting MapReduce process",
		slog.Int("total_urls", len(results)),
		slog.Int("total_segments", len(allSegments)))

	// 2. Mapフェーズの実行（Executorに委譲）
	intermediateSummaries, err := r.composer.RunMap(ctx, r.cfg.MapModel, allSegments)
	if err != nil {
		return "", fmt.Errorf("セグメント処理（Mapフェーズ）に失敗しました: %w", err)
	}

	// 3. Reduceフェーズの準備：中間要約の結合
	const summarySeparator = "\n\n--- INTERMEDIATE SUMMARY END ---\n\n"
	finalCombinedText := strings.Join(intermediateSummaries, summarySeparator)

	// 4. Reduceフェーズ：最終的な統合と構造化
	slog.InfoContext(ctx, "Final structuring started (Reduce phase)")

	finalResponseText, err := r.composer.RunReduce(ctx, r.cfg.ReduceModel, finalCombinedText)
	if err != nil {
		return "", fmt.Errorf("LLM最終構造化処理（Reduceフェーズ）に失敗しました: %w", err)
	}

	result := strings.TrimSpace(finalResponseText)
	if result == "" {
		return "", fmt.Errorf("Reduceフェーズの結果が空です")
	}

	return result, nil
}

// segmentText は、テキストを最大文字数を超えないように分割します。
func segmentText(ctx context.Context, text string, maxChars int) []string {
	var segments []string

	for len(text) > 0 {
		byteIdx := 0
		runeCount := 0
		// maxChars ルーン分のバイトインデックスを特定しつつ、終端判定を行う
		for byteIdx < len(text) && runeCount < maxChars {
			_, size := utf8.DecodeRuneInString(text[byteIdx:])
			byteIdx += size
			runeCount++
		}

		// 残りが maxChars ルーン以下の場合はそのまま追加して終了
		if byteIdx == len(text) {
			segments = append(segments, text)
			break
		}

		candidate := text[:byteIdx]
		lastByteIdx := strings.LastIndex(candidate, defaultSeparator)

		splitByteIdx := byteIdx
		if lastByteIdx != -1 {
			if utf8.RuneCountInString(candidate[:lastByteIdx]) > maxChars/2 {
				splitByteIdx = lastByteIdx + len(defaultSeparator)
			}
		} else {
			slog.WarnContext(ctx, "No suitable separator found in segment. Forced splitting at max chars.",
				slog.Int("forced_chars", maxChars))
		}

		segments = append(segments, text[:splitByteIdx])
		text = text[splitByteIdx:]
	}

	return segments
}
