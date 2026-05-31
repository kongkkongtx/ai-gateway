package security

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

// PIIRedactor handles detection and redaction of personally identifiable information.
type PIIRedactor struct {
	rules  []piiRule
	action string // "mask", "hash", or "block"
}

type piiRule struct {
	Type    string
	Pattern *regexp.Regexp
	Mask    string // Replacement template; use "{hash}" for hashed value
}

// built-in PII patterns
var defaultPIIRules = []piiRule{
	{
		Type:    "email",
		Pattern: regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
		Mask:    "[EMAIL]",
	},
	{
		Type:    "phone",
		Pattern: regexp.MustCompile(`\b(\+?\d{1,3}[-.\s]?)?\(?\d{2,4}\)?[-.\s]?\d{3,4}[-.\s]?\d{3,4}\b`),
		Mask:    "[PHONE]",
	},
	{
		Type:    "ip",
		Pattern: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		Mask:    "[IP]",
	},
	{
		Type:    "api_key",
		Pattern: regexp.MustCompile(`\b(sk-[A-Za-z0-9]{20,}|sk-ant-[A-Za-z0-9]{20,}|AIza[A-Za-z0-9\-_]{35,})\b`),
		Mask:    "[API_KEY]",
	},
	{
		Type:    "credit_card",
		Pattern: regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
		Mask:    "[CREDIT_CARD]",
	},
	{
		Type:    "ssn",
		Pattern: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		Mask:    "[SSN]",
	},
	{
		Type:    "authorization",
		Pattern: regexp.MustCompile(`(?i)(Bearer\s+|api-key\s+)(sk-[A-Za-z0-9]{10,}|[A-Za-z0-9+/=]{20,})`),
		Mask:    "$1[REDACTED]",
	},
}

// NewPIIRedactor creates a PII redactor.
// enabledTypes: which PII types to check. Empty = all types.
// action: "mask" | "hash" | "block"
func NewPIIRedactor(enabledTypes []string, action string) *PIIRedactor {
	allowed := make(map[string]bool)
	for _, t := range enabledTypes {
		allowed[strings.ToLower(t)] = true
	}

	var activeRules []piiRule
	if len(enabledTypes) == 0 {
		activeRules = defaultPIIRules
	} else {
		for _, r := range defaultPIIRules {
			if allowed[r.Type] {
				activeRules = append(activeRules, r)
			}
		}
	}

	if action == "" {
		action = "mask"
	}

	return &PIIRedactor{rules: activeRules, action: action}
}

// Redact scans text for PII and returns the redacted version plus detected matches.
// If action is "block", it returns an error on first PII detection.
func (r *PIIRedactor) Redact(text string) (string, []PIIMatch, error) {
	var matches []PIIMatch
	result := text

	for _, rule := range r.rules {
		locs := rule.Pattern.FindAllStringSubmatchIndex(result, -1)
		// Process in reverse order to keep indices stable
		for i := len(locs) - 1; i >= 0; i-- {
			loc := locs[i]
			start := loc[0]
			end := loc[1]
			original := result[start:end]

			replacement := r.buildReplacement(rule, original)

			if r.action == "block" {
				return "", nil, fmt.Errorf("PII detected: %s (action=block)", rule.Type)
			}

			result = result[:start] + replacement + result[end:]

			matches = append([]PIIMatch{{
				Type:     rule.Type,
				Original: original,
				Redacted: replacement,
				Start:    start,
				End:      start + len(replacement),
			}}, matches...)
		}
	}

	return result, matches, nil
}

// RedactContent redacts PII from a content string. Convenience wrapper for quick use.
func (r *PIIRedactor) RedactContent(content string) string {
	redacted, _, _ := r.Redact(content)
	return redacted
}

func (r *PIIRedactor) buildReplacement(rule piiRule, original string) string {
	switch r.action {
	case "hash":
		hash := sha256.Sum256([]byte(original))
		return fmt.Sprintf("[%s:%x]", strings.ToUpper(rule.Type), hash[:8])
	default: // "mask"
		// Handle templates with $1 backreferences
		if strings.Contains(rule.Mask, "$1") {
			match := rule.Pattern.FindStringSubmatch(original)
			if len(match) > 1 {
				return strings.Replace(rule.Mask, "$1", match[1], 1)
			}
		}
		return rule.Mask
	}
}