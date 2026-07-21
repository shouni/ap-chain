package adapters

import (
	"io"
	"testing"
)

func TestMarkdownConverter_Run(t *testing.T) {
	c := NewMarkdownConverter()

	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "正常系: title・sections・source_urlsが変換されること",
			content: `{"title":"レポート","sections":[{"heading":"見出し1","body":"本文1","source_urls":["https://example.com/a","https://example.com/b"]}]}`,
			want:    "# レポート\n\n## 見出し1\n\n本文1\n\n### 関連URL\n\n- https://example.com/a\n- https://example.com/b\n\n",
		},
		{
			name:    "正常系: source_urlsが空の場合は関連URLセクションが出ない",
			content: `{"title":"t","sections":[{"heading":"h","body":"b","source_urls":[]}]}`,
			want:    "# t\n\n## h\n\nb\n\n",
		},
		{
			name:    "正常系: sectionsが空",
			content: `{"title":"t","sections":[]}`,
			want:    "# t\n\n",
		},
		{
			name:    "異常系: 不正なJSON",
			content: "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := c.Run([]byte(tt.content))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("failed to read result: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Run() = %q, want %q", string(got), tt.want)
			}
		})
	}
}
