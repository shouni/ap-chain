package domain

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
