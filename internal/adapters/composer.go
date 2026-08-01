package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"ap-chain/internal/config"
	"ap-chain/internal/domain"
)

// PromptBuilder は、プロンプト文字列を生成する責務を定義します。
type PromptBuilder interface {
	GenerateMap(text string) (string, error)
	GenerateReduce(segments []domain.Segment) (string, error)
}

// ComposerAdapter は、LLMを使用してコンテンツを構成するAdapter層の実装です。
type ComposerAdapter struct {
	aiClient       gemini.MultimodalGenerator
	promptBuilder  PromptBuilder
	maxConcurrency int
	rateInterval   time.Duration
}

// NewComposerAdapter は、ComposerAdapter の新しいインスタンスを生成します。
func NewComposerAdapter(ai gemini.MultimodalGenerator, pb PromptBuilder, opts ...ComposerOption) (*ComposerAdapter, error) {
	if ai == nil || pb == nil {
		return nil, fmt.Errorf("aiClient and promptBuilder are required")
	}
	c := &ComposerAdapter{
		aiClient:       ai,
		promptBuilder:  pb,
		maxConcurrency: config.DefaultMaxConcurrency,
		rateInterval:   config.DefaultRateInterval,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// ComposerOption は ComposerAdapter の設定を変更する関数型です。
type ComposerOption func(*ComposerAdapter)

// WithMaxConcurrency は、最大並列数を設定します。
func WithMaxConcurrency(value int) ComposerOption {
	return func(g *ComposerAdapter) {
		if value > 0 {
			g.maxConcurrency = value
		}
	}
}

// WithRateInterval は、レートリミット間隔を設定します。
func WithRateInterval(d time.Duration) ComposerOption {
	return func(g *ComposerAdapter) {
		if d > 0 {
			g.rateInterval = d
		}
	}
}

// mapResponse は、Mapフェーズの構造化出力(mapOutputSchema)に対応します。
type mapResponse struct {
	CleanedText string `json:"cleaned_text"`
}

// RunMap は errgroup と rate.Limiter を使用して、安全かつ効率的に並列実行を行います。
// 各セグメントをAIでクリーンアップし、出典URL(Go側で既知)と組にして返します。
func (a *ComposerAdapter) RunMap(ctx context.Context, model string, allSegments []domain.Segment) ([]domain.Segment, error) {
	total := len(allSegments)
	cleaned := make([]domain.Segment, total)

	eg, ctx := errgroup.WithContext(ctx)
	// 同時実行数を制限
	eg.SetLimit(a.maxConcurrency)
	// APIレート制限を管理
	limiter := rate.NewLimiter(rate.Every(a.rateInterval), 1)

	slog.InfoContext(ctx, "セグメントの並列処理を開始します",
		slog.Int("total_segments", total),
		slog.Int("max_parallel", a.maxConcurrency))

	for i, seg := range allSegments {
		eg.Go(func() error {
			if err := limiter.Wait(ctx); err != nil {
				return fmt.Errorf("レート制限の待機中にエラー: %w", err)
			}

			prompt, err := a.promptBuilder.GenerateMap(seg.Text)
			if err != nil {
				return fmt.Errorf("セグメント %d 処理失敗: %w", i+1, err)
			}

			response, err := a.aiClient.GenerateWithAttachments(ctx, model, prompt, nil, gemini.GenerateOptions{
				ResponseMIMEType: "application/json",
				ResponseSchema:   mapOutputSchema(),
			})
			if err != nil {
				return fmt.Errorf("セグメント %d (URL: %s) 処理失敗: %w", i+1, seg.URL, err)
			}

			var out mapResponse
			if err := json.Unmarshal([]byte(response.Text), &out); err != nil {
				return fmt.Errorf("セグメント %d (URL: %s) のJSON解析に失敗: %w", i+1, seg.URL, err)
			}

			cleaned[i] = domain.Segment{Text: out.CleanedText, URL: seg.URL}

			slog.InfoContext(ctx, "セグメント処理成功",
				slog.Int("index", i+1),
				slog.String("url", seg.URL))

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return compactSegments(ctx, cleaned), nil
}

// compactSegments は、本文が空になったセグメントを取り除きます。
// モデルが空の cleaned_text を返した場合、そのまま Reduce へ渡すと
// 出典URLだけを持つ中身のない要素がプロンプトに載るため、ここで落とします。
func compactSegments(ctx context.Context, segments []domain.Segment) []domain.Segment {
	compacted := make([]domain.Segment, 0, len(segments))
	for _, seg := range segments {
		if strings.TrimSpace(seg.Text) == "" {
			slog.WarnContext(ctx, "本文が空のセグメントを除外しました", slog.String("url", seg.URL))
			continue
		}
		compacted = append(compacted, seg)
	}

	return compacted
}

// RunReduce は中間要約を統合し、最終的な構造化レポートのJSON文字列を生成します。
// (title/sections 形式。reduceOutputSchema 参照。JSON→Markdown/HTML等への変換は
// 呼び出し側の責務とし、ここではunmarshalしません。)
func (a *ComposerAdapter) RunReduce(ctx context.Context, model string, segments []domain.Segment) (string, error) {
	slog.InfoContext(ctx, "最終的な構造化（Reduceフェーズ）を開始します。", slog.String("model", model))

	prompt, err := a.promptBuilder.GenerateReduce(segments)
	if err != nil {
		return "", fmt.Errorf("最終 Reduce プロンプトの生成に失敗しました: %w", err)
	}

	response, err := a.aiClient.GenerateWithAttachments(ctx, model, prompt, nil, gemini.GenerateOptions{
		ResponseMIMEType: "application/json",
		ResponseSchema:   reduceOutputSchema(),
	})
	if err != nil {
		return "", fmt.Errorf("LLM最終構造化処理（Reduceフェーズ）に失敗しました: %w", err)
	}

	slog.InfoContext(ctx, "Reduce処理成功", slog.String("model", model))

	return response.Text, nil
}
