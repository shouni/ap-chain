package adapters

import (
	"strings"
	"testing"

	"github.com/shouni/ap-chain/internal/domain"
)

func TestPromptAdapter_GenerateMap(t *testing.T) {
	pa, err := NewPromptAdapter()
	if err != nil {
		t.Fatalf("NewPromptAdapter failed: %v", err)
	}

	prompt, err := pa.GenerateMap("segment body")
	if err != nil {
		t.Fatalf("GenerateMap failed: %v", err)
	}
	if !strings.Contains(prompt, "segment body") {
		t.Errorf("prompt does not contain segment text:\n%s", prompt)
	}
	if !strings.Contains(prompt, "cleaned_text") {
		t.Errorf("prompt does not describe the cleaned_text output field:\n%s", prompt)
	}
	if strings.Contains(prompt, "<no value>") {
		t.Errorf("prompt has unresolved template placeholder:\n%s", prompt)
	}
}

func TestPromptAdapter_GenerateReduce(t *testing.T) {
	pa, err := NewPromptAdapter()
	if err != nil {
		t.Fatalf("NewPromptAdapter failed: %v", err)
	}

	segments := []domain.Segment{{Text: "cleaned body", URL: "https://example.com/a"}}
	prompt, err := pa.GenerateReduce(segments)
	if err != nil {
		t.Fatalf("GenerateReduce failed: %v", err)
	}
	if !strings.Contains(prompt, "cleaned body") || !strings.Contains(prompt, "https://example.com/a") {
		t.Errorf("prompt does not contain the segments JSON:\n%s", prompt)
	}
	if !strings.Contains(prompt, "source_urls") {
		t.Errorf("prompt does not describe the source_urls output field:\n%s", prompt)
	}
	if strings.Contains(prompt, "<no value>") {
		t.Errorf("prompt has unresolved template placeholder:\n%s", prompt)
	}
}
