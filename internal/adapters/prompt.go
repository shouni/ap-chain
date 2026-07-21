package adapters

import (
	"encoding/json"
	"fmt"

	"github.com/shouni/go-prompt-kit/prompts"

	"ap-chain/assets"
	"ap-chain/internal/domain"
)

// MapTemplateData は、Mapフェーズ（個別セグメントの要約）で使用するテンプレートデータです。
type MapTemplateData struct {
	SegmentText string
}

// ReduceTemplateData は、Reduceフェーズ（要約の統合と構造化）で使用するテンプレートデータです。
type ReduceTemplateData struct {
	SegmentsJSON string
}

// templateBuilder は、テンプレートに基づいてプロンプト文字列を構築するための内部インターフェースです。
type templateBuilder interface {
	Build(mode string, data any) (string, error)
}

// PromptAdapter は、AP Chain の各処理フェーズに適した AI プロンプトを生成するアダプターです。
type PromptAdapter struct {
	builder templateBuilder
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
func (p *PromptAdapter) GenerateMap(text string) (string, error) {
	data := MapTemplateData{
		SegmentText: text,
	}
	prompt, err := p.builder.Build("map", data)
	if err != nil {
		return "", fmt.Errorf("Mapテンプレートの構築に失敗: %w", err)
	}
	return prompt, nil
}

// GenerateReduce は、Mapフェーズで得られたセグメント群(JSON化)を統合し、
// 最終的な構造化文書を作成するための Reduce プロンプトを構築します。
func (p *PromptAdapter) GenerateReduce(segments []domain.Segment) (string, error) {
	segmentsJSON, err := json.Marshal(segments)
	if err != nil {
		return "", fmt.Errorf("セグメントJSONの構築に失敗: %w", err)
	}

	data := ReduceTemplateData{
		SegmentsJSON: string(segmentsJSON),
	}
	prompt, err := p.builder.Build("reduce", data)
	if err != nil {
		return "", fmt.Errorf("Reduceテンプレートの構築に失敗: %w", err)
	}
	return prompt, nil
}
