package adapters

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shouni/go-notify/notify"
)

// recordingNotifier は送信された notify.Message を記録するフェイクです。
// Slack 記法への変換は go-notify 側の責務なので、ここでは ap-chain が
// 組み立てた見出しと本文だけを検証します。
type recordingNotifier struct {
	got []notify.Message
}

// Notify は notify.Notifier を実装し、受け取った Message を記録します。
func (r *recordingNotifier) Notify(_ context.Context, msg notify.Message) error {
	r.got = append(r.got, msg)
	return nil
}

// last は最後に送信された Message を返します。
func (r *recordingNotifier) last(t *testing.T) notify.Message {
	t.Helper()
	if len(r.got) == 0 {
		t.Fatal("通知が送信されていません")
	}
	return r.got[len(r.got)-1]
}

// newTestSlackAdapter は記録用 Notifier を差し込んだアダプターを返します。
func newTestSlackAdapter() (*SlackAdapter, *recordingNotifier) {
	rec := &recordingNotifier{}
	return &SlackAdapter{pipeline: notify.NewPipeline(rec, slackTitles)}, rec
}

// TestNotifySuccessBuildsBody は完了通知の見出しと本文を検証します。
func TestNotifySuccessBuildsBody(t *testing.T) {
	t.Parallel()

	adapter, rec := newTestSlackAdapter()

	err := adapter.NotifySuccess(context.Background(),
		"gs://ap-chain/out/doc.md", "https://signed.example.com/doc.md", 3)
	if err != nil {
		t.Fatalf("NotifySuccess() error = %v", err)
	}

	msg := rec.last(t)
	if msg.Title != slackTitles.Success {
		t.Errorf("Title = %q, want %q", msg.Title, slackTitles.Success)
	}

	want := "**出力先:** [gs://ap-chain/out/doc.md](https://signed.example.com/doc.md)\n" +
		"**ソース数:** `3 URLs`\n" +
		"**ステータス:** `Success`"
	if msg.Body != want {
		t.Errorf("Body =\n%q\nwant\n%q", msg.Body, want)
	}
}

// TestNotifySuccessOmitsLinkWithoutPublicURL は、公開URLが無い場合に出力先の行が
// 省かれることを検証します。ローカル出力では署名付きURLが生成されません。
func TestNotifySuccessOmitsLinkWithoutPublicURL(t *testing.T) {
	t.Parallel()

	adapter, rec := newTestSlackAdapter()

	if err := adapter.NotifySuccess(context.Background(), "out/doc.md", "", 1); err != nil {
		t.Fatalf("NotifySuccess() error = %v", err)
	}

	body := rec.last(t).Body
	if strings.Contains(body, "出力先") {
		t.Errorf("Body = %q, 公開URLが無い場合は出力先の行を出さない想定です", body)
	}
	if !strings.Contains(body, "**ソース数:** `1 URLs`") {
		t.Errorf("Body = %q, want the source count", body)
	}
}

// TestNotifyFailureIncludesCause は失敗通知が原因を含むことを検証します。
func TestNotifyFailureIncludesCause(t *testing.T) {
	t.Parallel()

	adapter, rec := newTestSlackAdapter()

	if err := adapter.NotifyFailure(context.Background(), errors.New("収集に失敗しました")); err != nil {
		t.Fatalf("NotifyFailure() error = %v", err)
	}

	msg := rec.last(t)
	if msg.Title != slackTitles.Failure {
		t.Errorf("Title = %q, want %q", msg.Title, slackTitles.Failure)
	}
	if msg.Body != "**エラー内容:**\n収集に失敗しました" {
		t.Errorf("Body = %q, want the cause", msg.Body)
	}
}

// TestNotifyFailureWithNilCause は、原因が nil でも通知が壊れないことを検証します。
func TestNotifyFailureWithNilCause(t *testing.T) {
	t.Parallel()

	adapter, rec := newTestSlackAdapter()

	if err := adapter.NotifyFailure(context.Background(), nil); err != nil {
		t.Fatalf("NotifyFailure() error = %v", err)
	}

	if body := rec.last(t).Body; !strings.Contains(body, "**エラー内容:**\n"+notify.NotAvailable) {
		t.Errorf("Body = %q, want the N/A fallback", body)
	}
}

// TestNewSlackAdapterDisabledWhenWebhookURLEmpty は、Webhook URL が未設定なら
// エラーにならず通知が無効化されることを検証します。
func TestNewSlackAdapterDisabledWhenWebhookURLEmpty(t *testing.T) {
	t.Parallel()

	adapter, err := NewSlackAdapter(nil, "")
	if err != nil {
		t.Fatalf("NewSlackAdapter() error = %v", err)
	}
	if adapter.pipeline.Enabled() {
		t.Fatal("Webhook URL 未設定なのに通知が有効になっています")
	}

	ctx := context.Background()
	if err := adapter.NotifySuccess(ctx, "gs://a/b", "https://c", 1); err != nil {
		t.Errorf("NotifySuccess() = %v, want nil", err)
	}
	if err := adapter.NotifyFailure(ctx, errors.New("boom")); err != nil {
		t.Errorf("NotifyFailure() = %v, want nil", err)
	}
}

// TestNewSlackAdapterRequiresHTTPClientWhenWebhookSet は、Webhook URL があるのに
// HTTP クライアントが nil の場合はエラーになることを検証します。
func TestNewSlackAdapterRequiresHTTPClientWhenWebhookSet(t *testing.T) {
	t.Parallel()

	if _, err := NewSlackAdapter(nil, "https://hooks.slack.example/webhook"); err == nil {
		t.Fatal("HTTPクライアントが nil なのにエラーになりません")
	}
}

// TestNotifySetsLevel は、完了・失敗が種別を伴って送信されることを検証します。
// Slack 側はこれを attachment の色に落とすため、見出しの絵文字とは別に必要です。
func TestNotifySetsLevel(t *testing.T) {
	tests := []struct {
		name string
		call func(a *SlackAdapter) error
		want notify.Level
	}{
		{
			name: "完了",
			call: func(a *SlackAdapter) error {
				return a.NotifySuccess(context.Background(), "gs://out/doc.md", "https://example.com/doc.md", 3)
			},
			want: notify.LevelSuccess,
		},
		{
			name: "失敗",
			call: func(a *SlackAdapter) error {
				return a.NotifyFailure(context.Background(), errors.New("boom"))
			},
			want: notify.LevelFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, rec := newTestSlackAdapter()

			if err := tt.call(adapter); err != nil {
				t.Fatalf("通知に失敗しました: %v", err)
			}
			if got := rec.last(t).Level; got != tt.want {
				t.Errorf("Level = %v, want %v", got, tt.want)
			}
		})
	}
}
