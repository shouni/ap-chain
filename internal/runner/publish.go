package runner

import (
	"context"
	"fmt"
	"io"

	"github.com/shouni/go-remote-io/remoteio"

	"github.com/shouni/ap-chain/internal/config"
	"github.com/shouni/ap-chain/internal/domain"
)

// converter は、構造化レポートJSONをMarkdownへ変換する契約です。
type converter interface {
	Run(content []byte) (io.Reader, error)
}

// PublishRunner は、成果物の保存と署名付きURLの生成を担当します。
type PublishRunner struct {
	writer    remoteio.Writer
	signer    remoteio.URLSigner
	converter converter
}

// NewPublishRunner は PublishRunner の新しいインスタンスを作成します。
func NewPublishRunner(writer remoteio.Writer, signer remoteio.URLSigner, converter converter) *PublishRunner {
	return &PublishRunner{
		writer:    writer,
		signer:    signer,
		converter: converter,
	}
}

// Run は公開処理を実行し、署名付きURLを含む結果を返します。
// content は Reduce フェーズが生成した構造化レポートのJSON文字列です。
func (r *PublishRunner) Run(ctx context.Context, storageURI, content string) (*domain.PublishResult, error) {
	const contentTypeMD = "text/markdown; charset=utf-8"
	const defaultCacheControl = "public, max-age=1800"

	markdown, err := r.converter.Run([]byte(content))
	if err != nil {
		return nil, err
	}

	if err := r.writer.Write(ctx, storageURI, markdown,
		remoteio.WithContentType(contentTypeMD),
		remoteio.WithCacheControl(defaultCacheControl),
	); err != nil {
		return nil, fmt.Errorf("markdown write failed: %w", err)
	}

	signed, err := r.generateSignedResultURL(ctx, storageURI)
	if err != nil {
		return nil, fmt.Errorf("failed to sign markdown URL: %w", err)
	}

	return &domain.PublishResult{
		Markdown: domain.PublishedFile{
			StorageURI: storageURI,
			PublicURL:  signed,
		},
	}, nil
}

// generateSignedResultURL は StorageURI から署名付きURLを作るヘルパーです。
func (r *PublishRunner) generateSignedResultURL(ctx context.Context, storageURI string) (string, error) {
	// 署名器が設定されていない場合は、フォールバックとして元のURIを返す
	if r.signer == nil {
		return storageURI, nil
	}
	// 有効なGETリクエスト用URLを生成
	return r.signer.GenerateSignedURL(ctx, storageURI, "GET", config.DefaultSignedURLExpiration)
}
