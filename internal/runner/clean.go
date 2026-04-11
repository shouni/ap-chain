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
	ExecuteMap(ctx context.Context, model string, allSegments []domain.URLResult) ([]string, error)
	ExecuteReduce(ctx context.Context, model, combinedText string) (string, error)
}

type CleanRunner struct {
	cfg      *config.Config
	executor LLMExecutor
}

// NewCleanRunner は CleanRunner の新しいインスタンスを作成します。
func NewCleanRunner(cfg *config.Config, executor LLMExecutor) (*CleanRunner, error) {
	if executor == nil {
		return nil, errors.New("executor cannot be nil")
	}

	return &CleanRunner{
		cfg,
		executor,
	}, nil
}

// Run は、取得したURL結果のリストに対してクリーンアップと構造化処理を実行者に委譲します。
func (r *CleanRunner) Run(ctx context.Context, urls []domain.URLResult) (string, error) {
	if len(urls) == 0 {
		return "", errors.New("urls is empty")
	}

	result, err := r.cleanAndStructureText(ctx, urls)
	if err != nil {
		return "", err
	}

	return result, nil
}

// CleanAndStructureText は、MapReduce処理を実行し、最終的なクリーンアップと構造化を行います。
func (r *CleanRunner) cleanAndStructureText(ctx context.Context, results []domain.URLResult) (string, error) {
	// 1. MapフェーズのためのURL単位のテキスト分割
	allSegments := make([]domain.URLResult, 0, len(results)*2)
	for _, res := range results {
		segments := segmentText(ctx, res.Content, maxSegmentChars)
		for _, segText := range segments {
			allSegments = append(allSegments, domain.URLResult{Content: segText, URL: res.URL})
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
	current := []rune(text)

	for len(current) > 0 {
		// すでに残りルーン数が上限以下ならそのまま終了
		if len(current) <= maxChars {
			segments = append(segments, string(current))
			break
		}

		// デフォルトは最大文字数で分割
		splitIndex := maxChars
		// candidate は最初の maxChars 文字分を string に戻したもの
		candidate := string(current[:maxChars])

		// 優先的な区切り文字（改行）をバイト単位で検索
		lastByteIdx := strings.LastIndex(candidate, defaultSeparator)

		if lastByteIdx != -1 {
			// 重要: バイトインデックスをルーン数に変換する
			// これにより、マルチバイト文字が混在していても正しい切り出し位置が算出される
			runeCountBeforeSep := utf8.RuneCountInString(candidate[:lastByteIdx])

			// セグメントが極端に短くなりすぎないよう、半分より後ろで見つかった場合のみ採用
			if runeCountBeforeSep > maxChars/2 {
				splitIndex = runeCountBeforeSep + utf8.RuneCountInString(defaultSeparator)
			}
		}

		// 安全な区切りが見つからなかった（または前の方すぎた）場合
		if splitIndex == maxChars {
			slog.WarnContext(ctx, "No suitable separator found in segment. Forced splitting at max chars.",
				slog.Int("forced_chars", maxChars))
		}

		// 決定したルーン位置でスライス
		segments = append(segments, string(current[:splitIndex]))
		current = current[splitIndex:]
	}

	return segments
}
