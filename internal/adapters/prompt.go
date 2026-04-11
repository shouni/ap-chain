package adapters

import (
	"fmt"

	"github.com/shouni/go-prompt-kit/prompts"

	"ap-chain/assets"
)

// MapTemplateData は、Mapフェーズ（個別セグメントの要約）で使用するテンプレートデータです。
type MapTemplateData struct {
	SegmentText string
	SourceURL   string
}

// ReduceTemplateData は、Reduceフェーズ（要約の統合と構造化）で使用するテンプレートデータです。
type ReduceTemplateData struct {
	CombinedText string
}

// promptBuilder は、テンプレートに基づいてプロンプト文字列を構築するための内部インターフェースです。
type promptBuilder interface {
	Build(mode string, data any) (string, error)
}

// PromptAdapter は、AP Chain の各処理フェーズに適した AI プロンプトを生成するアダプターです。
type PromptAdapter struct {
	builder promptBuilder
}

// NewPromptAdapter は、埋め込まれたアセットからテンプレートを読み込み、アダプターを初期化します。
func NewPromptAdapter() (*PromptAdapter, error) {
	templates, err := assets.LoadPrompts()
	if err != nil {
		return nil, err
	}

	builder, err := prompts.NewBuilder(templates)
	if err != nil {
		return nil, fmt.Errorf("プロンプトビルダーの構築に失敗: %w", err)
	}

	return &PromptAdapter{
		builder: builder,
	}, nil
}

// GenerateMap は、個別セグメントから中間要約を生成するための Map プロンプトを構築します。
func (p *PromptAdapter) GenerateMap(text, url string) (string, error) {
	data := MapTemplateData{
		SegmentText: text,
		SourceURL:   url,
	}
	prompt, err := p.builder.Build("map", data)
	if err != nil {
		return "", fmt.Errorf("Mapテンプレートの構築に失敗: %w", err)
	}
	return prompt, nil
}

// GenerateReduce は、中間要約群を統合し、最終的な構造化文書を作成するための Reduce プロンプトを構築します。
func (p *PromptAdapter) GenerateReduce(text string) (string, error) {
	data := ReduceTemplateData{
		CombinedText: text,
	}
	prompt, err := p.builder.Build("reduce", data)
	if err != nil {
		return "", fmt.Errorf("Reduceテンプレートの構築に失敗: %w", err)
	}
	return prompt, nil
}
