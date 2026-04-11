package adapters

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
func (e *LLMConcurrentExecutor) ExecuteMap(ctx context.Context, model string, allSegments []domain.URLResult) ([]string, error) {
	total := len(allSegments)
	summaries := make([]string, total)
	errChan := make(chan error, 1)

	// エラー時に他の処理を即座に停止するためのコンテキスト
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.concurrency)
	limiter := rate.NewLimiter(rate.Every(defaultLLMRateLimit), e.concurrency)

	slog.InfoContext(ctx, "セグメントの並列処理を開始します",
		slog.Int("total_segments", total),
		slog.Int("max_parallel", e.concurrency))

Loop:
	for i, seg := range allSegments {
		// 1. 事前のエラーチェック (Blocker 修正)
		// return ではなく break することで、下部の wg.Wait() を必ず通過させ、リークを防ぐ
		if ctx.Err() != nil {
			break Loop
		}

		// 2. メインループでレートリミットを待機
		if err := limiter.Wait(ctx); err != nil {
			e.sendError(errChan, err)
			cancel()
			break Loop
		}

		// 3. セマフォを取得
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			e.sendError(errChan, ctx.Err())
			cancel()
			break Loop
		}

		wg.Add(1)
		go func(index int, s domain.URLResult) {
			defer func() { <-sem }()
			defer wg.Done()

			prompt, err := e.promptBuilder.GenerateMap(s.Content, s.URL)
			if err != nil {
				e.sendError(errChan, fmt.Errorf("セグメント %d 処理失敗: %w", index+1, err))
				cancel()
				return
			}

			response, err := e.aiClient.GenerateContent(ctx, model, prompt)
			if err != nil {
				e.sendError(errChan, fmt.Errorf("セグメント %d (URL: %s) 処理失敗: %w", index+1, s.URL, err))
				cancel()
				return
			}

			summaries[index] = response.Text

			slog.InfoContext(ctx, "セグメント処理成功",
				slog.Int("index", index+1),
				slog.String("url", s.URL))
		}(i, seg)
	}

	// すべての起動済み Goroutine が完了（またはキャンセル検知で終了）するのを待機
	wg.Wait()

	// 4. 最終的なエラー評価
	select {
	case err := <-errChan:
		return nil, err
	default:
		// errChan が空でもコンテキストがキャンセルされていればそのエラーを返す
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return summaries, nil
}

func (e *LLMConcurrentExecutor) sendError(ch chan error, err error) {
	select {
	case ch <- err:
	default:
	}
}

// ExecuteReduce は変更なし
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
