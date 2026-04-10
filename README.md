# 🤖 AP Chain

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/ap-chain)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/ap-chain)](https://github.com/shouni/ap-chain/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🌟 概要: 高精度な情報収集とAI構造化のパイプライン

**AP Chain** は、大量のウェブページから情報を**並列・高速に取得**し、LLM（大規模言語モデル）の**マルチステップ処理**によって、情報の欠落や重複を排除した「高密度かつ論理的な構造化文書」を生成する、Go言語製の高機能CLIツールです。

単なる要約ツールではなく、**MapReduce型の処理フロー**を採用することで、長大なテキストでも重要な詳細を落とさず、情報の透明性（ソースURLの明示）を維持したまま、最終的なHTMLドキュメントへと変換します。

---

## 🛠️ 主要な機能と設計の特長

### LLMマルチステップ処理（MapReduce型）

巨大な情報を「分割・要約・統合」のステップで処理し、コンテキストウィンドウの制限を克服します。

* **Mapフェーズ**: セグメント化された各テキストから中間要約を並列生成。
* **Reduceフェーズ**: 中間情報を統合し、重複を排除。論理的なセクション（`##`）へと再構成します。
* **トレーサビリティ**: 各セクションの直後に、情報源となった**参照元URLリスト**を自動付与します。

---

## ✨ 技術スタック

| 要素 | 技術 / ライブラリ | 役割 |
| :--- | :--- | :--- |
| **言語** | **Go (Golang)** | ツールの開発言語。並列処理と堅牢な実行環境を提供します。 |
| **CLI** | **Cobra** | コマンドライン引数とオプションの解析に使用します。 |


## 🧱 基盤ライブラリ (Core Components)

AP Chain は以下の自作ライブラリ群を統合して構築されています：
* **[Go Web Reader](https://github.com/shouni/go-web-reader)**: マルチプロトコル I/O と本文抽出。
* **[Go Remote IO](https://github.com/shouni/go-remote-io)**: GCS/ローカルストレージの抽象化。
* **[Go Web Exact](https://github.com/shouni/go-web-exact)**: 高精度なメインコンテンツ抽出。

-----

## 🛠️ 事前準備と設定

Gemini API を利用するために、APIキーが必要です。設定は以下の**どちらか**の方法で行います。

```bash
# GCP プロジェクトID
export GCP_PROJECT_ID="YOUR_GCP_PROJECT_ID"
# Gemini API キー
export GEMINI_API_KEY="YOUR_GEMINI_API_KEY"
```

---

## 🚀 使い方 (Usage)

本ツールは、処理対象のURLを記載した**ファイル**を読み込む形式のみをサポートします。

### 実行コマンド形式とオプション

| オプション | フラグ | 説明 | デフォルト値 |
| :--- | :--- | :--- | :--- |
| `--input`  | `-i` | **処理対象のURLリストを記載したファイルパス**を指定します。ローカルパスまたは**GCS URI (`gs://...`)** を指定できます。 **(必須)** | なし |
| `--output` | `-o` | **最終的な構造化結果の出力先パス**を指定します。ローカルパスまたは**GCS URI (`gs://...`)** を指定できます。GCS URIを指定した場合、ローカルへの出力はスキップされます。 | `./output/output.md` |

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
