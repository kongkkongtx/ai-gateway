package cost

// Config defines cost control and quota management settings.
type Config struct {
	Enabled       bool          `yaml:"enabled"`
	DefaultLimit  QuotaLimit    `yaml:"default_limit"`
	KeyLimits     []KeyQuota    `yaml:"key_limits"`
	DegradeConfig DegradeConfig `yaml:"degrade"`
}

// QuotaLimit defines a token quota over a time window.
type QuotaLimit struct {
	InputTokens  int     `yaml:"input_tokens"`
	OutputTokens int     `yaml:"output_tokens"`
	Window       string  `yaml:"window"` 
}

// KeyQuota overrides the default quota for a specific API key.
type KeyQuota struct {
	KeyName string    `yaml:"key_name"`
	Limit   QuotaLimit `yaml:"limit"`
}

// DegradeConfig controls what happens when a quota is exceeded.
type DegradeConfig struct {
	Action          string  `yaml:"action"`     // "block" | "warn" | "degrade"
	CheaperProvider string  `yaml:"cheaper_provider"`
	AlertWebhook    string  `yaml:"alert_webhook"`
	AlertThreshold  float64 `yaml:"alert_threshold"`
}
