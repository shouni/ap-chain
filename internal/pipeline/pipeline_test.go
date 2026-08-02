package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shouni/ap-chain/internal/domain"
)

type stubCollector struct {
	results []domain.URLResult
	err     error
	called  bool
}

func (s *stubCollector) Run(context.Context, string) ([]domain.URLResult, error) {
	s.called = true
	return s.results, s.err
}

type stubComposer struct {
	content string
	err     error
	called  bool
	input   []domain.URLResult
}

func (s *stubComposer) Run(_ context.Context, results []domain.URLResult) (string, error) {
	s.called = true
	s.input = results
	return s.content, s.err
}

type stubPublisher struct {
	result  *domain.PublishResult
	err     error
	called  bool
	uri     string
	content string
}

func (s *stubPublisher) Run(_ context.Context, uri, content string) (*domain.PublishResult, error) {
	s.called = true
	s.uri = uri
	s.content = content
	return s.result, s.err
}

type stubNotifier struct {
	successCalls int
	failureCalls int
	lastCount    int
	lastErr      error
	returnErr    error
}

func (s *stubNotifier) NotifySuccess(_ context.Context, _, _ string, count int) error {
	s.successCalls++
	s.lastCount = count
	return s.returnErr
}

func (s *stubNotifier) NotifyFailure(_ context.Context, err error) error {
	s.failureCalls++
	s.lastErr = err
	return s.returnErr
}

func okPublishResult() *domain.PublishResult {
	return &domain.PublishResult{
		Markdown: domain.PublishedFile{
			StorageURI: "gs://bucket/out.md",
			PublicURL:  "https://signed.example.com/out.md",
		},
	}
}

func validRequest() domain.Request {
	return domain.Request{InputURI: "urls.txt", OutputURI: "gs://bucket/out.md"}
}

