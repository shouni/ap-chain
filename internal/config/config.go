package config

import (
	"time"

	"github.com/shouni/go-utils/envutil"
)

const (
	DefaultHTTPTimeout    = 60 * time.Second
	MinInputContentLength = 10
	MaxInputSize          = 10 * 1024 * 1024
	// scraper
	defaultLScraperTimeout = 15 * time.Second
	defaultParallel        = 5

	// AI
	defaultMapModelName    = "gemini-3-flash-preview"
	defaultReduceModelName = "gemini-3-flash-preview"
	defaultLLMTimeout      = 5 * time.Minute
)

// Config はコマンドラインフラグを保持する構造体です。
type Config struct {
	InputSource  string
	OutputSource string

	HTTPTimeout        time.Duration
	ScraperTimeout     time.Duration
	MaxScraperParallel int

	MapModel    string
	ReduceModel string
	Concurrency int
	LLMTimeout  time.Duration

	ProjectID    string
	GeminiAPIKey string
	GCSBucket    string
}

// FillDefaults は、現在の設定で空のフィールドを envCfg の値で補完します。
func (c *Config) FillDefaults(envCfg *Config) {
	if c.ProjectID == "" {
		c.ProjectID = envCfg.ProjectID
	}
	if c.GeminiAPIKey == "" {
		c.GeminiAPIKey = envCfg.GeminiAPIKey
	}

	c.HTTPTimeout = DefaultHTTPTimeout
	c.ScraperTimeout = defaultLScraperTimeout

	c.Concurrency = 1
	c.LLMTimeout = defaultLLMTimeout
	c.MaxScraperParallel = defaultParallel
	c.MapModel = defaultMapModelName
	c.ReduceModel = defaultReduceModelName
}

// LoadConfig は環境変数から設定を読み込みます。
func LoadConfig() *Config {
	return &Config{
		ProjectID:    envutil.GetEnv("GCP_PROJECT_ID", ""),
		GeminiAPIKey: envutil.GetEnv("GEMINI_API_KEY", ""),
		GCSBucket:    envutil.GetEnv("GCS_BUCKET", ""),
	}
}
