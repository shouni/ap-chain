package runner

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/shouni/go-remote-io/remoteio"

	"ap-chain/internal/domain"
)

const defaultSignedURLExpiration = 10 * time.Minute

// mdRunner は Markdown 変換処理を実行するコアサービスを抽象化します。
type mdRunner interface {
	Run(title string, markdown []byte) (*bytes.Buffer, error)
}

// PublishRunner は、成果物の保存と署名付きURLの生成を担当します。
type PublishRunner struct {
	writer remoteio.Writer
	signer remoteio.URLSigner
	md     mdRunner
}

// NewPublisherRunner は PublishRunner の新しいインスタンスを作成します。
func NewPublisherRunner(writer remoteio.Writer, signer remoteio.URLSigner, md mdRunner) *PublishRunner {
	return &PublishRunner{
		writer: writer,
		signer: signer,
		md:     md,
	}
}

// Run は公開処理を実行し、署名付きURLを含む結果を返します。
func (r *PublishRunner) Run(ctx context.Context, storageURI, content string) (*domain.PublishResult, error) {
	const contentTypeHTML = "text/html; charset=utf-8"
	const contentTypeMD = "text/markdown; charset=utf-8"

	// 1. Markdown 保存
	if err := r.writer.Write(ctx, storageURI, strings.NewReader(content), contentTypeMD); err != nil {
		return nil, fmt.Errorf("markdown write failed: %w", err)
	}

	// 2. HTML 変換・保存
	htmlReader, err := r.md.Run("", []byte(content))
	if err != nil {
		return nil, err
	}
	htmlURI := r.replaceExt(storageURI, ".html")
	if err := r.writer.Write(ctx, htmlURI, htmlReader, contentTypeHTML); err != nil {
		return nil, fmt.Errorf("html write failed: %w", err)
	}

	// 3. 署名付きURLの生成
	mdSigned, err := r.generateSignedResultURL(ctx, storageURI)
	if err != nil {
		return nil, fmt.Errorf("failed to sign markdown URL: %w", err)
	}
	htmlSigned, err := r.generateSignedResultURL(ctx, htmlURI)
	if err != nil {
		return nil, fmt.Errorf("failed to sign html URL: %w", err)
	}

	// 結果を返却
	return &domain.PublishResult{
		Markdown: domain.PublishedFile{
			StorageURI: storageURI,
			PublicURL:  mdSigned,
		},
		HTML: domain.PublishedFile{
			StorageURI: htmlURI,
			PublicURL:  htmlSigned,
		},
	}, nil
}

// replaceExt は URI の拡張子を新しいものに差し替えます
func (r *PublishRunner) replaceExt(uri, newExt string) string {
	ext := path.Ext(uri)
	return strings.TrimSuffix(uri, ext) + newExt
}

// generateSignedResultURL は StorageURI から署名付きURLを作るヘルパーです。
func (r *PublishRunner) generateSignedResultURL(ctx context.Context, storageURI string) (string, error) {
	// 署名器が設定されていない場合は、フォールバックとして元のURIを返す
	if r.signer == nil {
		return storageURI, nil
	}
	// 有効なGETリクエスト用URLを生成
	return r.signer.GenerateSignedURL(ctx, storageURI, "GET", defaultSignedURLExpiration)
}
