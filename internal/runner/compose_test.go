package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/shouni/ap-chain/internal/domain"
)

// stubComposer は Composer の呼び出しを記録するテスト用の実装です。
type stubComposer struct {
	mapFunc    func(ctx context.Context, model string, segs []domain.Segment) ([]domain.Segment, error)
	reduceFunc func(ctx context.Context, model string, segs []domain.Segment) (string, error)

	mapModel     string
	reduceModel  string
	mapSegments  []domain.Segment
	reduceInput  []domain.Segment
	reduceCalled bool
}

func (s *stubComposer) RunMap(ctx context.Context, model string, segs []domain.Segment) ([]domain.Segment, error) {
	s.mapModel = model
	s.mapSegments = segs
	if s.mapFunc != nil {
		return s.mapFunc(ctx, model, segs)
	}
	return segs, nil
}

func (s *stubComposer) RunReduce(ctx context.Context, model string, segs []domain.Segment) (string, error) {
	s.reduceCalled = true
	s.reduceModel = model
	s.reduceInput = segs
	if s.reduceFunc != nil {
		return s.reduceFunc(ctx, model, segs)
	}
	return `{"title":"t"}`, nil
}

func testModels() Models {
	return Models{MapModel: "map-model", ReduceModel: "reduce-model"}
}

