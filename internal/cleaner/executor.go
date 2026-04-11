package cleaner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"golang.org/x/time/rate"

	"ap-chain/internal/domain"
)

// defaultLLMRateLimit は、レートリミットを制御するための間隔です。
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

// ExecuteMap は Mapフェーズの並列処理を効率的に実行します。
func (e *LLMConcurrentExecutor) ExecuteMap(ctx context.Context, model string, allSegments []Segment) ([]string, error) {
	total := len(allSegments)
	summaries := make([]string, total)
	errChan := make(chan error, 1)

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.concurrency)

	limiter := rate.NewLimiter(rate.Every(defaultLLMRateLimit), 1)

	slog.InfoContext(ctx, "セグメントの並列処理を開始します",
		slog.Int("total_segments", total),
		slog.Int("max_parallel", e.concurrency))

	for i, seg := range allSegments {
		// 事前のエラーチェック
		select {
		case err := <-errChan:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(index int, s Segment) {
			defer wg.Done()

			// 1. レートリミットの待機（セマフォ取得前に行うことでスロットを無駄に占有しない）
			if err := limiter.Wait(ctx); err != nil {
				return
			}

			// 2. セマフォ（同時実行数）の取得
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			// 3. LLM 処理の実行
			prompt, err := e.promptBuilder.GenerateMap(s.Text, s.URL)
			if err != nil {
				e.sendError(errChan, fmt.Errorf("セグメント %d 処理失敗: %w", index+1, err))
				return
			}

			response, err := e.aiClient.GenerateContent(ctx, model, prompt)
			if err != nil {
				e.sendError(errChan, fmt.Errorf("セグメント %d (URL: %s) 処理失敗: %w", index+1, s.URL, err))
				return
			}

			summaries[index] = response.Text

			slog.InfoContext(ctx, "セグメント処理成功",
				slog.Int("index", index+1),
				slog.String("url", s.URL))
		}(i, seg)
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return nil, err
	default:
	}

	return summaries, nil
}

// sendError は安全に最初のエラーをチャネルに送信します。
func (e *LLMConcurrentExecutor) sendError(ch chan error, err error) {
	select {
	case ch <- err:
	default:
		// すでにエラーが送信済みの場合は何もしない
	}
}

// ExecuteReduce は ReduceフェーズのAPI呼び出しを実行します。
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
