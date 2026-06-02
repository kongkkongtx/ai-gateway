// Package security provides prompt injection detection, PII redaction,
// and content security utilities for the AI Gateway.
package security

// Config defines security-related settings for prompt injection detection
// and PII redaction. Embedded in the main gateway configuration.
type Config struct {
	PromptInjection PromptInjectionConfig `yaml:"prompt_injection"`
	PII             PIIConfig             `yaml:"pii"`
	IPAllowlist     []string              `yaml:"ip_allowlist,omitempty" json:"ip_allowlist,omitempty"`
	IPBlocklist     []string              `yaml:"ip_blocklist,omitempty" json:"ip_blocklist,omitempty"`
}

// PromptInjectionConfig controls prompt injection detection behavior.
type PromptInjectionConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Action      string   `yaml:"action"`        // "block" | "log" | "sanitize"
	RiskThresh  string   `yaml:"risk_threshold"` // "low" | "medium" | "high" | "critical"
	Keywords    []string `yaml:"keywords,omitempty"`
	ExternalURL string   `yaml:"external_url,omitempty"`
}

// PIIConfig controls personally identifiable information redaction.
type PIIConfig struct {
	Enabled bool     `yaml:"enabled"`
	Action  string   `yaml:"action"`
	Types   []string `yaml:"types,omitempty"`
}

// DefaultConfig returns a disabled-by-default security config.
func DefaultConfig() Config {
	return Config{
		PromptInjection: PromptInjectionConfig{
			Enabled:    false,
			Action:     "block",
			RiskThresh: "medium",
		},
		PII: PIIConfig{
			Enabled: false,
			Action:  "mask",
		},
	}
}

// InjectionResult describes a detected prompt injection attempt.
type InjectionResult struct {
	Detected bool   `json:"detected"`
	Risk     string `json:"risk"`
	Pattern  string `json:"pattern"`
	Message  string `json:"message"`
}

// PIIMatch describes a single detected PII instance.
type PIIMatch struct {
	Type     string `json:"type"`
	Original string `json:"original"`
	Redacted string `json:"redacted"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
}
