package adapters

import (
	"context"
	"fmt"
	"strings"

	"github.com/shouni/go-http-kit/httpkit"
	"github.com/shouni/go-notifier/pkg/slack"
)

// SlackAdapter は Slack への通知を担当します。
type SlackAdapter struct {
	slackClient *slack.Client
	webhookURL  string
}

// NewSlackAdapter は新しい Slack アダプターを初期化します。
func NewSlackAdapter(httpClient httpkit.Requester, webhookURL string) (*SlackAdapter, error) {
	if webhookURL == "" {
		return &SlackAdapter{}, nil
	}

	client, err := slack.NewClient(httpClient, webhookURL)
	if err != nil {
		return nil, fmt.Errorf("Slackクライアントの初期化に失敗しました: %w", err)
	}

	return &SlackAdapter{
		slackClient: client,
		webhookURL:  webhookURL,
	}, nil
}

// NotifySuccess は処理が正常に完了したことを通知します。
func (s *SlackAdapter) NotifySuccess(ctx context.Context, outputURI, publicURL string, sourceCount int) error {
	if s.webhookURL == "" || s.slackClient == nil {
		return nil
	}

	title := "✅ AP Chain: 構造化ドキュメントの生成が完了しました"
	content := fmt.Sprintf(
		"*出力先:* <%s|%s>\n"+
			"*ソース数:* `%d` URLs\n"+
			"*ステータス:* `Success`",
		publicURL,
		outputURI,
		sourceCount,
	)

	return s.slackClient.SendTextWithHeader(ctx, title, strings.TrimSpace(content))
}

// NotifyFailure はエラーが発生したことを通知します。
func (s *SlackAdapter) NotifyFailure(ctx context.Context, err error) error {
	if s.webhookURL == "" || s.slackClient == nil {
		return nil
	}

	title := "❌ AP Chain: 実行エラーが発生しました"
	content := fmt.Sprintf("*エラー詳細:* ```%v```", err)

	return s.slackClient.SendTextWithHeader(ctx, title, content)
}
