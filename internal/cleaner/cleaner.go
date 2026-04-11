package cleaner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ap-chain/internal/domain"
)

// Segment は、LLMに渡すテキストと、それが由来する元のURLを保持します。
type Segment struct {
	Text string
	URL  string
}

type LLMExecutor interface {
	ExecuteMap(ctx context.Context, segments []Segment) ([]string, error)
	ExecuteReduce(ctx context.Context, combinedText string) (string, error)
}

// Cleaner はコンテンツのクリーンアップと要約を担当します。
type Cleaner struct {
	builder  domain.PromptBuilder
	executor LLMExecutor
}

// NewCleaner は新しい Cleaner インスタンスを作成し、PromptBuilderを一度だけ初期化します。
func NewCleaner(builder domain.PromptBuilder, executor LLMExecutor) (*Cleaner, error) {
	if executor == nil {
		return nil, fmt.Errorf("LLM Executor は nil にできません")
	}

	return &Cleaner{
		builder:  builder,
		executor: executor,
	}, nil
}

// CleanAndStructureText は、MapReduce処理を実行し、最終的なクリーンアップと構造化を行います。
// LLMExecutor に依存することで、APIキーの処理や並列実行の詳細から解放されています。
func (c *Cleaner) CleanAndStructureText(ctx context.Context, results []domain.URLResult) (string, error) {
	// 1. MapフェーズのためのURL単位のテキスト分割
	var allSegments []Segment
	for _, res := range results {
		// URLResultのContentを個別にセグメント分割
		segments := segmentText(res.Content, MaxSegmentChars)
		for _, segText := range segments {
			allSegments = append(allSegments, Segment{Text: segText, URL: res.URL})
		}
	}

	slog.Info("コンテンツをURL単位でセグメントに分割しました。中間要約を開始します。",
		slog.Int("total_segments", len(allSegments)))

	// 2. Mapフェーズの実行（Executorに委譲）
	intermediateSummaries, err := c.executor.ExecuteMap(ctx, allSegments)
	if err != nil {
		return "", fmt.Errorf("セグメント処理（Mapフェーズ）に失敗しました: %w", err)
	}

	// 3. Reduceフェーズの準備：中間要約の結合
	finalCombinedText := strings.Join(intermediateSummaries, "\n\n--- INTERMEDIATE SUMMARY END ---\n\n")

	// 4. Reduceフェーズ：最終的な統合と構造化のためのLLM呼び出し（Executorに委譲）
	slog.Info("中間要約の結合が完了しました。最終的な構造化（Reduceフェーズ）を開始します。")

	finalResponseText, err := c.executor.ExecuteReduce(ctx, finalCombinedText)
	if err != nil {
		return "", fmt.Errorf("LLM最終構造化処理（Reduceフェーズ）に失敗しました: %w", err)
	}

	return strings.TrimSpace(finalResponseText), nil
}

// segmentText は、結合されたテキストを、安全な最大文字数を超えないように分割します。
// これは純粋な関数であり、外部の状態に依存しません。
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
		separatorFound := false
		separatorLen := 0

		// 1. 一般的な改行(\n\n)を探す
		if lastSepIdx := strings.LastIndex(segmentCandidate, DefaultSeparator); lastSepIdx != -1 && lastSepIdx > maxChars/2 {
			splitIndex = lastSepIdx
			separatorLen = len(DefaultSeparator)
			separatorFound = true
		}

		// 区切り文字の種類に応じて、加算する長さを適切に選択
		if separatorFound {
			// 区切り文字の直後までを分割位置とする
			splitIndex += separatorLen
		} else {
			// 安全な区切りが見つからない場合は、そのまま最大文字数で切り、警告を出す
			slog.Warn("⚠️ 分割点で適切な区切りが見つかりませんでした。強制的に分割します。",
				slog.Int("forced_chars", maxChars))
			splitIndex = maxChars
		}

		segments = append(segments, string(current[:splitIndex]))
		current = current[splitIndex:]
	}

	return segments
}
