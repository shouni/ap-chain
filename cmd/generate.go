package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"ap-chain/internal/builder"
)

// generateCmd は、メインのCLIコマンド定義です。
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Webコンテンツの取得とAIクリーンアップを実行します。",
	Long: `
Webコンテンツの取得とAIクリーンアップを実行します。
`,
	RunE: generateCommand,
}

// generateCommand は、入力ソースからLLMマルチステップを実行し、指定されたURIのクラウドストレージにWアップロード
func generateCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	appCtx, err := builder.BuildContainer(ctx, &opts)
	if err != nil {
		// コンテナの構築エラーをラップして返す
		return fmt.Errorf("コンテナの構築に失敗しました: %w", err)
	}
	defer func() {
		if closeErr := appCtx.Close(); closeErr != nil {
			slog.ErrorContext(ctx, "コンテナのクローズに失敗しました", "error", closeErr)
		}
	}()

	err = appCtx.Pipeline.Execute(ctx)
	if err != nil {
		return err
	}

	return nil
}
