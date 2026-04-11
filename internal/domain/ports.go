package domain

import (
	"context"
	"io"

	"github.com/shouni/go-web-exact/v2/ports"
)

// Pipeline は、処理を行うインターフェースです。
type Pipeline interface {
	// Execute は、すべての依存関係を構築し実行します。
	Execute(ctx context.Context) error
}

// FetchRunner は、指定されたソースからURLリストを取得し、スクレイピング処理を実行者に委譲します。
type FetchRunner interface {
	Run(ctx context.Context, sourceURI string) ([]ports.URLResult, error)
}

// CleanRunner は、URL結果のクリーンアップと構造化を実行する責務を持つインターフェースです。
type CleanRunner interface {
	Run(ctx context.Context, urls []ports.URLResult) (string, error)
}

// PublishRunner は、生成されたスクリプトの公開処理を実行する責務を持つインターフェースです。
type PublishRunner interface {
	Run(ctx context.Context, storageURI, content string) error
}

// ContentReader は、指定されたURIからコンテンツを取得するためのインターフェースです。
type ContentReader interface {
	Open(ctx context.Context, uri string) (io.ReadCloser, error)
}

// Cleaner は、URL結果のクリーンアップと構造化を実行する責務を持つインターフェースです。
type Cleaner interface {
	CleanAndStructureText(ctx context.Context, results []ports.URLResult) (string, error)
}

// PromptBuilder は、プロンプト文字列を生成する責務を定義します。
type PromptBuilder interface {
	GenerateMap(text, url string) (string, error)
	GenerateReduce(text string) (string, error)
}
