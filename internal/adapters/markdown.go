package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// reportOutput は、Reduceフェーズの構造化出力(reduceOutputSchema)に対応する構造です。
type reportOutput struct {
	Title    string          `json:"title"`
	Sections []reportSection `json:"sections"`
}

type reportSection struct {
	Heading    string   `json:"heading"`
	Body       string   `json:"body"`
	SourceURLs []string `json:"source_urls"`
}

// MarkdownConverter は、構造化レポートJSONを決定的にMarkdownへ整形します。
// AIの自由記述に頼らずコード側でMarkdownを組み立てることで、見出しレベルのズレや
// 出典URLの記載漏れといった出力揺れを避けます。
type MarkdownConverter struct{}

// NewMarkdownConverter は MarkdownConverter の新しいインスタンスを作成します。
func NewMarkdownConverter() *MarkdownConverter {
	return &MarkdownConverter{}
}

// Run は、構造化レポートJSONをMarkdownへ変換します。
func (c *MarkdownConverter) Run(content []byte) (io.Reader, error) {
	var out reportOutput
	if err := json.Unmarshal(content, &out); err != nil {
		return nil, fmt.Errorf("レポートJSONのパースに失敗しました: %w", err)
	}

	var b bytes.Buffer
	if out.Title != "" {
		fmt.Fprintf(&b, "# %s\n\n", out.Title)
	}

	for _, s := range out.Sections {
		if s.Heading != "" {
			fmt.Fprintf(&b, "## %s\n\n", s.Heading)
		}
		if s.Body != "" {
			fmt.Fprintf(&b, "%s\n\n", s.Body)
		}
		if len(s.SourceURLs) > 0 {
			b.WriteString("### 関連URL\n\n")
			for _, u := range s.SourceURLs {
				fmt.Fprintf(&b, "- %s\n", u)
			}
			b.WriteString("\n")
		}
	}

	return &b, nil
}
