package runner

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

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
func (r *PublishRunner) Run(ctx context.Context, storageURI, content string) error {
	const contentTypeHTML = "text/html; charset=utf-8"
	const contentTypeMD = "text/markdown; charset=utf-8"

	// 1. Markdown 形式でそのまま保存
	mdReader := strings.NewReader(content)
	if err := r.writer.Write(ctx, storageURI, mdReader, contentTypeMD); err != nil {
		return fmt.Errorf("Markdownのリモート書き込みに失敗しました: %w", err)
	}

	// 2. HTML への変換
	htmlReader, err := r.md.Run("", []byte(content))
	if err != nil {
		return err
	}

	// 3. 拡張子を .html に変更した URI を生成
	// 例: gs://bucket/report.md -> gs://bucket/report.html
	htmlURI := r.replaceExt(storageURI, ".html")

	// 4. HTML を保存
	if err := r.writer.Write(ctx, htmlURI, htmlReader, contentTypeHTML); err != nil {
		return fmt.Errorf("HTMLのリモート書き込みに失敗しました: %w", err)
	}

	return nil
}

// replaceExt は URI の拡張子を新しいものに差し替えます
func (r *PublishRunner) replaceExt(uri, newExt string) string {
	ext := path.Ext(uri)
	return strings.TrimSuffix(uri, ext) + newExt
}
