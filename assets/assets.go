// Package assets は、埋め込みプロンプトテンプレートを提供します。
package assets

import (
	"embed"

	"github.com/shouni/go-prompt-kit/resource"
)

const (
	promptDir    = "prompts"
	promptPrefix = "prompt_"
)

// PromptFiles は、Map/Reduce 各フェーズのプロンプトテンプレートを保持します。
//
//go:embed prompts/prompt_*.md
var PromptFiles embed.FS

// LoadPrompts は埋め込まれたプロンプトファイルを読み込みます。
func LoadPrompts() (map[string]string, error) {
	return resource.Load(PromptFiles, promptDir, promptPrefix)
}
