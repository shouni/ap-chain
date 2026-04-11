package adapters

import (
	"fmt"

	"github.com/shouni/go-prompt-kit/prompts"

	"ap-chain/assets"
)

type MapTemplateData struct {
	SegmentText string
	SourceURL   string
}

type ReduceTemplateData struct {
	CombinedText string
}

// promptBuilder は、フォーマット済みのプロンプトを作成するためのインターフェース
type promptBuilder interface {
	Build(mode string, data any) (string, error)
}

// PromptAdapter は、さまざまなモードとデータに基づいてプロンプトを生成する役割を担います。
type PromptAdapter struct {
	builder promptBuilder
}

// NewPromptAdapter は動的に読み込んだテンプレートを使用して Builder を構築します。
func NewPromptAdapter() (*PromptAdapter, error) {
	templates, err := assets.LoadPrompts()
	if err != nil {
		return nil, err
	}

	builder, err := prompts.NewBuilder(templates)
	if err != nil {
		return nil, fmt.Errorf("レビュービルダーの構築に失敗: %w", err)
	}

	return &PromptAdapter{
		builder: builder,
	}, nil
}

// GenerateMap はコードレビューのMarkdownレポートを生成します。
func (p *PromptAdapter) GenerateMap(text, url string) (string, error) {
	data := MapTemplateData{
		SegmentText: text,
		SourceURL:   url,
	}
	prompt, err := p.builder.Build("map", data)
	if err != nil {
		return "", fmt.Errorf("マップテンプレートの実行に失敗: %w", err)
	}
	return prompt, nil
}

// GenerateReduce はコードレビューのMarkdownレポートを生成します。
func (p *PromptAdapter) GenerateReduce(text string) (string, error) {
	data := ReduceTemplateData{
		CombinedText: text,
	}
	prompt, err := p.builder.Build("reduce", data)
	if err != nil {
		return "", fmt.Errorf("マップテンプレートの実行に失敗: %w", err)
	}
	return prompt, nil
}
