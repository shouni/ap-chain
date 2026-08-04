package adapters

import (
	"context"
	"fmt"
	"strconv"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-notify/notify"
	"github.com/shouni/go-notify/slack"

	"github.com/shouni/ap-chain/internal/domain"
)

// slackTitles はパイプラインの結果ごとの見出しです。
// スキップ通知は行わないため Skipped は設定していません。
var slackTitles = notify.Titles{
	Success: "✅ AP Chain: 構造化ドキュメントの生成が完了しました",
	Failure: "❌ AP Chain: 実行エラーが発生しました",
}

// SlackAdapter は Slack への通知を担当します。
type SlackAdapter struct {
	pipeline *notify.Pipeline
}

var _ domain.Notifier = (*SlackAdapter)(nil)

// NewSlackAdapter は新しい Slack アダプターを初期化します。
// webhookURL が空の場合は通知を行わないアダプターを返します。
func NewSlackAdapter(httpClient httpkit.Requester, webhookURL string) (*SlackAdapter, error) {
	notifier, err := slack.NewNotifier(httpClient, webhookURL)
	if err != nil {
		return nil, fmt.Errorf("通知クライアント(Slack)の初期化に失敗しました: %w", err)
	}

	return &SlackAdapter{
		pipeline: notify.NewPipeline(notifier, slackTitles),
	}, nil
}

// NotifySuccess は処理が正常に完了したことを通知します。
func (s *SlackAdapter) NotifySuccess(ctx context.Context, outputURI, publicURL string, sourceCount int) error {
	if !s.pipeline.Enabled() {
		return nil
	}

	body := notify.NewBody().
		Link("出力先", publicURL, outputURI).
		Code("ソース数", strconv.Itoa(sourceCount)+" URLs").
		Code("ステータス", "Success")

	return s.pipeline.Success(ctx, body)
}

// NotifyFailure はエラーが発生したことを通知します。
func (s *SlackAdapter) NotifyFailure(ctx context.Context, err error) error {
	if !s.pipeline.Enabled() {
		return nil
	}

	return s.pipeline.Failure(ctx, nil, err)
}