func TestNewComposeRunner(t *testing.T) {
	tests := []struct {
		name     string
		composer Composer
		models   Models
		wantErr  bool
	}{
		{name: "正常系", composer: &stubComposer{}, models: testModels()},
		{name: "異常系: composerがnil", composer: nil, models: testModels(), wantErr: true},
		{
			name:     "異常系: MapModelが空",
			composer: &stubComposer{},
			models:   Models{ReduceModel: "r"},
			wantErr:  true,
		},
		{
			name:     "異常系: ReduceModelが空",
			composer: &stubComposer{},
			models:   Models{MapModel: "m"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewComposeRunner(tt.composer, tt.models)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewComposeRunner() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComposeRunner_Run(t *testing.T) {
	ctx := context.Background()

	t.Run("正常系: MapとReduceが設定されたモデルで順に呼ばれる", func(t *testing.T) {
		stub := &stubComposer{
			reduceFunc: func(context.Context, string, []domain.Segment) (string, error) {
				return "  {\"title\":\"結果\"}  ", nil
			},
		}
		r, err := NewComposeRunner(stub, testModels())
		if err != nil {
			t.Fatalf("NewComposeRunner() error = %v", err)
		}

		got, err := r.Run(ctx, []domain.URLResult{
			{URL: "https://example.com/a", Content: "本文A"},
			{URL: "https://example.com/b", Content: "本文B"},
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		// 前後の空白は落とされる
		if want := `{"title":"結果"}`; got != want {
			t.Errorf("Run() = %q, want %q", got, want)
		}
		if stub.mapModel != "map-model" || stub.reduceModel != "reduce-model" {
			t.Errorf("モデルの割り当てが不正です: map=%q reduce=%q", stub.mapModel, stub.reduceModel)
		}
		// URL ごとにセグメント化され、出典URLが保持される
		if len(stub.mapSegments) != 2 {
			t.Fatalf("Mapへ渡ったセグメント数 = %d, want 2", len(stub.mapSegments))
		}
		if stub.mapSegments[0].URL != "https://example.com/a" {
			t.Errorf("出典URLが保持されていません: %q", stub.mapSegments[0].URL)
		}
	})

	t.Run("異常系: urlsが空", func(t *testing.T) {
		r, _ := NewComposeRunner(&stubComposer{}, testModels())
		if _, err := r.Run(ctx, nil); err == nil {
			t.Error("空の urls でエラーになりませんでした")
		}
	})

	t.Run("異常系: Mapが失敗するとReduceは呼ばれない", func(t *testing.T) {
		wantErr := errors.New("map failed")
		stub := &stubComposer{
			mapFunc: func(context.Context, string, []domain.Segment) ([]domain.Segment, error) {
				return nil, wantErr
			},
		}
		r, _ := NewComposeRunner(stub, testModels())

		_, err := r.Run(ctx, []domain.URLResult{{URL: "u", Content: "c"}})
		if !errors.Is(err, wantErr) {
			t.Errorf("元のエラーが伝播していません: %v", err)
		}
		if stub.reduceCalled {
			t.Error("Mapが失敗したのに Reduce が呼ばれました")
		}
	})

	t.Run("異常系: Mapが空を返したらReduceは呼ばれない", func(t *testing.T) {
		// 空セグメントで Reduce を呼ぶと中身のないレポートが生成されてしまう
		stub := &stubComposer{
			mapFunc: func(context.Context, string, []domain.Segment) ([]domain.Segment, error) {
				return nil, nil
			},
		}
		r, _ := NewComposeRunner(stub, testModels())

		if _, err := r.Run(ctx, []domain.URLResult{{URL: "u", Content: "c"}}); err == nil {
			t.Error("Mapが空を返したのにエラーになりませんでした")
		}
		if stub.reduceCalled {
			t.Error("Mapが空なのに Reduce が呼ばれました")
		}
	})

	t.Run("異常系: Reduceの結果が空白のみ", func(t *testing.T) {
		stub := &stubComposer{
			reduceFunc: func(context.Context, string, []domain.Segment) (string, error) {
				return "   \n  ", nil
			},
		}
		r, _ := NewComposeRunner(stub, testModels())

		if _, err := r.Run(ctx, []domain.URLResult{{URL: "u", Content: "c"}}); err == nil {
			t.Error("空の Reduce 結果でエラーになりませんでした")
		}
	})
}

func TestSegmentText(t *testing.T) {
	ctx := context.Background()

	t.Run("上限以下なら分割されない", func(t *testing.T) {
		got := segmentText(ctx, "あいうえお", 10)
		if len(got) != 1 || got[0] != "あいうえお" {
			t.Errorf("segmentText() = %q, want 1 segment", got)
		}
	})

	t.Run("空文字はセグメントを返さない", func(t *testing.T) {
		if got := segmentText(ctx, "", 10); len(got) != 0 {
			t.Errorf("segmentText(\"\") = %q, want empty", got)
		}
	})

	t.Run("各セグメントが上限を超えない", func(t *testing.T) {
		text := strings.Repeat("あ", 60) + defaultSeparator + strings.Repeat("い", 60)
		for i, seg := range segmentText(ctx, text, 50) {
			if n := utf8.RuneCountInString(seg); n > 50 {
				t.Errorf("seg[%d] は %d 文字で上限50を超えています", i, n)
			}
		}
	})

	t.Run("結合すると元のテキストへ戻る", func(t *testing.T) {
		text := strings.Repeat("あ"+defaultSeparator+"い", 100)
		if got := strings.Join(segmentText(ctx, text, 50), ""); got != text {
			t.Errorf("分割・結合でテキストが変化しました (元 %d bytes, 結合 %d bytes)", len(text), len(got))
		}
	})

	t.Run("区切りが後半にあれば区切りで分割される", func(t *testing.T) {
		// 前半40文字＋区切り＋後半。区切り位置は maxChars/2 より後ろなので採用される
		text := strings.Repeat("あ", 40) + defaultSeparator + strings.Repeat("い", 40)
		got := segmentText(ctx, text, 50)
		if len(got) < 2 {
			t.Fatalf("分割されませんでした: %d セグメント", len(got))
		}
		if !strings.HasSuffix(got[0], defaultSeparator) {
			t.Errorf("区切りで分割されていません: %q", got[0])
		}
	})

	t.Run("区切りが前半すぎる場合は上限で強制分割される", func(t *testing.T) {
		// 区切りが maxChars/2 より手前にしかないため、そこでは切らない
		text := "あ" + defaultSeparator + strings.Repeat("い", 100)
		got := segmentText(ctx, text, 50)
		if len(got) < 2 {
			t.Fatalf("分割されませんでした: %d セグメント", len(got))
		}
		if n := utf8.RuneCountInString(got[0]); n != 50 {
			t.Errorf("強制分割の長さ = %d, want 50", n)
		}
	})

	t.Run("マルチバイト文字がセグメント境界で壊れない", func(t *testing.T) {
		text := strings.Repeat("あ", 200)
		for i, seg := range segmentText(ctx, text, 30) {
			if !utf8.ValidString(seg) {
				t.Errorf("seg[%d] が不正なUTF-8です", i)
			}
		}
	})

	// maxChars が 0 以下でも無限ループしないことを保証します。
	// ガードが無いと切り出し幅が 0 になり、空セグメントを無限に積み続けます。
	t.Run("maxCharsが0以下でも停止する", func(t *testing.T) {
		for _, maxChars := range []int{0, -1} {
			done := make(chan []string, 1)
			go func() { done <- segmentText(ctx, "あいうえお", maxChars) }()

			select {
			case got := <-done:
				if len(got) != 1 || got[0] != "あいうえお" {
					t.Errorf("maxChars=%d: segmentText() = %q, want 全文1セグメント", maxChars, got)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("maxChars=%d で処理が返りませんでした", maxChars)
			}
		}
	})
}
