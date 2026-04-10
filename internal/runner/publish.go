package runner

import (
	"bytes"
	"context"
	"fmt"

	"github.com/shouni/go-remote-io/remoteio"
)

// mdRunner は Markdown 変換処理を実行するコアサービスを抽象化します。
type mdRunner interface {
	Run(title string, markdown []byte) (*bytes.Buffer, error)
}

// PublishRunner は、スクリプトの公開処理を実行する具象構造体です。
type PublishRunner struct {
	writer remoteio.Writer
	md     mdRunner
}

// NewPublisherRunner は PublishRunner の新しいインスタンスを作成します。
func NewPublisherRunner(writer remoteio.Writer, md mdRunner) *PublishRunner {
	return &PublishRunner{
		writer: writer,
		md:     md,
	}
}

// Run は公開処理のパイプライン全体を実行します。
func (p *PublishRunner) Run(ctx context.Context, storageURI, content string) error {
	const contentTypeHTML = "text/html; charset=utf-8"
	htmlReader, err := p.md.Run("", []byte(content))
	if err != nil {
		return err
	}
	if err := p.writer.Write(ctx, storageURI, htmlReader, contentTypeHTML); err != nil {
		return fmt.Errorf("リモートストレージへの書き込みに失敗しました: %w", err)
	}

	return nil
}
