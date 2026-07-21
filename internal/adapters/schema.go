package adapters

import "google.golang.org/genai"

// mapOutputSchema は、Mapフェーズ(セグメント単位のクリーンアップ)の構造化出力スキーマです。
// 出典URLはGo側が既に保持している(Segment.URL)ため、AIには聞きません。
func mapOutputSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"cleaned_text": {Type: genai.TypeString},
		},
		Required: []string{"cleaned_text"},
	}
}

// reduceOutputSchema は、Reduceフェーズ(統合・構造化)の構造化出力スキーマです。
func reduceOutputSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"title": {Type: genai.TypeString},
			"sections": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"heading":     {Type: genai.TypeString},
						"body":        {Type: genai.TypeString},
						"source_urls": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					},
					Required: []string{"heading", "body"},
				},
			},
		},
		Required: []string{"title", "sections"},
	}
}
