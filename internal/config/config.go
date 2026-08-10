// Package config は、コマンドラインフラグと環境変数から設定を組み立てます。
// 既定値はこのパッケージが単一の出所で、利用側で再定義しません。
package config

import (
	"errors"
	"strings"
	"time"

	"github.com/shouni/go-utils/envutil"
)

// パイプライン全体で共有する既定値です。個々のアダプタで再定義せず、ここを参照します。
const (
	DefaultHTTPTimeout         = 60 * time.Second
	MinInputContentLength      = 10
	MaxInputSize               = 10 * 1024 * 1024
	DefaultSignedURLExpiration = 10 * time.Minute
	// scraper
	defaultParallel = 5

	// DefaultMaxConcurrency は、LLM呼び出しのデフォルト最大並列数です。
	DefaultMaxConcurrency = 1
	// DefaultRateInterval は、LLM呼び出しのデフォルトレート制限間隔です。
	DefaultRateInterval = 10 * time.Second
)

// Config はコマンドラインフラグを保持する構造体です。
type Config struct {
	InputFile  string
	OutputFile string

	HTTPTimeout        time.Duration
	MaxScraperParallel int

	ProjectID    string
	GeminiAPIKey string

	// GeminiModel は Map / Reduce の両フェーズで使うモデル名です。既定値は持たず、
	// --model か GEMINI_MODEL で必ず指定させます。モデル ID が古くなるのは Google の
	// リリース周期であってこのリポジトリの都合ではないため、既定値を置くと
	// 誰も気付かないまま古いモデルを使い続けることになります。
	GeminiModel string

	MaxConcurrency  int
	RateInterval    time.Duration
	SlackWebhookURL string
}

// Normalize は設定値の文字列フィールドから前後の空白を一括で削除します。
func (c *Config) Normalize() {
	if c == nil {
		return
	}
	c.InputFile = strings.TrimSpace(c.InputFile)
	c.OutputFile = strings.TrimSpace(c.OutputFile)
	c.GeminiModel = strings.TrimSpace(c.GeminiModel)
	c.ProjectID = strings.TrimSpace(c.ProjectID)
	c.GeminiAPIKey = strings.TrimSpace(c.GeminiAPIKey)
	c.SlackWebhookURL = strings.TrimSpace(c.SlackWebhookURL)
}

// FillDefaults は、現在の設定で空のフィールドを envCfg の値で補完します。
func (c *Config) FillDefaults(envCfg *Config) {
	if c.ProjectID == "" {
		c.ProjectID = envCfg.ProjectID
	}
	if c.GeminiAPIKey == "" {
		c.GeminiAPIKey = envCfg.GeminiAPIKey
	}
	if c.GeminiModel == "" {
		c.GeminiModel = envCfg.GeminiModel
	}

	if c.SlackWebhookURL == "" {
		c.SlackWebhookURL = envCfg.SlackWebhookURL
	}
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = envCfg.MaxConcurrency
	}
	if c.RateInterval <= 0 {
		c.RateInterval = envCfg.RateInterval
	}

	c.HTTPTimeout = DefaultHTTPTimeout
	c.MaxScraperParallel = defaultParallel
}

// Validate は実行に最低限必要な設定が揃っているかを確認します。
// フラグと環境変数の両方を見たあと（FillDefaults の後）に呼びます。
func (c *Config) Validate() error {
	if c.GeminiModel == "" {
		return errors.New("モデル名が指定されていません。--model / -g フラグか GEMINI_MODEL 環境変数で指定してください")
	}
	return nil
}

// LoadConfig は環境変数から設定を読み込みます。
func LoadConfig() *Config {
	return &Config{
		ProjectID:       envutil.GetEnv("GCP_PROJECT_ID", ""),
		GeminiAPIKey:    envutil.GetEnv("GEMINI_API_KEY", ""),
		SlackWebhookURL: envutil.GetEnv("SLACK_WEBHOOK_URL", ""),
		GeminiModel:     envutil.GetEnv("GEMINI_MODEL", ""),
		MaxConcurrency:  envutil.GetEnvAsInt("MAX_CONCURRENCY", DefaultMaxConcurrency),
		RateInterval:    time.Duration(envutil.GetEnvAsInt("RATE_INTERVAL_SEC", int(DefaultRateInterval/time.Second))) * time.Second,
	}
}
