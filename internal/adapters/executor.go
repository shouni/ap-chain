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

	for i, seg := range allSegments {
		// 事前のエラーチェック
		select {
		case err := <-errChan:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 1. メインループでレートリミットを待機（ゴルーチンの大量生成を防ぐ）
		if err := limiter.Wait(ctx); err != nil {
			e.sendError(errChan, err)
			cancel()
			break
		}

		// 2. セマフォを取得して同時実行数を制限
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			e.sendError(errChan, ctx.Err())
			cancel()
			break
		}

		wg.Add(1)
		go func(index int, s domain.URLResult) {
			defer func() { <-sem }()
			defer wg.Done()

			prompt, err := e.promptBuilder.GenerateMap(s.Content, s.URL)
			if err != nil {
				e.sendError(errChan, fmt.Errorf("セグメント %d 処理失敗: %w", index+1, err))
				cancel() // 他のゴルーチンをキャンセル
				return
			}

			response, err := e.aiClient.GenerateContent(ctx, model, prompt)
			if err != nil {
				e.sendError(errChan, fmt.Errorf("セグメント %d (URL: %s) 処理失敗: %w", index+1, s.URL, err))
				cancel() // 他のゴルーチンをキャンセル
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
