package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"ap-chain/internal/domain"
)

const (
	defaultMaxConcurrency = 1
	defaultLLMRateLimit   = 10 * time.Second
)

// PromptBuilder は、プロンプト文字列を生成する責務を定義します。
type PromptBuilder interface {
	GenerateMap(text, url string) (string, error)
	GenerateReduce(text string) (string, error)
}

// ComposerAdapter は、LLMを使用してコンテンツを構成するAdapter層の実装です。
type ComposerAdapter struct {
	aiClient       gemini.ContentGenerator
	promptBuilder  PromptBuilder
	maxConcurrency int
	rateInterval   time.Duration
}

// NewComposerAdapter は、ComposerAdapter の新しいインスタンスを生成します。
func NewComposerAdapter(ai gemini.ContentGenerator, pb PromptBuilder, opts ...ComposerOption) (*ComposerAdapter, error) {
	if ai == nil || pb == nil {
		return nil, fmt.Errorf("aiClient and promptBuilder are required")
	}
	c := &ComposerAdapter{
		aiClient:       ai,
		promptBuilder:  pb,
		maxConcurrency: defaultMaxConcurrency,
		rateInterval:   defaultLLMRateLimit,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

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

// RunMap は errgroup と rate.Limiter を使用して、安全かつ効率的に並列実行を行います。
func (a *ComposerAdapter) RunMap(ctx context.Context, model string, allSegments []domain.Segment) ([]string, error) {
	total := len(allSegments)
	summaries := make([]string, total)

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

			prompt, err := a.promptBuilder.GenerateMap(seg.Text, seg.URL)
			if err != nil {
				return fmt.Errorf("セグメント %d 処理失敗: %w", i+1, err)
			}

			response, err := a.aiClient.GenerateContent(ctx, model, prompt)
			if err != nil {
				return fmt.Errorf("セグメント %d (URL: %s) 処理失敗: %w", i+1, seg.URL, err)
			}

			summaries[i] = response.Text

			slog.InfoContext(ctx, "セグメント処理成功",
				slog.Int("index", i+1),
				slog.String("url", seg.URL))

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return summaries, nil
}

// RunReduce は中間要約を統合し、最終的な構造化レポートを生成します。
func (a *ComposerAdapter) RunReduce(ctx context.Context, model, combinedText string) (string, error) {
	slog.InfoContext(ctx, "最終的な構造化（Reduceフェーズ）を開始します。", slog.String("model", model))

	prompt, err := a.promptBuilder.GenerateReduce(combinedText)
	if err != nil {
		return "", fmt.Errorf("最終 Reduce プロンプトの生成に失敗しました: %w", err)
	}

	response, err := a.aiClient.GenerateContent(ctx, model, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM最終構造化処理（Reduceフェーズ）に失敗しました: %w", err)
	}

	slog.InfoContext(ctx, "Reduce処理成功", slog.String("model", model))

	return response.Text, nil
}
