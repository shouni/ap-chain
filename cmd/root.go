package cmd

import (
	"github.com/shouni/clibase"
	"github.com/spf13/cobra"

	"github.com/shouni/ap-chain/internal/config"
)

const (
	appName = "ap-chain"
)

// opts は、実行のパラメータです
var opts config.Config

// Execute は、アプリケーションのエントリポイントです。
func Execute() {
	clibase.Execute(clibase.App{
		Name:     appName,
		AddFlags: addAppPersistentFlags,
		PreRunE:  initAppPreRunE,
		Commands: []*cobra.Command{
			generateCmd,
		},
	})
}

// --- アプリケーション固有のカスタム関数 ---

// initAppPreRunE は、共通処理の後に実行される初期化ロジックです。
func initAppPreRunE(_ *cobra.Command, _ []string) error {
	opts.FillDefaults(config.LoadConfig())
	opts.Normalize()

	return opts.Validate()
}

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVarP(&opts.InputFile, "input", "i", "", "処理対象のURLリストファイル (必須)")
	rootCmd.PersistentFlags().StringVarP(&opts.OutputFile, "output", "o", "./output/output.md", "出力ファイル (local or gs://)")
	rootCmd.PersistentFlags().StringVarP(&opts.GeminiModel, "model", "g", "", "Map / Reduce で使用する Google Gemini モデル名（必須。環境変数 GEMINI_MODEL でも指定できます）")

	_ = rootCmd.MarkPersistentFlagRequired("input")
}
