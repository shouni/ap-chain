package domain

// URLResult は、特定のURLから抽出された結果を保持します。
type URLResult struct {
	URL     string // 処理対象のURL
	Content string // 抽出された記事の本文
}

// Segment は、URLから抽出されたセグメントを表します。
// Map フェーズの入出力・Reduce フェーズの入力として共通で使い回します
// (Map出力はAIがクリーンアップした Text と、Go側が保持する URL の組)。
type Segment struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// PublishedFile は公開されたファイルの情報です
type PublishedFile struct {
	StorageURI string
	PublicURL  string
}

// PublishResult は公開処理の最終結果をまとめます
type PublishResult struct {
	Markdown PublishedFile
}
