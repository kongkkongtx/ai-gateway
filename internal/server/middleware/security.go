package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"log/slog"
	"net/http"

	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
	"github.com/kongkkongtx/ai-gateway/internal/security"
)

// SecurityEventCallback is called when a security event occurs.
type SecurityEventCallback func(eventType, severity, title, message string, details map[string]interface{})

// SecurityMiddleware provides prompt injection detection and PII redaction
// for incoming AI requests, plus IP filtering.
type SecurityMiddleware struct {
	promptDetector security.InjectionDetector
	piiRedactor    *security.PIIRedactor
	injCfg         security.PromptInjectionConfig
	piiCfg         security.PIIConfig
	ipAllowlist    []string
	ipBlocklist    []string
	logger         *slog.Logger
	onEvent        SecurityEventCallback
}

// NewSecurity creates a SecurityMiddleware.
func NewSecurity(
	detector security.InjectionDetector,
	redactor *security.PIIRedactor,
	cfg security.Config,
	logger *slog.Logger,
	onEvent SecurityEventCallback,
) *SecurityMiddleware {
	return &SecurityMiddleware{
		promptDetector: detector,
		piiRedactor:    redactor,
		injCfg:         cfg.PromptInjection,
		piiCfg:         cfg.PII,
		ipAllowlist:    cfg.IPAllowlist,
		ipBlocklist:    cfg.IPBlocklist,
		logger:         logger,
		onEvent:        onEvent,
	}
}

var riskLevel = map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}

func (s *SecurityMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// IP filtering (applies to all endpoints)
		clientIP := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			clientIP = strings.Split(fwd, ",")[0]
		}
		clientIP = strings.TrimSpace(clientIP)
		if idx := strings.LastIndex(clientIP, ":"); idx > 0 {
			clientIP = clientIP[:idx]
		}

		if len(s.ipBlocklist) > 0 {
			for _, blocked := range s.ipBlocklist {
				if clientIP == blocked || (strings.HasSuffix(blocked, "*") && strings.HasPrefix(clientIP, blocked[:len(blocked)-1])) {
					s.logger.Warn("IP blocked by security policy", "ip", clientIP)
					http.Error(w, `{"error":{"message":"Access denied by security policy","type":"security_error","code":"ip_blocked"}}`, http.StatusForbidden)
					return
				}
			}
		}
		if len(s.ipAllowlist) > 0 {
			allowed := false
			for _, allowedIP := range s.ipAllowlist {
				if clientIP == allowedIP || (strings.HasSuffix(allowedIP, "*") && strings.HasPrefix(clientIP, allowedIP[:len(allowedIP)-1])) {
					allowed = true
					break
				}
			}
			if !allowed {
				s.logger.Warn("IP not in allowlist", "ip", clientIP)
				http.Error(w, `{"error":{"message":"Access denied by security policy","type":"security_error","code":"ip_not_allowed"}}`, http.StatusForbidden)
				return
			}
		}

		// Skip further security checks for non-AI endpoints
		if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/v1/embeddings" {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":{"message":"Failed to read request body","type":"gateway_error"}}`, http.StatusBadRequest)
			return
		}

		if r.URL.Path == "/v1/chat/completions" {
			var chatReq struct {
				Messages []struct {
					Role    string `json:"role"`
					Content any `json:"content"`
				} `json:"messages"`
				Stream bool `json:"stream"`
			}
			if err := json.Unmarshal(body, &chatReq); err == nil {
				modified := s.processChatRequest(body, chatReq.Messages, chatReq.Stream)
				if modified == nil {
					http.Error(w, `{"error":{"message":"Request blocked by security policy","type":"security_error","code":"content_filter"}}`, http.StatusForbidden)
					return
				}
				body = modified
			}
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

func (s *SecurityMiddleware) processChatRequest(originalBody []byte, messages []struct {
	Role    string `json:"role"`
	Content any `json:"content"`
}, stream bool) []byte {

	var sanitized bool
	if s.promptDetector != nil && s.injCfg.Enabled {
		var messageTexts []string
		for _, msg := range messages {
			messageTexts = append(messageTexts, openai.ExtractText(msg.Content))
		}

		result, err := security.ScanMessages(messageTexts, s.promptDetector)
		if err != nil {
			s.logger.Error("prompt injection scan failed", "error", err)
		} else if result != nil && result.Detected {
			threshold := riskLevel[s.injCfg.RiskThresh]
			actual := riskLevel[result.Risk]

			if actual >= threshold {
				s.logger.Warn("prompt injection blocked",
					"risk", result.Risk,
					"pattern", result.Pattern,
					"action", s.injCfg.Action)

				if s.onEvent != nil {
					severity := "warning"
					if result.Risk == "high" || result.Risk == "critical" {
						severity = "critical"
					}
					s.onEvent("security.block", severity,
						fmt.Sprintf("Prompt injection detected: %s", result.Pattern),
						fmt.Sprintf("Risk level %s, action: %s", result.Risk, s.injCfg.Action),
						map[string]interface{}{
							"risk": result.Risk,
							"pattern": result.Pattern,
							"action": s.injCfg.Action,
						})
				}

				switch s.injCfg.Action {
				case "block":
					return nil
				case "log":
				case "sanitize":
					for i, msg := range messages {
						cleaned, changed := sanitizeContent(openai.ExtractText(msg.Content), result.Pattern)
						if changed {
							s.logger.Info("prompt sanitized", "message_idx", i, "pattern", result.Pattern)
							// sanitize only modifies text, not images
								messages[i].Content = cleaned
						}
					}
					sanitized = true
				}
			}
		}
	}

	if s.piiRedactor != nil && s.piiCfg.Enabled {
		if s.onEvent != nil && len(s.piiCfg.Types) > 0 {
			s.onEvent("security.sanitize", "info",
				"PII detected and masked",
				fmt.Sprintf("PII types: %v", s.piiCfg.Types),
				map[string]interface{}{
					"types": s.piiCfg.Types,
					"action": s.piiCfg.Action,
				})
		}
		redacted := false
		for i, msg := range messages {
			newContent, matches, err := s.piiRedactor.Redact(openai.ExtractText(msg.Content))
			if err != nil {
				s.logger.Warn("PII check blocked request", "error", err)
				return nil
			}
			if len(matches) > 0 {
				redacted = true
				messages[i].Content = newContent
				for _, m := range matches {
					s.logger.Info("PII redacted", "type", m.Type, "position", m.Start)
				}
			}
		}
		if redacted {
			var reqMap map[string]interface{}
			if err := json.Unmarshal(originalBody, &reqMap); err == nil {
				type msgStruct struct {
					Role    string `json:"role"`
					Content any `json:"content"`
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

	if sanitized {
		var reqMap map[string]interface{}
		if err := json.Unmarshal(originalBody, &reqMap); err == nil {
			type msgStruct struct {
				Role    string `json:"role"`
				Content any `json:"content"`
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

	return originalBody
}

func sanitizeContent(content, pattern string) (string, bool) {
	lower := strings.ToLower(content)
	patLower := strings.ToLower(pattern)
	idx := strings.Index(lower, patLower)
	if idx < 0 {
		return content, false
	}
	content = content[:idx] + "[SANITIZED]" + content[idx+len(pattern):]
	return content, true
}