func TestPipeline_Execute_Success(t *testing.T) {
	col := &stubCollector{results: []domain.URLResult{
		{URL: "https://example.com/a", Content: "A"},
		{URL: "https://example.com/b", Content: "B"},
	}}
	com := &stubComposer{content: `{"title":"t"}`}
	pub := &stubPublisher{result: okPublishResult()}
	n := &stubNotifier{}

	if err := New(col, com, pub, n).Execute(context.Background(), validRequest()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !col.called || !com.called || !pub.called {
		t.Errorf("全ステップが実行されていません: collect=%v compose=%v publish=%v",
			col.called, com.called, pub.called)
	}
	// Collect の結果が Compose へ、Compose の結果が Publish へ渡る
	if len(com.input) != 2 {
		t.Errorf("Compose へ渡った件数 = %d, want 2", len(com.input))
	}
	if pub.content != `{"title":"t"}` {
		t.Errorf("Publish へ渡った内容 = %q", pub.content)
	}
	if pub.uri != "gs://bucket/out.md" {
		t.Errorf("Publish へ渡った URI = %q", pub.uri)
	}
	// 成功通知にはソース件数が載る
	if n.successCalls != 1 || n.failureCalls != 0 {
		t.Errorf("通知回数が不正です: success=%d failure=%d", n.successCalls, n.failureCalls)
	}
	if n.lastCount != 2 {
		t.Errorf("成功通知のソース件数 = %d, want 2", n.lastCount)
	}
}

func TestPipeline_Execute_Validation(t *testing.T) {
	tests := []struct {
		name string
		req  domain.Request
	}{
		{name: "InputURIが空", req: domain.Request{OutputURI: "out"}},
		{name: "OutputURIが空", req: domain.Request{InputURI: "in"}},
		{name: "両方空", req: domain.Request{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := &stubCollector{}
			n := &stubNotifier{}

			err := New(col, &stubComposer{}, &stubPublisher{}, n).Execute(context.Background(), tt.req)
			if err == nil {
				t.Fatal("バリデーションエラーになりませんでした")
			}
			if col.called {
				t.Error("バリデーション失敗なのに Collect が実行されました")
			}
			// バリデーション失敗も失敗通知の対象
			if n.failureCalls != 1 {
				t.Errorf("失敗通知の回数 = %d, want 1", n.failureCalls)
			}
		})
	}
}

func TestPipeline_Execute_StepFailures(t *testing.T) {
	stepErr := errors.New("boom")

	tests := []struct {
		name          string
		col           *stubCollector
		com           *stubComposer
		pub           *stubPublisher
		wantSubstr    string
		wantComCalled bool
		wantPubCalled bool
	}{
		{
			name:       "Collectが失敗すると以降は実行されない",
			col:        &stubCollector{err: stepErr},
			com:        &stubComposer{content: "x"},
			pub:        &stubPublisher{result: okPublishResult()},
			wantSubstr: "collection process failed",
		},
		{
			name:          "Composeが失敗するとPublishは実行されない",
			col:           &stubCollector{results: []domain.URLResult{{URL: "u", Content: "c"}}},
			com:           &stubComposer{err: stepErr},
			pub:           &stubPublisher{result: okPublishResult()},
			wantSubstr:    "composition process failed",
			wantComCalled: true,
		},
		{
			name:          "Composeの結果が空白のみなら失敗する",
			col:           &stubCollector{results: []domain.URLResult{{URL: "u", Content: "c"}}},
			com:           &stubComposer{content: "   \n "},
			pub:           &stubPublisher{result: okPublishResult()},
			wantSubstr:    "composed content is empty",
			wantComCalled: true,
		},
		{
			name:          "Publishが失敗する",
			col:           &stubCollector{results: []domain.URLResult{{URL: "u", Content: "c"}}},
			com:           &stubComposer{content: "x"},
			pub:           &stubPublisher{err: stepErr},
			wantSubstr:    "publish process failed",
			wantComCalled: true,
			wantPubCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &stubNotifier{}

			err := New(tt.col, tt.com, tt.pub, n).Execute(context.Background(), validRequest())
			if err == nil {
				t.Fatal("エラーになりませんでした")
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantSubstr)
			}
			if tt.com.called != tt.wantComCalled {
				t.Errorf("Compose called = %v, want %v", tt.com.called, tt.wantComCalled)
			}
			if tt.pub.called != tt.wantPubCalled {
				t.Errorf("Publish called = %v, want %v", tt.pub.called, tt.wantPubCalled)
			}
			// 失敗時は必ず失敗通知が飛び、成功通知は飛ばない
			if n.failureCalls != 1 || n.successCalls != 0 {
				t.Errorf("通知回数が不正です: success=%d failure=%d", n.successCalls, n.failureCalls)
			}
		})
	}
}

func TestPipeline_Notifier(t *testing.T) {
	t.Run("Notifierがnilでも落ちない", func(t *testing.T) {
		p := New(
			&stubCollector{results: []domain.URLResult{{URL: "u", Content: "c"}}},
			&stubComposer{content: "x"},
			&stubPublisher{result: okPublishResult()},
			nil,
		)
		if err := p.Execute(context.Background(), validRequest()); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	t.Run("通知の失敗は処理結果に影響しない", func(t *testing.T) {
		n := &stubNotifier{returnErr: errors.New("slack down")}
		p := New(
			&stubCollector{results: []domain.URLResult{{URL: "u", Content: "c"}}},
			&stubComposer{content: "x"},
			&stubPublisher{result: okPublishResult()},
			n,
		)
		if err := p.Execute(context.Background(), validRequest()); err != nil {
			t.Errorf("通知失敗が Execute のエラーになっています: %v", err)
		}
	})

	// 通知は context.WithoutCancel で切り離されるため、
	// 呼び出し元がキャンセル済みでも失敗通知は届く必要があります。
	t.Run("キャンセル済みcontextでも失敗通知は送られる", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		n := &stubNotifier{}
		p := New(&stubCollector{err: errors.New("boom")}, &stubComposer{}, &stubPublisher{}, n)

		if err := p.Execute(ctx, validRequest()); err == nil {
			t.Fatal("エラーになりませんでした")
		}
		if n.failureCalls != 1 {
			t.Errorf("失敗通知の回数 = %d, want 1", n.failureCalls)
		}
	})
}
