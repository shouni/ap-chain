package cleaner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ap-chain/internal/domain"
)

const (
	// defaultSeparator は、一般的な段落区切りに使用される標準的な区切り文字です。
	defaultSeparator = "\n\n"
	// maxSegmentChars は、MapフェーズでLLMに一度に渡す安全な最大文字数。
	maxSegmentChars = 20000
)

// Segment は、LLMに渡すテキストと、それが由来する元のURLを保持します。
type Segment struct {
	Text string
	URL  string
}

// LLMModels マップ操作およびリデュース操作に使用されるモデルを表します。
type LLMModels struct {
	Map    string
	Reduce string
}

type LLMExecutor interface {
	ExecuteMap(ctx context.Context, model string, allSegments []Segment) ([]string, error)
	ExecuteReduce(ctx context.Context, model, combinedText string) (string, error)
}

// Cleaner はコンテンツのクリーンアップと要約を担当します。
type Cleaner struct {
	executor LLMExecutor
	models   LLMModels
}

// NewCleaner は新しい Cleaner インスタンスを作成します。
func NewCleaner(executor LLMExecutor, models LLMModels) (*Cleaner, error) {
	if executor == nil {
		return nil, fmt.Errorf("LLM Executor は nil にできません")
	}

	return &Cleaner{
		executor: executor,
		models:   models,
	}, nil
}

// CleanAndStructureText は、MapReduce処理を実行し、最終的なクリーンアップと構造化を行います。
func (c *Cleaner) CleanAndStructureText(ctx context.Context, results []domain.URLResult) (string, error) {
	// 1. MapフェーズのためのURL単位のテキスト分割
	// results の数からある程度のセグメント数を予測し、アロケーションを最適化
	allSegments := make([]Segment, 0, len(results)*2)
	for _, res := range results {
		segments := segmentText(res.Content, maxSegmentChars)
		for _, segText := range segments {
			allSegments = append(allSegments, Segment{Text: segText, URL: res.URL})
		}
	}

	slog.InfoContext(ctx, "Starting MapReduce process",
		slog.Int("total_urls", len(results)),
		slog.Int("total_segments", len(allSegments)))

	// 2. Mapフェーズの実行（Executorに委譲）
	intermediateSummaries, err := c.executor.ExecuteMap(ctx, c.models.Map, allSegments)
	if err != nil {
		return "", fmt.Errorf("セグメント処理（Mapフェーズ）に失敗しました: %w", err)
	}

	// 3. Reduceフェーズの準備：中間要約の結合
	const summarySeparator = "\n\n--- INTERMEDIATE SUMMARY END ---\n\n"
	finalCombinedText := strings.Join(intermediateSummaries, summarySeparator)

	// 4. Reduceフェーズ：最終的な統合と構造化
	slog.InfoContext(ctx, "Final structuring started (Reduce phase)")

	finalResponseText, err := c.executor.ExecuteReduce(ctx, c.models.Reduce, finalCombinedText)
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
func segmentText(text string, maxChars int) []string {
	var segments []string
	current := []rune(text)

	for len(current) > 0 {
		if len(current) <= maxChars {
			segments = append(segments, string(current))
			break
		}

		splitIndex := maxChars
		segmentCandidate := string(current[:maxChars])

		// 優先的な区切り文字（改行）を、最大文字数の半分より後ろから探す
		lastSepIdx := strings.LastIndex(segmentCandidate, defaultSeparator)

		if lastSepIdx != -1 && lastSepIdx > maxChars/2 {
			// 区切り文字が見つかった場合は、その直後までをセグメントとする
			splitIndex = lastSepIdx + len(defaultSeparator)
		} else {
			// 安全な区切りが見つからない場合は、そのまま最大文字数で切る
			slog.Warn("No suitable separator found in segment. Forced splitting at max chars.",
				slog.Int("forced_chars", maxChars))
			splitIndex = maxChars
		}

		segments = append(segments, string(current[:splitIndex]))
		current = current[splitIndex:]
	}

	return segments
}
