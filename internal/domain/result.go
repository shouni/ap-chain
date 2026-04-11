package domain

// URLResult は、特定のURLから抽出された結果を保持します。
type URLResult struct {
	URL     string // 処理対象のURL
	Content string // 抽出された記事の本文
}

// Segment は、URLから抽出されたセグメントを表します
type Segment struct {
	Text string
	URL  string
}

// PublishedFile は公開されたファイルの情報です
type PublishedFile struct {
	StorageURI string
	PublicURL  string
}

// PublishResult は公開処理の最終結果をまとめます
type PublishResult struct {
	Markdown PublishedFile
	HTML     PublishedFile
}
