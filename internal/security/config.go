// Package security provides prompt injection detection, PII redaction,
// and content security utilities for the AI Gateway.
package security

// Config defines security-related settings for prompt injection detection
// and PII redaction. Embedded in the main gateway configuration.
type Config struct {
	PromptInjection PromptInjectionConfig `yaml:"prompt_injection"`
	PII             PIIConfig             `yaml:"pii"`
}

// PromptInjectionConfig controls prompt injection detection behavior.
type PromptInjectionConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Action      string   `yaml:"action"`       // "block" | "log" | "sanitize"
	RiskThresh  string   `yaml:"risk_threshold"` // "low" | "medium" | "high" | "critical"
	Keywords    []string `yaml:"keywords,omitempty"`  // Additional custom keywords
	ExternalURL string   `yaml:"external_url,omitempty"` // Optional external detection API
}

// PIIConfig controls personally identifiable information redaction.
type PIIConfig struct {
	Enabled bool     `yaml:"enabled"`
	Action  string   `yaml:"action"`  // "mask" | "hash" | "block"
	Types   []string `yaml:"types,omitempty"` // Subset of: email, phone, ip, api_key, ssn, credit_card
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
	Risk     string `json:"risk"`     // "low", "medium", "high", "critical"
	Pattern  string `json:"pattern"`  // Name of the matched pattern
	Message  string `json:"message"`  // Human-readable description
}

// PIIMatch describes a single detected PII instance.
type PIIMatch struct {
	Type     string `json:"type"`     // "email", "phone", "ip", etc.
	Original string `json:"original"` // The original sensitive text
	Redacted string `json:"redacted"` // The replacement text
	Start    int    `json:"start"`    // Start position in the text
	End      int    `json:"end"`      // End position in the text
}