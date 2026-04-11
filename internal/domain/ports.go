package domain

import (
	"context"
)

// Pipeline は、処理を行うインターフェースです。
type Pipeline interface {
	// Execute は、すべての依存関係を構築し実行します。
	Execute(ctx context.Context, req Request) error
}

// Notifier は、処理の成功または失敗を通知する責務を定義します。
type Notifier interface {
	NotifySuccess(ctx context.Context, outputURI, publicURL string, sourceCount int) error
	NotifyFailure(ctx context.Context, err error) error
}
