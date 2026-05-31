package security

import (
	"fmt"
	"log/slog"
	"strings"
)

// InjectionDetector defines the interface for prompt injection detection.
// Implementations can use built-in rules, call external APIs, or both.
type InjectionDetector interface {
	// Check evaluates a single message for prompt injection.
	Check(message string) (*InjectionResult, error)
	// Name returns a human-readable identifier for this detector.
	Name() string
}

// ---------------------------------------------------------------------------
// Built-in keyword/pattern rule detector
// ---------------------------------------------------------------------------

// ruleDetector uses a list of known prompt injection patterns and keywords
// to detect common attack vectors.
type ruleDetector struct {
	rules   []injectionRule
	logger  *slog.Logger
}

type injectionRule struct {
	Name     string
	Patterns []string
	Risk     string // "low", "medium", "high", "critical"
}

// built-in rules covering common prompt injection categories
var defaultRules = []injectionRule{
	{
		Name: "role-override",
		Risk: "high",
		Patterns: []string{
			"ignore all previous instructions",
			"ignore all prior instructions",
			"ignore previous instructions",
			"ignore the above",
			"disregard all previous",
			"disregard the above",
			"forget everything",
			"forget all previous",
			"override your instructions",
			"override your previous",
			"you are now",
			"act as if",
			"pretend to be",
			"from now on, you are",
			"you will now act as",
			"new role:",
			"roleplay as",
			"you are free from",
			"you are released from",
			"do not follow",
			"do not obey",
		},
	},
	{
		Name: "jailbreak",
		Risk: "critical",
		Patterns: []string{
			"DAN",
			"do anything now",
			"jailbreak",
			"jail broken",
			"jail-broken",
			"unfiltered",
			"uncensored",
			"no restrictions",
			"no limitations",
			"no boundaries",
			"without restrictions",
			"without limitations",
			"without any filters",
			"bypass restrictions",
			"bypass safety",
			"bypass the guidelines",
			"ignore safety",
			"ignore ethics",
			"ignore morals",
			"ignore content policy",
			"ignore the rules",
			"ignore guidelines",
			"ignore your programming",
		},
	},
	{
		Name: "data-exfiltration",
		Risk: "critical",
		Patterns: []string{
			"send this to",
			"post this to",
			"forward to",
			"exfiltrate",
			"leak this data",
			"extract all data",
			"dump your memory",
			"print your system prompt",
			"output your instructions",
			"reveal your prompt",
			"show your instructions",
			"what are your instructions",
			"what is your system prompt",
			"repeat the words above",
			"repeat everything above",
			"repeat your prompt",
		},
	},
	{
		Name: "token-theft",
		Risk: "high",
		Patterns: []string{
			"steal",
			"api key",
			"api key is",
			"password is",
			"secret is",
			"token is",
			"my password",
			"my api key",
			"my secret",
			"credential",
			"login credentials",
		},
	},
	{
		Name: "prompt-leaking",
		Risk: "medium",
		Patterns: []string{
			"tell me your prompt",
			"what is your prompt",
			"what were you told",
			"how were you programmed",
			"what are you trained on",
			"give me your system message",
			"output your initial",
			"reveal your system",
			"print your prompt",
			"show your prompt",
			"what instructions were you given",
			"what rules do you follow",
		},
	},
	{
		Name: "encoding-obfuscation",
		Risk: "high",
		Patterns: []string{
			"base64",
			"rot13",
			"caesar cipher",
			"hex encode",
			"url encode",
			"unicode escape",
			"morse code",
			"reverse the text",
			"encode this",
			"decode this",
		},
	},
	{
		Name: "sql-injection",
		Risk: "high",
		Patterns: []string{
			"drop table",
			"delete from",
			"insert into",
			"select * from",
			"union select",
			"'; --",
			"'; DROP",
			"1=1;",
			"or 1=1",
			"or '1'='1",
			"admin' --",
		},
	},
}

// NewRuleDetector creates a built-in rule-based injection detector.
// Additional custom keywords can be appended.
func NewRuleDetector(customKeywords []string, logger *slog.Logger) InjectionDetector {
	rules := make([]injectionRule, len(defaultRules))
	copy(rules, defaultRules)

	// Add custom keywords as a separate rule
	if len(customKeywords) > 0 {
		rules = append(rules, injectionRule{
			Name:     "custom-keywords",
			Risk:     "medium",
			Patterns: customKeywords,
		})
	}

	return &ruleDetector{rules: rules, logger: logger}
}

func (d *ruleDetector) Name() string { return "built-in-rules" }

func (d *ruleDetector) Check(message string) (*InjectionResult, error) {
	msgLower := strings.ToLower(message)

	for _, rule := range d.rules {
		for _, pattern := range rule.Patterns {
			if strings.Contains(msgLower, pattern) {
				d.logger.Debug("prompt injection pattern matched",
					"rule", rule.Name,
					"pattern", pattern,
					"risk", rule.Risk)
				return &InjectionResult{
					Detected: true,
					Risk:     rule.Risk,
					Pattern:  fmt.Sprintf("%s:%s", rule.Name, pattern),
					Message:  fmt.Sprintf("Prompt injection detected: %s (risk: %s)", rule.Name, rule.Risk),
				}, nil
			}
		}
	}

	return &InjectionResult{Detected: false}, nil
}

// ---------------------------------------------------------------------------
// Composite detector — runs multiple detectors in sequence
// ---------------------------------------------------------------------------

// multiDetector runs multiple checkers and returns the highest-risk result.
type multiDetector struct {
	detectors []InjectionDetector
}

// NewMultiDetector creates a composite detector that runs all sub-detectors.
func NewMultiDetector(detectors ...InjectionDetector) InjectionDetector {
	return &multiDetector{detectors: detectors}
}

func (m *multiDetector) Name() string { return "composite" }

func (m *multiDetector) Check(message string) (*InjectionResult, error) {
	var worst *InjectionResult
	riskOrder := map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}

	for _, d := range m.detectors {
		result, err := d.Check(message)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.Name(), err)
		}
		if result.Detected {
			if worst == nil || riskOrder[result.Risk] > riskOrder[worst.Risk] {
				worst = result
			}
		}
	}

	if worst != nil {
		return worst, nil
	}
	return &InjectionResult{Detected: false}, nil
}

// ---------------------------------------------------------------------------
// Message-level scanning helper
// ---------------------------------------------------------------------------

// ScanMessages checks all messages in a list for prompt injection.
// Returns the highest-risk detection, or nil if safe.
func ScanMessages(messages []string, detector InjectionDetector) (*InjectionResult, error) {
	var worst *InjectionResult
	riskOrder := map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}

	for _, msg := range messages {
		result, err := detector.Check(msg)
		if err != nil {
			return nil, err
		}
		if result.Detected {
			if worst == nil || riskOrder[result.Risk] > riskOrder[worst.Risk] {
				worst = result
			}
		}
	}
	return worst, nil
}