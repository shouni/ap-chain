package config

import (
	"strings"
	"time"

	"github.com/shouni/go-utils/envutil"
)

const (
	DefaultHTTPTimeout         = 60 * time.Second
	MinInputContentLength      = 10
	MaxInputSize               = 10 * 1024 * 1024
	DefaultSignedURLExpiration = 10 * time.Minute
	// scraper
	defaultLScraperTimeout = 15 * time.Second
	defaultParallel        = 5

	// AI
	defaultMapModelName    = "gemini-3-flash-preview"
	defaultReduceModelName = "gemini-3-flash-preview"
	defaultLLMTimeout      = 5 * time.Minute
	defaultLMaxConcurrency = 1
)

// Config はコマンドラインフラグを保持する構造体です。
type Config struct {
	InputFile  string
	OutputFile string

	HTTPTimeout        time.Duration
	ScraperTimeout     time.Duration
	MaxScraperParallel int

	ProjectID       string
	GeminiAPIKey    string
	MapModel        string
	ReduceModel     string
	Concurrency     int
	LLMTimeout      time.Duration
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
	if c.SlackWebhookURL == "" {
		c.SlackWebhookURL = envCfg.SlackWebhookURL
	}
	if c.Concurrency == 0 {
		c.Concurrency = envCfg.Concurrency
	}

	c.HTTPTimeout = DefaultHTTPTimeout
	c.ScraperTimeout = defaultLScraperTimeout

	c.LLMTimeout = defaultLLMTimeout
	c.MaxScraperParallel = defaultParallel
	c.MapModel = defaultMapModelName
	c.ReduceModel = defaultReduceModelName
}

// LoadConfig は環境変数から設定を読み込みます。
func LoadConfig() *Config {
	return &Config{
		ProjectID:       envutil.GetEnv("GCP_PROJECT_ID", ""),
		GeminiAPIKey:    envutil.GetEnv("GEMINI_API_KEY", ""),
		SlackWebhookURL: envutil.GetEnv("SLACK_WEBHOOK_URL", ""),
		Concurrency:     envutil.GetEnvAsInt("MAX_CONCURRENCY", defaultLMaxConcurrency),
	}
}
