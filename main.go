// Command ap-chain は、URL一覧を収集し Gemini の MapReduce で構造化文書へまとめる CLI です。
package main

import "ap-chain/cmd"

func main() {
	// cmdパッケージで定義されたルートコマンドを実行します
	cmd.Execute()
}
