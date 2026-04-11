package cleaner

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"ap-chain/internal/domain"
)

// defaultLLMRateLimit は、2sごとに1リクエストを許可するレートリミットです。
const defaultLLMRateLimit = 20 * time.Second

// LLMConcurrentExecutor は LLMExecutor の具体的な実装で、Goroutine、セマフォ、レートリミッターを使用して並列実行を行います。
type LLMConcurrentExecutor struct {
	aiClient      gemini.ContentGenerator
	promptBuilder domain.PromptBuilder
	concurrency   int
}

// NewLLMConcurrentExecutor は新しい LLMConcurrentExecutor インスタンスを作成します。
func NewLLMConcurrentExecutor(ai gemini.ContentGenerator, pb domain.PromptBuilder, concurrency int) (*LLMConcurrentExecutor, error) {
	return &LLMConcurrentExecutor{
		aiClient:      ai,
		promptBuilder: pb,
		concurrency:   concurrency,
	}, nil
}

// MapResult はセグメント処理の結果を保持します。
type MapResult struct {
	Summary string
	Err     error
}

// ExecuteMap は Mapフェーズの並列処理を実行します。
func (e *LLMConcurrentExecutor) ExecuteMap(ctx context.Context, model string, allSegments []Segment) ([]string, error) {
	total := len(allSegments)
	// 結果を格納するスライスをあらかじめ確保（順序を維持するため）
	summaries := make([]string, total)
	errChan := make(chan error, 1) // 最初のエラーだけ捕まえれば良いためバッファ1

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.concurrency)

	// レートリミット管理
	ticker := time.NewTicker(defaultLLMRateLimit)
	defer ticker.Stop()

	slog.Info("セグメントの並列処理を開始します",
		slog.Int("total_segments", total),
		slog.Int("max_parallel", e.concurrency))

	for i, seg := range allSegments {
		// エラーが発生していたら新規 Goroutine 生成を停止
		select {
		case err := <-errChan:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(index int, s Segment) {
			defer func() { <-sem }()
			defer wg.Done()

			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}

			prompt, err := e.promptBuilder.GenerateMap(s.Text, s.URL)
			if err != nil {
				select {
				case errChan <- fmt.Errorf("セグメント %d 処理失敗: %w", index+1, err):
				default:
				}
				return
			}

			response, err := e.aiClient.GenerateContent(ctx, model, prompt)
			if err != nil {
				select {
				case errChan <- fmt.Errorf("セグメント %d (URL: %s) 処理失敗: %w", index+1, s.URL, err):
				default:
				}
				return
			}

			// 指定されたインデックスに直接書き込むことで順序を維持
			summaries[index] = response.Text

			slog.Info("セグメント処理成功", "index", index+1, "url", s.URL)
		}(i, seg)
	}

	wg.Wait()

	// 最後にエラーが届いていないか確認
	select {
	case err := <-errChan:
		return nil, err
	default:
	}

	return summaries, nil
}

// ExecuteReduce は ReduceフェーズのAPI呼び出しを実行します。
func (e *LLMConcurrentExecutor) ExecuteReduce(ctx context.Context, model, combinedText string) (string, error) {
	slog.Info("最終的な構造化（Reduceフェーズ）を開始します。", slog.String("model", model))

	prompt, err := e.promptBuilder.GenerateReduce(combinedText)
	if err != nil {
		return "", fmt.Errorf("最終 Reduce プロンプトの生成に失敗しました: %w", err)
	}

	response, err := e.aiClient.GenerateContent(ctx, model, prompt)
	if err != nil {
		return "", fmt.Errorf("LLM最終構造化処理（Reduceフェーズ）に失敗しました: %w", err)
	}

	slog.Info(
		"Reduce処理成功",
		"model", model,
	)

	return response.Text, nil
}
