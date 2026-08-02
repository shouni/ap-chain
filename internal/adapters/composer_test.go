package adapters

import (
	"context"
	"errors"
	"testing"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/ap-chain/internal/domain"
)

// mockGeminiClient は gemini.MultimodalGenerator インターフェースのモックです。
// genai の型を含まない 1 メソッドのインターフェースなので、これだけで足ります。
type mockGeminiClient struct {
	generateWithPartsFunc func(opts gemini.GenerateOptions) (*gemini.Response, error)
	lastPrompt            string
}

func (m *mockGeminiClient) GenerateWithAttachments(_ context.Context, _ string, prompt string, _ []gemini.Attachment, opts gemini.GenerateOptions) (*gemini.Response, error) {
	m.lastPrompt = prompt
	if m.generateWithPartsFunc != nil {
		return m.generateWithPartsFunc(opts)
	}
	return &gemini.Response{Text: "{}"}, nil
}

// mockPromptBuilder は PromptBuilder インターフェースのモックです。
type mockPromptBuilder struct {
	generateMapFunc    func(text string) (string, error)
	generateReduceFunc func(segments []domain.Segment) (string, error)
}

func (m *mockPromptBuilder) GenerateMap(text string) (string, error) {
	if m.generateMapFunc != nil {
		return m.generateMapFunc(text)
	}
	return "prompt:" + text, nil
}

func (m *mockPromptBuilder) GenerateReduce(segments []domain.Segment) (string, error) {
	if m.generateReduceFunc != nil {
		return m.generateReduceFunc(segments)
	}
	return "reduce-prompt", nil
}

func TestComposerAdapter_RunMap(t *testing.T) {
	ctx := context.Background()

	t.Run("成功: cleaned_textを取り出し、URLはSegment由来のものを維持する", func(t *testing.T) {
		var gotOpts gemini.GenerateOptions
		mock := &mockGeminiClient{
			generateWithPartsFunc: func(opts gemini.GenerateOptions) (*gemini.Response, error) {
				gotOpts = opts
				return &gemini.Response{Text: `{"cleaned_text":"cleaned"}`}, nil
			},
		}

		c, err := NewComposerAdapter(mock, &mockPromptBuilder{})
		if err != nil {
			t.Fatalf("NewComposerAdapter failed: %v", err)
		}

		segments := []domain.Segment{{Text: "raw text", URL: "https://example.com/a"}}
		got, err := c.RunMap(ctx, "model", segments)
		if err != nil {
			t.Fatalf("RunMap failed: %v", err)
		}

		want := []domain.Segment{{Text: "cleaned", URL: "https://example.com/a"}}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("RunMap() = %+v, want %+v", got, want)
		}
		if gotOpts.ResponseSchema == nil {
			t.Error("ResponseSchemaが構造化出力の制約として渡されている必要がある")
		}
		if gotOpts.ResponseMIMEType != "application/json" {
			t.Errorf("ResponseMIMEType = %q, want application/json", gotOpts.ResponseMIMEType)
		}
	})

	t.Run("異常系: 不正なJSONはエラーになる", func(t *testing.T) {
		mock := &mockGeminiClient{
			generateWithPartsFunc: func(gemini.GenerateOptions) (*gemini.Response, error) {
				return &gemini.Response{Text: "not json"}, nil
			},
		}
		c, err := NewComposerAdapter(mock, &mockPromptBuilder{})
		if err != nil {
			t.Fatalf("NewComposerAdapter failed: %v", err)
		}

		_, err = c.RunMap(ctx, "model", []domain.Segment{{Text: "x", URL: "https://example.com/a"}})
		if err == nil {
			t.Fatal("expected an error for invalid JSON, got nil")
		}
	})
}

func TestComposerAdapter_RunReduce(t *testing.T) {
	ctx := context.Background()

	t.Run("成功: セグメントがプロンプトビルダーに渡り、JSON文字列がそのまま返る", func(t *testing.T) {
		var gotSegments []domain.Segment
		var gotOpts gemini.GenerateOptions
		expected := `{"title":"t","sections":[]}`

		pb := &mockPromptBuilder{
			generateReduceFunc: func(segments []domain.Segment) (string, error) {
				gotSegments = segments
				return "reduce-prompt", nil
			},
		}
		mock := &mockGeminiClient{
			generateWithPartsFunc: func(opts gemini.GenerateOptions) (*gemini.Response, error) {
				gotOpts = opts
				return &gemini.Response{Text: expected}, nil
			},
		}

		c, err := NewComposerAdapter(mock, pb)
		if err != nil {
			t.Fatalf("NewComposerAdapter failed: %v", err)
		}

		segments := []domain.Segment{{Text: "a", URL: "https://example.com/a"}}
		got, err := c.RunReduce(ctx, "model", segments)
		if err != nil {
			t.Fatalf("RunReduce failed: %v", err)
		}
		if got != expected {
			t.Errorf("RunReduce() = %q, want %q", got, expected)
		}
		if len(gotSegments) != 1 || gotSegments[0] != segments[0] {
			t.Errorf("GenerateReduce received %+v, want %+v", gotSegments, segments)
		}
		if gotOpts.ResponseSchema == nil {
			t.Error("ResponseSchemaが構造化出力の制約として渡されている必要がある")
		}
		if gotOpts.ResponseMIMEType != "application/json" {
			t.Errorf("ResponseMIMEType = %q, want application/json", gotOpts.ResponseMIMEType)
		}
	})

	t.Run("異常系: AI呼び出し失敗はエラーになる", func(t *testing.T) {
		pb := &mockPromptBuilder{}
		mock := &mockGeminiClient{
			generateWithPartsFunc: func(gemini.GenerateOptions) (*gemini.Response, error) {
				return nil, errors.New("api error")
			},
		}
		c, err := NewComposerAdapter(mock, pb)
		if err != nil {
			t.Fatalf("NewComposerAdapter failed: %v", err)
		}

		_, err = c.RunReduce(ctx, "model", nil)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}
