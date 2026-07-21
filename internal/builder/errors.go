package builder

import "fmt"

// wrapInitErr は、コンポーネント名を付与した初期化失敗エラーを生成します。
func wrapInitErr(name string, err error) error {
	return fmt.Errorf("%sの初期化に失敗しました: %w", name, err)
}
