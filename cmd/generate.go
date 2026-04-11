package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"ap-chain/internal/builder"
	"ap-chain/internal/domain"
)

// generateCmd は、メインのCLIコマンド定義です。
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "複数URLからの情報収集とAIによる構造化レポートの生成",
	Long: `入力されたURLリストからコンテンツを収集し、LLMパイプラインで構造化文書を生成します。
ワークフロー:
- Collector: Webコンテンツの並列スクレイピングとメインテキストの抽出
- Composer: MapReduce処理による情報の統合・重複排除・ソース明示
- Publisher: Markdown/HTML保存、署名付きURL発行`,
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

	req := domain.Request{
		InputURI:  opts.InputFile,
		OutputURI: opts.OutputFile,
	}
	err = appCtx.Pipeline.Execute(ctx, req)
	if err != nil {
		return err
	}

	return nil
}
