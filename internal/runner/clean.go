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

type LLMExecutor interface {
	ExecuteMap(ctx context.Context, model string, allSegments []domain.Segment) ([]string, error)
	ExecuteReduce(ctx context.Context, model, combinedText string) (string, error)
}

type CleanRunner struct {
	cfg      *config.Config
	executor LLMExecutor
}

// NewCleanRunner は CleanRunner の新しいインスタンスを作成します。
func NewCleanRunner(cfg *config.Config, executor LLMExecutor) (*CleanRunner, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}
	if executor == nil {
		return nil, errors.New("executor cannot be nil")
	}

	return &CleanRunner{
		cfg:      cfg,
		executor: executor,
	}, nil
}

// Run は、取得したURL結果のリストに対してクリーンアップと構造化処理を実行者に委譲します。
func (r *CleanRunner) Run(ctx context.Context, urls []domain.URLResult) (string, error) {
	if len(urls) == 0 {
		return "", errors.New("urls is empty")
	}
	return r.cleanAndStructureText(ctx, urls)
}

// CleanAndStructureText は、MapReduce処理を実行し、最終的なクリーンアップと構造化を行います。
func (r *CleanRunner) cleanAndStructureText(ctx context.Context, results []domain.URLResult) (string, error) {
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
	intermediateSummaries, err := r.executor.ExecuteMap(ctx, r.cfg.MapModel, allSegments)
	if err != nil {
		return "", fmt.Errorf("セグメント処理（Mapフェーズ）に失敗しました: %w", err)
	}

	// 3. Reduceフェーズの準備：中間要約の結合
	const summarySeparator = "\n\n--- INTERMEDIATE SUMMARY END ---\n\n"
	finalCombinedText := strings.Join(intermediateSummaries, summarySeparator)

	// 4. Reduceフェーズ：最終的な統合と構造化
	slog.InfoContext(ctx, "Final structuring started (Reduce phase)")

	finalResponseText, err := r.executor.ExecuteReduce(ctx, r.cfg.ReduceModel, finalCombinedText)
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
		// 先頭から maxChars ルーン分のバイトインデックスを特定
		byteIdx := 0
		runeCount := 0
		for i := 0; i < len(text) && runeCount < maxChars; {
			_, size := utf8.DecodeRuneInString(text[i:])
			i += size
			byteIdx = i
			runeCount++
		}

		// 残りのテキストが maxChars ルーン以下の場合
		if runeCount <= maxChars && byteIdx == len(text) {
			segments = append(segments, text)
			break
		}

		candidate := text[:byteIdx]
		lastByteIdx := strings.LastIndex(candidate, defaultSeparator)

		splitByteIdx := byteIdx
		if lastByteIdx != -1 {
			runeCountBeforeSep := utf8.RuneCountInString(candidate[:lastByteIdx])
			if runeCountBeforeSep > maxChars/2 {
				splitByteIdx = lastByteIdx + len(defaultSeparator)
			}
		}

		if splitByteIdx == byteIdx {
			slog.WarnContext(ctx, "No suitable separator found in segment. Forced splitting at max chars.",
				slog.Int("forced_chars", maxChars))
		}

		segments = append(segments, text[:splitByteIdx])
		text = text[splitByteIdx:]
	}

	return segments
}
