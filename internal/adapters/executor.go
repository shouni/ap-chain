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

const defaultLLMRateLimit = 20 * time.Second

type LLMConcurrentExecutor struct {
	aiClient      gemini.ContentGenerator
	promptBuilder domain.PromptBuilder
	concurrency   int
}

func NewLLMConcurrentExecutor(ai gemini.ContentGenerator, pb domain.PromptBuilder, concurrency int) (*LLMConcurrentExecutor, error) {
	return &LLMConcurrentExecutor{
		aiClient:      ai,
		promptBuilder: pb,
		concurrency:   concurrency,
	}, nil
}

// ExecuteMap は errgroup と rate.Limiter を使用して Mapフェーズを実行します。
func (e *LLMConcurrentExecutor) ExecuteMap(ctx context.Context, model string, allSegments []domain.Segment) ([]string, error) {
	total := len(allSegments)
	summaries := make([]string, total)

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(e.concurrency)

	// RPM制限のためのリミッター
	limiter := rate.NewLimiter(rate.Every(defaultLLMRateLimit), e.concurrency)

	slog.InfoContext(ctx, "セグメントの並列処理を開始します",
		slog.Int("total_segments", total),
		slog.Int("max_parallel", e.concurrency))

	for i, seg := range allSegments {
		// 1. レートリミット待機（メインループで待機し、Goroutineのスパイクを防ぐ）
		if err := limiter.Wait(ctx); err != nil {
			return nil, err
		}

		// 2. errgroup.Go を使用して Goroutine を起動
		eg.Go(func() error {
			prompt, err := e.promptBuilder.GenerateMap(seg.Text, seg.URL)
			if err != nil {
				return fmt.Errorf("セグメント %d 処理失敗: %w", i+1, err)
			}

			response, err := e.aiClient.GenerateContent(ctx, model, prompt)
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

	// すべての完了を待ち、最初のエラーがあればそれを返す
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return summaries, nil
}

// ExecuteReduce は構造に変化がないため、ロジックのみ維持します。
func (e *LLMConcurrentExecutor) ExecuteReduce(ctx context.Context, model, combinedText string) (string, error) {
	slog.InfoContext(ctx, "最終的な構造化（Reduceフェーズ）を開始します。", slog.String("model", model))

	prompt, err := e.promptBuilder.GenerateReduce(combinedText)
	if err != nil {
		return "", fmt.Errorf("最終 Reduce プロンプトの生成に失敗しました: %w", err)
	}

	response, err := e.aiClient.GenerateContent(ctx, model, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM最終構造化処理（Reduceフェーズ）に失敗しました: %w", err)
	}

	slog.InfoContext(ctx, "Reduce処理成功", slog.String("model", model))

	return response.Text, nil
}
