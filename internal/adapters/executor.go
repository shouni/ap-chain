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

// PromptBuilder は、プロンプト文字列を生成する責務を定義します。
type PromptBuilder interface {
	GenerateMap(text, url string) (string, error)
	GenerateReduce(text string) (string, error)
}

type Executor struct {
	aiClient      gemini.ContentGenerator
	promptBuilder PromptBuilder
	concurrency   int
}

func NewExecutor(ai gemini.ContentGenerator, pb PromptBuilder, concurrency int) (*Executor, error) {
	return &Executor{
		aiClient:      ai,
		promptBuilder: pb,
		concurrency:   concurrency,
	}, nil
}

// ExecuteMap は errgroup と rate.Limiter を使用して、安全かつ効率的に並列実行を行います。
func (e *Executor) ExecuteMap(ctx context.Context, model string, allSegments []domain.Segment) ([]string, error) {
	total := len(allSegments)
	summaries := make([]string, total)

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(e.concurrency)
	limiter := rate.NewLimiter(rate.Every(defaultLLMRateLimit), 1)

	slog.InfoContext(ctx, "セグメントの並列処理を開始します",
		slog.Int("total_segments", total),
		slog.Int("max_parallel", e.concurrency))

	var waitErr error
Loop:
	for i, seg := range allSegments {
		if err := limiter.Wait(ctx); err != nil {
			waitErr = err
			break Loop
		}

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

	// eg.Wait() は、いずれかの Goroutine がエラーを返した場合、そのエラーを返す。
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// ループ中断時のエラー（コンテキストキャンセル等）があればここで返す
	if waitErr != nil {
		return nil, waitErr
	}

	return summaries, nil
}

// ExecuteReduce は現状のロジックを維持
func (e *Executor) ExecuteReduce(ctx context.Context, model, combinedText string) (string, error) {
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
