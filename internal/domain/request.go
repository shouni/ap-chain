package domain

// Request は Web 層で受け取り、Cloud Tasks でワーカーに渡す入力モデルです。
type Request struct {
	InputURI  string `json:"input_uri"`
	OutputURI string `json:"output_uri"`
}
