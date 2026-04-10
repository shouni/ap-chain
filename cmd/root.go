package cmd

import (
	"github.com/shouni/clibase"
	"github.com/spf13/cobra"

	"ap-chain/internal/config"
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
func initAppPreRunE(cmd *cobra.Command, args []string) error {
	opts.FillDefaults(config.LoadConfig())

	return nil
}

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVarP(&opts.URL, "input", "i", "", "処理対象のURLリストファイル (必須)")
	rootCmd.PersistentFlags().StringVarP(&opts.Output, "output", "o", "./output/output.md", "出力先パス (local or gs://)")

	_ = rootCmd.MarkPersistentFlagRequired("input")
}
