package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/yushi/ai-gateway/internal/security"
)

// SecurityMiddleware provides prompt injection detection and PII redaction
// for incoming AI requests. It intercepts the request body, performs security
// checks, and optionally blocks or sanitizes the request.
type SecurityMiddleware struct {
	promptDetector security.InjectionDetector
	piiRedactor    *security.PIIRedactor
	cfg            security.PromptInjectionConfig
	piiCfg         security.PIIConfig
	logger         *slog.Logger
}

// NewSecurity creates a SecurityMiddleware.
// If detector is nil, no injection checks are performed.
// If redactor is nil, no PII redaction is performed.
func NewSecurity(
	detector security.InjectionDetector,
	redactor *security.PIIRedactor,
	injCfg security.PromptInjectionConfig,
	piiCfg security.PIIConfig,
	logger *slog.Logger,
) *SecurityMiddleware {
	return &SecurityMiddleware{
		promptDetector: detector,
		piiRedactor:    redactor,
		cfg:            injCfg,
		piiCfg:         piiCfg,
		logger:         logger,
	}
}

// riskLevel maps string risk to numeric value for threshold comparison.
var riskLevel = map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}

func (s *SecurityMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip security checks for non-AI endpoints
		if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/v1/embeddings" {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":{"message":"Failed to read request body","type":"gateway_error"}}`, http.StatusBadRequest)
			return
		}

		// Only process chat completions (they have messages)
		if r.URL.Path == "/v1/chat/completions" {
			var chatReq struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
				Stream bool `json:"stream"`
			}
			if err := json.Unmarshal(body, &chatReq); err == nil {
				modified := s.processChatRequest(body, chatReq.Messages, chatReq.Stream)
				if modified == nil {
					return // Blocked
				}
				body = modified
			}
		}

		// Replace body with potentially modified version
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// processChatRequest runs injection check and PII redaction on chat messages.
// Returns modified body bytes, or nil if the request should be blocked.
func (s *SecurityMiddleware) processChatRequest(originalBody []byte, messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}, stream bool) []byte {

	// 1. Prompt injection detection
	if s.promptDetector != nil && s.cfg.Enabled {
		var messageTexts []string
		for _, msg := range messages {
			messageTexts = append(messageTexts, msg.Content)
		}

		result, err := security.ScanMessages(messageTexts, s.promptDetector)
		if err != nil {
			s.logger.Error("prompt injection scan failed", "error", err)
		} else if result != nil && result.Detected {
			threshold := riskLevel[s.cfg.RiskThresh]
			actual := riskLevel[result.Risk]

			if actual >= threshold {
				s.logger.Warn("prompt injection blocked",
					"risk", result.Risk,
					"pattern", result.Pattern,
					"action", s.cfg.Action)

				switch s.cfg.Action {
				case "block":
					// We can't write here directly; return nil to signal caller to write error
					return nil
				case "log":
					// Just log and continue
				case "sanitize":
					// TODO: implement content sanitization
				}
			}
		}
	}

	// 2. PII redaction on messages
	if s.piiRedactor != nil && s.piiCfg.Enabled {
		redacted := false
		for i, msg := range messages {
			newContent, matches, err := s.piiRedactor.Redact(msg.Content)
			if err != nil {
				s.logger.Warn("PII check blocked request",
					"error", err)
				return nil
			}
			if len(matches) > 0 {
				redacted = true
				messages[i].Content = newContent
				for _, m := range matches {
					s.logger.Info("PII redacted",
						"type", m.Type,
						"position", m.Start)
				}
			}
		}
		if redacted {
			// Re-marshal with redacted content
			var reqMap map[string]interface{}
			if err := json.Unmarshal(originalBody, &reqMap); err == nil {
				// Replace messages array
				type msgStruct struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}
				var newMsgs []msgStruct
				for _, m := range messages {
					newMsgs = append(newMsgs, msgStruct{Role: m.Role, Content: m.Content})
				}
				reqMap["messages"] = newMsgs
				modified, err := json.Marshal(reqMap)
				if err == nil {
					return modified
				}
			}
		}
	}

	return originalBody
}