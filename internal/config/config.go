// Package config は、コマンドラインフラグと環境変数から設定を組み立てます。
// 既定値はこのパッケージが単一の出所で、利用側で再定義しません。
package config

import (
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

	// AI
	defaultMapModelName    = "gemini-3-flash-preview"
	defaultReduceModelName = "gemini-3-flash-preview"

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

	ProjectID       string
	GeminiAPIKey    string
	MapModel        string
	ReduceModel     string
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
	c.MapModel = strings.TrimSpace(c.MapModel)
	c.ReduceModel = strings.TrimSpace(c.ReduceModel)
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
	if c.MapModel == "" {
		c.MapModel = envCfg.MapModel
	}
	if c.ReduceModel == "" {
		c.ReduceModel = envCfg.ReduceModel
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

// LoadConfig は環境変数から設定を読み込みます。
func LoadConfig() *Config {
	return &Config{
		ProjectID:       envutil.GetEnv("GCP_PROJECT_ID", ""),
		GeminiAPIKey:    envutil.GetEnv("GEMINI_API_KEY", ""),
		SlackWebhookURL: envutil.GetEnv("SLACK_WEBHOOK_URL", ""),
		MapModel:        envNonEmpty("GEMINI_MODEL", defaultMapModelName),
		ReduceModel:     envNonEmpty("GEMINI_QUALITY_MODEL", defaultReduceModelName),
		MaxConcurrency:  envutil.GetEnvAsInt("MAX_CONCURRENCY", DefaultMaxConcurrency),
		RateInterval:    time.Duration(envutil.GetEnvAsInt("RATE_INTERVAL_SEC", int(DefaultRateInterval/time.Second))) * time.Second,
	}
}

// envNonEmpty は、空文字で設定された環境変数を未設定として扱い既定値を返します。
//
// envutil.GetEnv は os.LookupEnv を使うため「設定されているが空」でもその空文字を返します。
// cloudbuild.yaml はモデル名を無条件に環境変数へ渡すので、置換が空のまま起動すると
// 既定値が効かず、モデル名が空のまま実行しようとしてしまいます。
func envNonEmpty(key, defaultValue string) string {
	if v := strings.TrimSpace(envutil.GetEnv(key, "")); v != "" {
		return v
	}
	return defaultValue
}
