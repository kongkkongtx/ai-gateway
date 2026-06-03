package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// EventType represents the type of webhook event.
type EventType string

const (
	EventCostAlert       EventType = "cost.alert"
	EventSecurityBlock   EventType = "security.block"
	EventSecuritySanitize EventType = "security.sanitize"
	EventUpstreamDown    EventType = "upstream.down"
	EventUpstreamUp      EventType = "upstream.up"
	EventExperimentSignificant EventType = "experiment.significant"
	EventEvalRunCompleted      EventType = "evaluation.completed"
	EventEvalRunFailed         EventType = "evaluation.failed"
	EventMCPToolExecuted       EventType = "mcp.tool.executed"
	EventMCPToolFailed         EventType = "mcp.tool.failed"
	EventMCPServerDown         EventType = "mcp.server.down"
	EventMCPServerUp           EventType = "mcp.server.up"
)

// Event carries data for webhook delivery.
type Event struct {
	Type      EventType            `json:"type"`
	Timestamp time.Time            `json:"timestamp"`
	Severity  string               `json:"severity"`
	Title     string               `json:"title"`
	Message   string               `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// EndpointConfig defines a webhook target.
type EndpointConfig struct {
	Name    string      `yaml:"name" json:"name"`
	URL     string      `yaml:"url" json:"url"`
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Events  []EventType `yaml:"events,omitempty" json:"events,omitempty"` // empty = all events
	Secret  string      `yaml:"secret,omitempty" json:"-"`               // HMAC signing secret
	Retry   int         `yaml:"retry,omitempty" json:"retry,omitempty"`  // max retries
}

// Config for the webhook system.
type Config struct {
	Endpoints []EndpointConfig `yaml:"endpoints,omitempty" json:"endpoints,omitempty"`
}

// Notifier manages webhook delivery with retry logic.
type Notifier struct {
	mu       sync.RWMutex
	config   Config
	client   *http.Client
	logger   *slog.Logger
	ch       chan Event
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// New creates a webhook notifier.
func New(cfg Config, logger *slog.Logger) *Notifier {
	return &Notifier{
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
		ch:     make(chan Event, 100),
		stopCh: make(chan struct{}),
	}
}

// Start launches the background event processing loop.
func (n *Notifier) Start() {
	n.wg.Add(1)
	go n.loop()
	n.logger.Info("webhook notifier started", "endpoints", len(n.config.Endpoints))
}

// Stop gracefully shuts down the notifier.
func (n *Notifier) Stop() {
	close(n.stopCh)
	n.wg.Wait()
	n.logger.Info("webhook notifier stopped")
}

// Send enqueues an event for delivery.
func (n *Notifier) Send(evt Event) {
	select {
	case n.ch <- evt:
	default:
		n.logger.Warn("webhook event channel full, dropping event", "type", evt.Type)
	}
}

// UpdateConfig hot-reloads the webhook configuration.
func (n *Notifier) UpdateConfig(cfg Config) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.config = cfg
}

func (n *Notifier) loop() {
	defer n.wg.Done()
	for {
		select {
		case evt := <-n.ch:
			n.deliver(evt)
		case <-n.stopCh:
			return
		}
	}
}

func (n *Notifier) deliver(evt Event) {
	n.mu.RLock()
	endpoints := n.config.Endpoints
	n.mu.RUnlock()

	for _, ep := range endpoints {
		if !ep.Enabled {
			continue
		}
		// Check if endpoint subscribes to this event type
		if len(ep.Events) > 0 {
			matched := false
			for _, e := range ep.Events {
				if e == evt.Type {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		go n.deliverTo(ep, evt)
	}
}

func (n *Notifier) deliverTo(ep EndpointConfig, evt Event) {
	body, err := json.Marshal(evt)
	if err != nil {
		n.logger.Error("failed to marshal webhook event", "error", err)
		return
	}

	maxRetries := ep.Retry
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			time.Sleep(time.Duration(1<<(attempt-1)) * time.Second)
		}

		req, err := http.NewRequest("POST", ep.URL, bytes.NewReader(body))
		if err != nil {
			n.logger.Error("failed to create webhook request", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "AI-Gateway-Webhook/1.0")

		if ep.Secret != "" {
			// Simple bearer auth for webhook signing
			req.Header.Set("Authorization", "Bearer "+ep.Secret)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req = req.WithContext(ctx)

		resp, err := n.client.Do(req)
		cancel()

		if err == nil && resp.StatusCode < 300 {
			resp.Body.Close()
			n.logger.Debug("webhook delivered",
				"endpoint", ep.Name,
				"event", evt.Type,
				"attempt", attempt+1,
			)
			return
		}

		if resp != nil {
			resp.Body.Close()
		}
		n.logger.Warn("webhook delivery failed",
			"endpoint", ep.Name,
			"event", evt.Type,
			"attempt", attempt+1,
			"error", err,
		)
	}
	n.logger.Error("webhook delivery exhausted retries",
		"endpoint", ep.Name,
		"event", evt.Type,
		"max_retries", maxRetries,
	)
}
