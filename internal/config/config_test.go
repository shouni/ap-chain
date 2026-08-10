package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfig_Normalize(t *testing.T) {
	t.Run("文字列フィールドの前後空白が落ちる", func(t *testing.T) {
		c := &Config{
			InputFile:       "  urls.txt \n",
			OutputFile:      "\tout.md ",
			GeminiModel:     " model ",
			ProjectID:       " proj ",
			GeminiAPIKey:    " key ",
			SlackWebhookURL: " https://hooks.example.com/x ",
		}
		c.Normalize()

		tests := map[string]struct{ got, want string }{
			"InputFile":       {c.InputFile, "urls.txt"},
			"OutputFile":      {c.OutputFile, "out.md"},
			"GeminiModel":     {c.GeminiModel, "model"},
			"ProjectID":       {c.ProjectID, "proj"},
			"GeminiAPIKey":    {c.GeminiAPIKey, "key"},
			"SlackWebhookURL": {c.SlackWebhookURL, "https://hooks.example.com/x"},
		}
		for name, tt := range tests {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", name, tt.got, tt.want)
			}
		}
	})

	t.Run("空白のみのフィールドは空になる", func(t *testing.T) {
		c := &Config{GeminiAPIKey: "   "}
		c.Normalize()
		if c.GeminiAPIKey != "" {
			t.Errorf("GeminiAPIKey = %q, want empty", c.GeminiAPIKey)
		}
	})

	t.Run("nilレシーバでも落ちない", func(_ *testing.T) {
		var c *Config
		c.Normalize()
	})
}

func TestConfig_FillDefaults(t *testing.T) {
	env := &Config{
		ProjectID:       "env-proj",
		GeminiAPIKey:    "env-key",
		GeminiModel:     "env-model",
		SlackWebhookURL: "env-hook",
		MaxConcurrency:  4,
		RateInterval:    5 * time.Second,
	}

	t.Run("空のフィールドだけが環境設定で補完される", func(t *testing.T) {
		c := &Config{GeminiModel: "flag-model", MaxConcurrency: 9}
		c.FillDefaults(env)

		// 明示指定されたものは維持される
		if c.GeminiModel != "flag-model" {
			t.Errorf("GeminiModel = %q, want flag-model（フラグ指定が上書きされています）", c.GeminiModel)
		}
		if c.MaxConcurrency != 9 {
			t.Errorf("MaxConcurrency = %d, want 9", c.MaxConcurrency)
		}
		// 空のものは環境設定で埋まる
		if c.ProjectID != "env-proj" || c.SlackWebhookURL != "env-hook" {
			t.Errorf("環境設定で補完されていません: %+v", c)
		}
		if c.RateInterval != 5*time.Second {
			t.Errorf("RateInterval = %v, want 5s", c.RateInterval)
		}
	})

	t.Run("モデル名が空なら環境設定で補完される", func(t *testing.T) {
		c := &Config{}
		c.FillDefaults(env)

		if c.GeminiModel != "env-model" {
			t.Errorf("GeminiModel = %q, want env-model", c.GeminiModel)
		}
	})

	t.Run("0以下の数値は未指定として補完される", func(t *testing.T) {
		c := &Config{MaxConcurrency: 0, RateInterval: 0}
		c.FillDefaults(env)

		if c.MaxConcurrency != 4 {
			t.Errorf("MaxConcurrency = %d, want 4", c.MaxConcurrency)
		}
		if c.RateInterval != 5*time.Second {
			t.Errorf("RateInterval = %v, want 5s", c.RateInterval)
		}

		negative := &Config{MaxConcurrency: -1, RateInterval: -time.Second}
		negative.FillDefaults(env)
		if negative.MaxConcurrency != 4 || negative.RateInterval != 5*time.Second {
			t.Errorf("負値が補完されていません: %+v", negative)
		}
	})

	// HTTPTimeout と MaxScraperParallel は環境変数やフラグで変えられず、
	// 常に定数へ揃えられます。可変にする場合はこのテストも変える必要があります。
	t.Run("HTTPTimeoutとMaxScraperParallelは常に定数へ揃う", func(t *testing.T) {
		c := &Config{HTTPTimeout: time.Hour, MaxScraperParallel: 999}
		c.FillDefaults(env)

		if c.HTTPTimeout != DefaultHTTPTimeout {
			t.Errorf("HTTPTimeout = %v, want %v", c.HTTPTimeout, DefaultHTTPTimeout)
		}
		if c.MaxScraperParallel != defaultParallel {
			t.Errorf("MaxScraperParallel = %d, want %d", c.MaxScraperParallel, defaultParallel)
		}
	})
}

func TestLoadConfig(t *testing.T) {
	// 既定値へ黙って落ちると、古いモデルを使い続けたまま気付けません。
	t.Run("モデル名が未設定なら Validate が落とす", func(t *testing.T) {
		for _, key := range []string{
			"GCP_PROJECT_ID", "GEMINI_API_KEY", "SLACK_WEBHOOK_URL",
			"GEMINI_MODEL", "MAX_CONCURRENCY", "RATE_INTERVAL_SEC",
		} {
			t.Setenv(key, "")
		}

		c := &Config{}
		c.FillDefaults(LoadConfig())
		c.Normalize()

		err := c.Validate()
		if err == nil {
			t.Fatal("モデル名未指定が素通りした")
		}
		if !strings.Contains(err.Error(), "GEMINI_MODEL") {
			t.Errorf("エラーに指定方法が無い: %v", err)
		}
	})

	t.Run("環境変数が設定されていれば反映される", func(t *testing.T) {
		t.Setenv("GCP_PROJECT_ID", "my-proj")
		t.Setenv("GEMINI_MODEL", "my-model")
		t.Setenv("MAX_CONCURRENCY", "8")
		t.Setenv("RATE_INTERVAL_SEC", "3")

		c := LoadConfig()
		if c.ProjectID != "my-proj" {
			t.Errorf("ProjectID = %q, want my-proj", c.ProjectID)
		}
		if c.GeminiModel != "my-model" {
			t.Errorf("GeminiModel = %q, want my-model", c.GeminiModel)
		}
		if c.MaxConcurrency != 8 {
			t.Errorf("MaxConcurrency = %d, want 8", c.MaxConcurrency)
		}
		// 秒指定が time.Duration へ変換される
		if c.RateInterval != 3*time.Second {
			t.Errorf("RateInterval = %v, want 3s", c.RateInterval)
		}
	})
}
