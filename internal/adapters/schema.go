package adapters

import "github.com/shouni/go-gemini-client/gemini"

// mapOutputSchema は、Mapフェーズ(セグメント単位のクリーンアップ)の構造化出力スキーマです。
// 出典URLはGo側が既に保持している(Segment.URL)ため、AIには聞きません。
func mapOutputSchema() *gemini.Schema {
	return &gemini.Schema{
		Type: gemini.TypeObject,
		Properties: map[string]*gemini.Schema{
			"cleaned_text": {Type: gemini.TypeString},
		},
		Required: []string{"cleaned_text"},
	}
}

// reduceOutputSchema は、Reduceフェーズ(統合・構造化)の構造化出力スキーマです。
func reduceOutputSchema() *gemini.Schema {
	return &gemini.Schema{
		Type: gemini.TypeObject,
		Properties: map[string]*gemini.Schema{
			"title": {Type: gemini.TypeString},
			"sections": {
				Type: gemini.TypeArray,
				Items: &gemini.Schema{
					Type: gemini.TypeObject,
					Properties: map[string]*gemini.Schema{
						"heading":     {Type: gemini.TypeString},
						"body":        {Type: gemini.TypeString},
						"source_urls": {Type: gemini.TypeArray, Items: &gemini.Schema{Type: gemini.TypeString}},
					},
					Required: []string{"heading", "body"},
				},
			},
		},
		Required: []string{"title", "sections"},
	}
}
