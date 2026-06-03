package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/kongkkongtx/ai-gateway/internal/balancer"
	"github.com/kongkkongtx/ai-gateway/internal/metrics"
	"github.com/kongkkongtx/ai-gateway/internal/cache"
	"github.com/kongkkongtx/ai-gateway/internal/cost"
	"github.com/kongkkongtx/ai-gateway/internal/prompt"
	"github.com/kongkkongtx/ai-gateway/internal/webhook"
	"github.com/kongkkongtx/ai-gateway/internal/plugin"
	"github.com/kongkkongtx/ai-gateway/internal/config"
	"github.com/kongkkongtx/ai-gateway/internal/provider"
	"github.com/kongkkongtx/ai-gateway/internal/provider/anthropic"
	"github.com/kongkkongtx/ai-gateway/internal/provider/azure"
	"github.com/kongkkongtx/ai-gateway/internal/provider/google"
	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
	"github.com/kongkkongtx/ai-gateway/internal/security"
	"github.com/kongkkongtx/ai-gateway/internal/router"
	"github.com/kongkkongtx/ai-gateway/internal/semantic"
	"github.com/kongkkongtx/ai-gateway/internal/server/middleware"
	"github.com/kongkkongtx/ai-gateway/internal/tracing"
	"github.com/kongkkongtx/ai-gateway/internal/user"
	"github.com/kongkkongtx/ai-gateway/internal/experiment"
	"github.com/kongkkongtx/ai-gateway/internal/evaluation"
	"github.com/kongkkongtx/ai-gateway/internal/mcp"
	"github.com/kongkkongtx/ai-gateway/internal/rag"
)

type Gateway struct {
	cfg            *config.Config
	router         *router.Matcher
	balancer       *balancer.Balancer
	metrics        *metrics.Collector
	server         *http.Server
	logger         *slog.Logger
	adapters       map[string]provider.ProviderAdapter
	providerMap    map[string]string
	tp             *sdktrace.TracerProvider
	configPath     string
	watcherStop    func()
	redisClient    *cache.Client
	semanticRouter *semantic.Router
	semanticCache  *semantic.Cache
	costTracker    *cost.Tracker
	auditStore     middleware.AuditStore
	promptManager  *prompt.Manager
	webhookNotifier *webhook.Notifier
	pluginManager   *plugin.Manager
	userStore        *user.Store
	experimentEngine  *experiment.Engine
	evaluationRunner  *evaluation.Runner
	prewarmer           *semantic.Prewarmer
	mcpHub              *mcp.Hub
	ragStore           rag.VectorStore
	ragEmbedder        rag.Embedder
}

func New(cfg *config.Config, configPath string) (*Gateway, error) {
	logger := initLogger(cfg.Log)

	tp, err := tracing.Init("ai-gateway")
	if err != nil {
		logger.Warn("failed to init tracing, continuing without", "error", err)
	}

	m := router.NewMatcher(cfg.Routes)
	b := balancer.New(logger)

	reg := prometheus.NewRegistry()
	collector := metrics.NewCollector(reg)

	adapters := make(map[string]provider.ProviderAdapter)
	providerMap := make(map[string]string)

	for _, u := range cfg.Upstream {
		b.AddUpstream(u.Name, u.Endpoint, u.Provider, u.Weight)
				switch u.Provider {
		case "openai":
			adapters[u.Name] = openai.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "anthropic":
			adapters[u.Name] = anthropic.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "google", "gemini":
			adapters[u.Name] = google.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "azure":
			adapters[u.Name] = azure.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout, u.APIVersion)
		default:
			logger.Warn("unsupported provider, will proxy directly", "provider", u.Provider, "name", u.Name)
		}
		providerMap[u.Name] = u.Provider
	}

	if len(cfg.Upstream) > 0 {
		interval := 30 * time.Second
		timeout := 5 * time.Second
		if cfg.Upstream[0].HealthCheck != nil {
			interval = cfg.Upstream[0].HealthCheck.Interval
			timeout = cfg.Upstream[0].HealthCheck.Timeout
		}
		b.StartHealthChecks(context.Background(), interval, timeout)
	}

	// Initialize Redis client (nil if not configured)
	redisClient := cache.New(cache.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if redisClient != nil {
		if err := redisClient.Ping(context.Background()); err != nil {
			logger.Warn("redis configured but unreachable, running without Redis", "error", err)
			redisClient = nil
		} else {
			logger.Info("redis connected", "addr", cfg.Redis.Addr)
		}
	}


	// Initialize semantic router
	var semRouter *semantic.Router
	var semCache *semantic.Cache
	var prewarmer *semantic.Prewarmer
	if cfg.Semantic.Enabled || cfg.SemanticCache.Enabled {
		if embedAdapter, ok := adapters[cfg.Semantic.Provider]; ok {
			oaAdapter, ok2 := embedAdapter.(*openai.Adapter)
			if ok2 {
				embedder := semantic.NewEmbeddingClient(oaAdapter, "text-embedding-3-small")
				if cfg.Semantic.Enabled {
					semRouter = semantic.NewRouter(cfg.Semantic, embedder, logger)
					if err := semRouter.Initialize(); err != nil {
						logger.Warn("semantic router init failed", "error", err)
					}
				}
				if cfg.SemanticCache.Enabled {
					semCache = semantic.NewCache(cfg.SemanticCache, redisClient, embedder, logger)
				}
			}
		}
		if semRouter == nil && cfg.Semantic.Enabled {
			logger.Warn("semantic router enabled but no embedding provider configured",
				"provider", cfg.Semantic.Provider)
		}

		// Initialize cache prewarmer
		if cfg.SemanticCache.Prewarm.Enabled && semCache != nil {
			var pwEmbedder semantic.Embedder
			if semCache != nil {
				pwEmbedder = semCache.Embedder()
			}
			if pwEmbedder != nil {
				prewarmer = semantic.NewPrewarmer(semCache, cfg.SemanticCache.Prewarm, pwEmbedder, logger)
				logger.Info("cache prewarmer initialized", "seed_queries", len(cfg.SemanticCache.Prewarm.SeedQueries))
			} else {
				logger.Warn("cache prewarmer disabled: cache has no embedder")
			}
		}
	}


	// Initialize cost tracker
	var costTracker *cost.Tracker
	if cfg.Cost.Enabled {
		costTracker = cost.NewTracker(cfg.Cost, logger)
		logger.Info("cost tracker initialized",
			"default_limit_input", cfg.Cost.DefaultLimit.InputTokens,
			"default_limit_output", cfg.Cost.DefaultLimit.OutputTokens)
	}

	// Initialize semantic middleware
	var semanticMiddleware *middleware.SemanticMiddleware
	if semRouter != nil || semCache != nil {
		semanticMiddleware = middleware.NewSemantic(semRouter, semCache, logger)
		logger.Info("semantic middleware initialized",
			"router", semRouter != nil,
			"cache", semCache != nil)
	}

	// Initialize RAG middleware
	// Always initialize RAG store for admin API
	var ragStore rag.VectorStore = rag.NewMemoryStore()
	var ragEmbedder rag.Embedder
	var ragMiddleware *middleware.RAGMiddleware
	if cfg.RAG.Enabled {
		if embProvider, ok := adapters[cfg.RAG.EmbeddingProvider]; ok {
			if oaAdapter, ok2 := embProvider.(*openai.Adapter); ok2 {
				ragEmbedder = rag.NewEmbeddingAdapter(oaAdapter, cfg.RAG.EmbeddingModel)
				ragMiddleware = middleware.NewRAG(cfg.RAG, ragStore, ragEmbedder, logger)
				logger.Info("rag middleware initialized", "kb", len(cfg.RAG.KnowledgeBases), "topK", cfg.RAG.TopK)
			} else {
				logger.Warn("rag: embedding provider is not an OpenAI adapter")
			}
		} else {
			logger.Warn("rag: embedding provider not found, RAG disabled", "provider", cfg.RAG.EmbeddingProvider)
		}
	}


	// Initialize prompt template manager
	promptManager := prompt.NewManager()
	logger.Info("prompt template manager initialized")

	// Initialize webhook notifier
	webhookNotifier := webhook.New(cfg.Webhook, logger)
	if cfg.Webhook.Endpoints != nil && len(cfg.Webhook.Endpoints) > 0 {
		webhookNotifier.Start()
	}

	// Initialize plugin manager
	pluginManager := plugin.NewManager(cfg.Plugin, logger)
	if len(cfg.Plugin.Plugins) > 0 {
		logger.Info("plugin manager initialized", "count", len(cfg.Plugin.Plugins))
	}

	// Initialize experiment engine
	var experimentEngine *experiment.Engine
	if cfg.Experiment.Enabled {
		experimentEngine = experiment.NewEngine(cfg.Experiment, logger)
		logger.Info("experiment engine initialized", "experiments", len(cfg.Experiment.Experiments))
	}

	// Initialize evaluation runner
	var evaluationRunner *evaluation.Runner
	if cfg.Evaluation.Enabled {
		evaluationRunner = evaluation.NewRunner(cfg.Evaluation, adapters, nil, logger, webhookNotifier)
		logger.Info("evaluation runner initialized", "suites", len(cfg.Evaluation.Suites))
	}

	// Initialize MCP Hub
	var mcpHub *mcp.Hub
	if cfg.MCP.Enabled {
		mcpHub = mcp.NewHub(cfg.MCP, logger)
		mcpHub.ConnectAll()
		logger.Info("mcp hub initialized", "servers", len(cfg.MCP.Servers))
	}

	// Initialize audit store with file persistence
	var auditStore middleware.AuditStore
	if cfg.Audit.Enabled && cfg.Audit.FilePath != "" {
		fileStore, err := middleware.NewFileAuditStore(cfg.Audit.FilePath, cfg.Audit.BufferSize)
		if err != nil {
			logger.Warn("failed to create file audit store, falling back to memory", "error", err)
			auditStore = middleware.NewMemoryAuditStore(10000)
		} else {
			auditStore = fileStore
			logger.Info("file audit store initialized", "path", cfg.Audit.FilePath, "buffer", cfg.Audit.BufferSize)
		}
	} else {
		auditStore = middleware.NewMemoryAuditStore(10000)
		logger.Info("memory audit store initialized", "capacity", 10000)
	}

	// Initialize cost middleware
	var costMiddleware *middleware.CostMiddleware
	if cfg.Cost.Enabled && costTracker != nil {
		costMiddleware = middleware.NewCost(costTracker, logger)
		logger.Info("cost middleware initialized")
	}

		// Initialize metrics collector for the test handler
	metricsReg := prometheus.NewRegistry()
	metricsCollector := metrics.NewCollector(metricsReg)

	// Initialize user store for JWT authentication
	userStore, _ := user.NewStore("data/users.json")
	if userStore != nil {
		logger.Info("user store initialized", "path", "data/users.json")
	}
	gw := &Gateway{
		cfg: cfg, router: m, balancer: b, metrics: metricsCollector,
		logger: logger, adapters: adapters, providerMap: providerMap,
		tp: tp,
		redisClient: redisClient,
		semanticRouter: semRouter,
		semanticCache: semCache,
		prewarmer:      prewarmer,
		costTracker: costTracker,
		auditStore: auditStore,
		promptManager:    promptManager,
		webhookNotifier:  webhookNotifier,
		pluginManager:    pluginManager,
		experimentEngine: experimentEngine,
		evaluationRunner:   evaluationRunner,
		mcpHub:             mcpHub,
		ragStore:           ragStore,
		ragEmbedder:        ragEmbedder,
		userStore: userStore,
	}

	// Initialize security components
	var securityMiddleware *middleware.SecurityMiddleware
	if cfg.Security.PromptInjection.Enabled || cfg.Security.PII.Enabled {
		injDetector := security.NewRuleDetector(cfg.Security.PromptInjection.Keywords, logger)
		piiRedactor := security.NewPIIRedactor(cfg.Security.PII.Types, cfg.Security.PII.Action)
		securityMiddleware = middleware.NewSecurity(
			injDetector, piiRedactor,
			cfg.Security,
			logger,
			func(evtType, severity, title, msg string, details map[string]interface{}) {
				if webhookNotifier != nil {
					webhookNotifier.Send(webhook.Event{
						Type:      webhook.EventType(evtType),
						Timestamp: time.Now(),
						Severity:  severity,
						Title:     title,
						Message:   msg,
						Details:   details,
					})
				}
			})
		logger.Info("security middleware initialized",
			"prompt_injection", cfg.Security.PromptInjection.Enabled,
			"pii", cfg.Security.PII.Enabled)

	}


	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.NewTracingMiddleware().Middleware)
	r.Use(middleware.NewRecovery(logger).Middleware)
	r.Use(middleware.NewAudit(logger, gw.auditStore).Middleware)
	r.Use(middleware.NewRateLimiter(cfg.RateLimit, redisClient, logger).Middleware)
	authMiddleware := middleware.NewAuth(cfg.Auth.Enabled, buildKeyMap(cfg.Auth.Keys), buildRoleMap(cfg.Auth.Keys), logger)
	if gw.userStore != nil {
		authMiddleware.SetUserStore(gw.userStore)
	}
	r.Use(authMiddleware.Middleware)
	r.Use(middleware.NewAdminPermission().Middleware)
	r.Use(chimw.Timeout(cfg.Server.WriteTimeout))
	r.Use(middleware.NewCORS(nil).Middleware)
	r.Use(middleware.NewCORS(nil).Middleware)
	if securityMiddleware != nil {
		r.Use(securityMiddleware.Middleware)
	}
	if semanticMiddleware != nil {
		r.Use(semanticMiddleware.Middleware)
	}
	if ragMiddleware != nil {
		r.Use(ragMiddleware.Middleware)
	}
	if costMiddleware != nil {
		r.Use(costMiddleware.Middleware)
	}


	r.Handle("/metrics", collector.Handler())
	r.Get("/admin/health", gw.handleHealth)
	r.Get("/admin/status", gw.handleStatus)
	r.Get("/admin/upstreams", gw.handleListUpstreams)
	r.Get("/admin/routes", gw.handleListRoutes)
	r.Get("/admin/keys", gw.handleListKeys)
	r.Post("/admin/routes", gw.handleReloadRoutes)
	r.Post("/admin/upstreams", gw.handleReloadUpstreams)
	r.Post("/admin/keys", gw.handleAddKey)
	r.Delete("/admin/keys/{key}", gw.handleDeleteKey)
	r.Post("/admin/upstreams/single", gw.handleAddUpstream)
	r.Delete("/admin/upstreams/{name}", gw.handleDeleteUpstream)
	r.Post("/admin/routes/single", gw.handleAddRoute)
	r.Delete("/admin/routes/{id}", gw.handleDeleteRoute)
	r.Get("/admin/config", gw.handleGetConfig)
	r.Put("/admin/config", gw.handleUpdateConfig)
	r.Get("/admin/audit-logs", gw.handleGetAuditLogs)
	r.Get("/admin/cost-stats", gw.handleGetCostStats)
	r.Get("/openapi.yaml", gw.handleOpenAPISpec)
	r.Get("/admin/config/versions", gw.handleConfigVersions)
	r.Post("/admin/config/rollback/{version}", gw.handleConfigRollback)
	r.Get("/admin/health/history", gw.handleHealthHistory)
	r.Get("/admin/plugins", gw.handleListPlugins)
	r.Post("/admin/plugins", gw.handleReloadPlugins)
	r.Get("/admin/webhook", gw.handleGetWebhookConfig)
	r.Put("/admin/webhook", gw.handleUpdateWebhookConfig)
	r.Get("/admin/prompts", gw.handleListPrompts)
	r.Post("/admin/prompts", gw.handleSavePrompt)
	r.Get("/admin/prompts/{id}", gw.handleGetPrompt)
	r.Delete("/admin/prompts/{id}", gw.handleDeletePrompt)
	r.Post("/admin/prompts/{id}/versions", gw.handleAddPromptVersion)
		r.Post("/admin/login", gw.handleLogin)
	r.Get("/admin/users", gw.handleListUsers)
	r.Post("/admin/users", gw.handleCreateUser)
	r.Delete("/admin/users/{username}", gw.handleDeleteUser)
	r.Put("/admin/users/{username}/role", gw.handleUpdateUserRole)
	r.Post("/admin/keys/{key}/rotate", gw.handleRotateKey)
	r.Get("/admin/security/policies", gw.handleGetSecurityPolicies)
	r.Put("/admin/security/policies", gw.handleUpdateSecurityPolicies)
	r.Get("/admin/experiments", gw.handleListExperiments)
	r.Post("/admin/experiments", gw.handleCreateExperiment)
	r.Get("/admin/experiments/{id}", gw.handleGetExperiment)
	r.Put("/admin/experiments/{id}", gw.handleUpdateExperiment)
	r.Delete("/admin/experiments/{id}", gw.handleDeleteExperiment)
	r.Get("/admin/experiments/{id}/results", gw.handleExperimentResults)
	r.Post("/admin/experiments/{id}/start", gw.handleStartExperiment)
	r.Post("/admin/experiments/{id}/stop", gw.handleStopExperiment)

	r.Get("/admin/evaluations", gw.handleListEvalSuites)
	r.Post("/admin/evaluations", gw.handleCreateEvalSuite)
	r.Get("/admin/evaluations/{id}", gw.handleGetEvalSuite)
	r.Delete("/admin/evaluations/{id}", gw.handleDeleteEvalSuite)
	r.Post("/admin/evaluations/{id}/run", gw.handleRunEvaluation)
	r.Get("/admin/evaluations/{id}/runs", gw.handleListEvalRuns)
	r.Get("/admin/evaluations/{id}/runs/{runId}", gw.handleGetEvalRun)
	r.Post("/admin/evaluations/{id}/runs/{runId}/cancel", gw.handleCancelEvalRun)
	r.Post("/admin/mcp/servers", gw.handleAddMCPServer)
	r.Get("/admin/mcp/servers", gw.handleListMCPServers)
	r.Get("/admin/mcp/servers/{name}", gw.handleGetMCPServer)
	r.Put("/admin/mcp/servers/{name}", gw.handleUpdateMCPServer)
	r.Delete("/admin/mcp/servers/{name}", gw.handleDeleteMCPServer)
		r.Get("/admin/cache/stats", gw.handleCacheStats)
	r.Post("/admin/cache/prewarm", gw.handleCachePrewarm)
	r.Get("/admin/cache/prewarm/stats", gw.handleCachePrewarmStats)
r.Get("/admin/mcp/tools", gw.handleListMCPTools)
	r.Post("/admin/mcp/servers/{name}/reconnect", gw.handleReconnectMCPServer)
	r.Post("/admin/mcp/reload", gw.handleReloadMCP)
	r.Get("/admin/rag/knowledge-bases", gw.handleListKnowledgeBases)
	r.Post("/admin/rag/knowledge-bases", gw.handleCreateKnowledgeBase)
	r.Get("/admin/rag/knowledge-bases/{id}", gw.handleGetKnowledgeBase)
	r.Delete("/admin/rag/knowledge-bases/{id}", gw.handleDeleteKnowledgeBase)
	r.Post("/admin/rag/knowledge-bases/{id}/ingest", gw.handleIngestDocument)
	r.Post("/admin/rag/knowledge-bases/{id}/ingest-batch", gw.handleIngestBatch)
	r.Get("/admin/rag/knowledge-bases/{id}/stats", gw.handleKBStats)
	r.Delete("/admin/rag/knowledge-bases/{id}/chunks/{chunkID}", gw.handleDeleteChunk)

	r.Post("/v1/chat/completions", gw.handleChatCompletion)
	// Serve UI static files for SPA
	uiFS := http.FileServer(http.Dir("./ui/dist"))
	// MCP JSON-RPC endpoint - handled via inbound middleware before chi routing
	// (POST to /mcp is intercepted by the mcpHandler wrapper before chi's /* catch-all)
	
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/admin/") || strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/metrics" {
			http.NotFound(w, r)
			return
		}
		path := "./ui/dist" + r.URL.Path
		if _, err := os.Stat(path); err == nil {
			uiFS.ServeHTTP(w, r)
		} else {
			http.ServeFile(w, r, "./ui/dist/index.html")
		}
	})
	r.Post("/v1/embeddings", gw.handleEmbeddings)

	// MCP interceptor - catches POST /mcp before chi routing (avoids catch-all conflict)
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost && req.URL.Path == "/mcp" {
			if mcpHub != nil {
				mcpHub.ServeHTTP(w, req)
			} else {
				mcpError(w, "MCP hub not initialized")
			}
			return
		}
		r.ServeHTTP(w, req)
	})

	gw.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	return gw, nil
}

// Reload updates the gateway's internal state from a new Config without restarting.
func (g *Gateway) Reload(cfg *config.Config) error {
	g.logger.Info("hot-reloading configuration")

	g.router.Update(cfg.Routes)
	g.cfg.Routes = cfg.Routes

	current := make(map[string]bool)
	for _, u := range g.balancer.Upstreams() {
		current[u.Name] = false
	}
	for _, u := range cfg.Upstream {
		if _, exists := current[u.Name]; exists {
			current[u.Name] = true
			continue
		}
		g.balancer.AddUpstream(u.Name, u.Endpoint, u.Provider, u.Weight)
				switch u.Provider {
		case "openai":
			g.adapters[u.Name] = openai.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "anthropic":
			g.adapters[u.Name] = anthropic.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "google", "gemini":
			g.adapters[u.Name] = google.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "azure":
			g.adapters[u.Name] = azure.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout, u.APIVersion)
		}
		g.providerMap[u.Name] = u.Provider
	}
	for name, kept := range current {
		if !kept {
			g.balancer.RemoveUpstream(name)
			delete(g.adapters, name)
			delete(g.providerMap, name)
			g.logger.Info("removed upstream via reload", "name", name)
		}
	}
	g.cfg.Upstream = cfg.Upstream

	if len(cfg.Upstream) > 0 {
		interval := 30 * time.Second
		timeout := 5 * time.Second
		if cfg.Upstream[0].HealthCheck != nil {
			interval = cfg.Upstream[0].HealthCheck.Interval
			timeout = cfg.Upstream[0].HealthCheck.Timeout
		}
		g.balancer.StartHealthChecks(context.Background(), interval, timeout)
	}

	if cfg.Server.Port != g.cfg.Server.Port {
		g.logger.Warn("server port change requires restart to take effect")
	}

	g.logger.Info("hot-reload complete", "routes", len(cfg.Routes), "upstreams", len(cfg.Upstream))
	return nil
}

func (g *Gateway) Start() error {
		if g.prewarmer != nil {
		g.prewarmer.Start(context.Background())
	}
	g.logger.Info("gateway starting",
		"addr", g.server.Addr,
		"routes", len(g.router.Routes()),
		"upstreams", len(g.balancer.Upstreams()),
	)

	if g.configPath != "" {
		stop, err := config.Watch(g.configPath, g.Reload, g.logger)
		if err != nil {
			g.logger.Warn("config file watching not available, hot-reload disabled", "error", err)
		} else {
			g.watcherStop = stop
			g.logger.Info("config file watcher started", "path", g.configPath)
		}
	}

	return g.server.ListenAndServe()
}

func (g *Gateway) Shutdown(ctx context.Context) error {
	g.logger.Info("gateway shutting down")
	if g.prewarmer != nil {
		g.prewarmer.Stop()
	}
	if g.watcherStop != nil {
		g.watcherStop()
	}
	if g.tp != nil {
		if err := g.tp.Shutdown(ctx); err != nil {
			g.logger.Error("tracer provider shutdown error", "error", err)
		}
	}
	if g.redisClient != nil {
		if err := g.redisClient.Close(); err != nil {
			g.logger.Error("redis close error", "error", err)
		}
		g.logger.Info("redis connection closed")
	}
	return g.server.Shutdown(ctx)
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	upstreams := g.balancer.Upstreams()
	statusList := make([]map[string]interface{}, 0, len(upstreams))
	for _, u := range upstreams {
		statusList = append(statusList, map[string]interface{}{
			"name": u.Name, "endpoint": u.Endpoint, "provider": u.Provider,
			"healthy": u.Healthy.Load(), "weight": u.Weight,
		})
	}
	routes := g.router.Routes()
	routeList := make([]map[string]interface{}, 0, len(routes))
	for _, r := range routes {
		routeList = append(routeList, map[string]interface{}{
			"id": r.ID, "model": r.Model, "upstream": r.Upstream,
			"fallbacks": r.Fallbacks, "priority": r.Priority,
		})
	}

	// Check Redis connectivity
	redisStatus := "not_configured"
	if g.redisClient != nil {
		if err := g.redisClient.Ping(r.Context()); err != nil {
			redisStatus = "disconnected"
		} else {
			redisStatus = "connected"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok", "upstreams": statusList, "routes": routeList,
		"redis": redisStatus,
	})
}


func (g *Gateway) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	g.metrics.IncActiveRequests()
	defer g.metrics.DecActiveRequests()
	start := time.Now()

	span := trace.SpanFromContext(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var chatReq openai.ChatCompletionRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	span.SetAttributes(attribute.String("ai.model", chatReq.Model))
	rCtx := context.WithValue(r.Context(), middleware.ModelContextKey, chatReq.Model)
	r = r.WithContext(rCtx)


	// Prompt template injection: prepend matching system prompt templates
	if g.promptManager != nil {
		templates := g.promptManager.FindByRouteMatch(chatReq.Model)
		for _, tmpl := range templates {
			if tmpl.Role == "system" {
				rendered, err := tmpl.Render(nil)
				if err == nil && rendered != "" {
					chatReq.Messages = append([]openai.Message{
						{Role: "system", Content: rendered},
					}, chatReq.Messages...)
					g.logger.Info("prompt template injected",
						"template", tmpl.Name,
						"version", tmpl.CurrentVer,
						"model", chatReq.Model)
				}
			}
		}
	}

	// ---- Plugin: Security ----
	// Run security plugins to check or transform messages
	if g.pluginManager != nil {
		secPlugins := g.pluginManager.SecurityPlugins()
		if len(secPlugins) > 0 {
			pluginMsgs := messagesToPlugin(chatReq.Messages)
			for name, sec := range secPlugins {
				modified, err := sec.CheckPrompt(r.Context(), pluginMsgs)
				if err != nil {
					g.logger.Warn("security plugin blocked request", "plugin", name, "error", err)
					writeError(w, http.StatusForbidden, "Request blocked by security policy")
					return
				}
				if modified != nil {
					pluginMsgs = modified
					g.logger.Info("security plugin modified messages", "plugin", name)
				}
			}
			chatReq.Messages = pluginToMessages(pluginMsgs)
		}
	}

// Semantic routing is handled by middleware

// Semantic cache lookup is handled by middleware

	route := g.router.Match(chatReq.Model)
	var upstreamNames []string
	if route != nil {
		upstreamNames = append([]string{route.Upstream}, route.Fallbacks...)
	} else {
		// ---- Plugin: Router ----
		// Try router plugins for upstream selection
		var pluginUpstream string
		if g.pluginManager != nil {
			routerPlugins := g.pluginManager.RouterPlugins()
			if len(routerPlugins) > 0 {
				pluginMsgs := messagesToPlugin(chatReq.Messages)
				for name, rt := range routerPlugins {
					if upstream, err := rt.SelectUpstream(r.Context(), chatReq.Model, pluginMsgs); err == nil && upstream != "" {
						pluginUpstream = upstream
						g.logger.Info("router plugin selected upstream", "plugin", name, "upstream", upstream)
						break
					}
				}
			}
		}
		if pluginUpstream != "" {
			upstreamNames = []string{pluginUpstream}
		} else if u := g.balancer.Next(); u != nil {
			upstreamNames = []string{u.Name}
		}
	}

	if len(upstreamNames) == 0 {
		writeError(w, http.StatusServiceUnavailable, "No upstream configured")
		return
	}

	// ---- Experiment Engine (v3.0) override ----
	// If an active A/B experiment matches the requested model,
	// override the upstream selection with the experiment's assigned variant.
	type experimentCtx struct {
		expID   string
		variant string
	}
	var expCtx *experimentCtx
	if g.experimentEngine != nil {
		if exp, variant := g.experimentEngine.MatchAndSelect(chatReq.Model); exp != nil && variant != nil {
			upstreamNames = []string{variant.Upstream}
			expCtx = &experimentCtx{expID: exp.ID, variant: variant.Name}
			g.logger.Info("experiment route selected",
				"experiment", exp.Name,
				"variant", variant.Name,
				"upstream", variant.Upstream,
			)
		}
	}

	var lastErr error
	for i, name := range upstreamNames {
		adapter, ok := g.adapters[name]
		if !ok {
			lastErr = fmt.Errorf("no adapter for upstream %q", name)
			continue
		}

		upstream := g.getUpstream(name)
		if upstream == nil {
			lastErr = fmt.Errorf("upstream %q not found in balancer", name)
			continue
		}

		span.SetAttributes(
			attribute.String("ai.provider", upstream.Provider),
			attribute.String("ai.upstream", upstream.Name),
			attribute.Int("ai.fallback_attempt", i),
		)

		if chatReq.Stream {
			lastErr = g.tryStream(w, adapter, &chatReq, upstream)
			if lastErr == nil {
				g.metrics.RecordRequest("POST", "/v1/chat/completions", "200", chatReq.Model, upstream.Provider, time.Since(start))
				return
			}
			g.logger.Warn("stream attempt failed, trying fallback",
				"upstream", name, "attempt", i, "error", lastErr)
			continue
		}

		result, err := adapter.ChatCompletion(&chatReq)
		if err == nil {
			if result.Usage != nil {
				rCtx := context.WithValue(r.Context(), middleware.TokensInContextKey, result.Usage.PromptTokens)
				rCtx = context.WithValue(rCtx, middleware.TokensOutContextKey, result.Usage.CompletionTokens)
				r = r.WithContext(rCtx)
				g.metrics.RecordTokens("prompt", result.Model, upstream.Provider, result.Usage.PromptTokens)
				g.metrics.RecordTokens("completion", result.Model, upstream.Provider, result.Usage.CompletionTokens)
				g.metrics.RecordTokens("total", result.Model, upstream.Provider, result.Usage.TotalTokens)
			if g.costTracker != nil && result.Usage != nil {
				keyName, _ := r.Context().Value(middleware.APIKeyNameContextKey).(string)
				// Cost recording is handled by cost middleware
				// Check budget and send webhook alert if threshold exceeded
				if g.webhookNotifier != nil && g.costTracker.ShouldAlert(keyName) {
					g.webhookNotifier.Send(webhook.Event{
						Type:      webhook.EventCostAlert,
						Timestamp: time.Now(),
						Severity:  "warning",
						Title:     "Budget threshold exceeded for key: " + keyName,
						Message:   fmt.Sprintf("API key \"%s\" has exceeded the alert threshold", keyName),
						Details: map[string]interface{}{
							"key_name": keyName,
							"prompt_tokens": result.Usage.PromptTokens,
							"completion_tokens": result.Usage.CompletionTokens,
						},
					})
				}
			}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			json.NewEncoder(w).Encode(result)
			g.metrics.RecordRequest("POST", "/v1/chat/completions", "200", chatReq.Model, upstream.Provider, time.Since(start))
			// Record experiment metrics
			if expCtx != nil && result.Usage != nil {
				g.experimentEngine.Record(expCtx.expID, expCtx.variant, experiment.Record{
					Latency:          time.Since(start),
					Success:          true,
					PromptTokens:     result.Usage.PromptTokens,
					CompletionTokens: result.Usage.CompletionTokens,
				})
			}
			return
		}

		lastErr = err
		g.logger.Warn("chat completion attempt failed, trying fallback",
			"upstream", name, "attempt", i, "error", err)
		// Record experiment metrics on failure
		if expCtx != nil {
			g.experimentEngine.Record(expCtx.expID, expCtx.variant, experiment.Record{
				Latency: time.Since(start),
				Success: false,
			})
		}
	}

	g.logger.Error("all upstream attempts failed", "error", lastErr)
	writeError(w, http.StatusBadGateway, lastErr.Error())
}

func (g *Gateway) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	g.metrics.IncActiveRequests()
	defer g.metrics.DecActiveRequests()
	start := time.Now()

	span := trace.SpanFromContext(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var embedReq openai.EmbeddingRequest
	if err := json.Unmarshal(body, &embedReq); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	span.SetAttributes(attribute.String("ai.model", embedReq.Model))

	upstream := g.balancer.Next()
	if upstream == nil {
		writeError(w, http.StatusServiceUnavailable, "No healthy upstream available")
		return
	}

	span.SetAttributes(
		attribute.String("ai.provider", upstream.Provider),
		attribute.String("ai.upstream", upstream.Name),
	)

	adapter, ok := g.adapters[upstream.Name]
	if !ok {
		g.proxyDirect(w, r, upstream.Endpoint, body)
		return
	}

	result, err := adapter.Embedding(&embedReq)
	if err != nil {
		g.logger.Error("embedding failed", "error", err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	if result.Usage != nil {
				rCtx := context.WithValue(r.Context(), middleware.TokensInContextKey, result.Usage.PromptTokens)
				rCtx = context.WithValue(rCtx, middleware.TokensOutContextKey, result.Usage.CompletionTokens)
				r = r.WithContext(rCtx)
		g.metrics.RecordTokens("prompt", embedReq.Model, upstream.Provider, result.Usage.PromptTokens)
		g.metrics.RecordTokens("total", embedReq.Model, upstream.Provider, result.Usage.TotalTokens)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
	g.metrics.RecordRequest("POST", "/v1/embeddings", "200", embedReq.Model, upstream.Provider, time.Since(start))
}

func (g *Gateway) tryStream(w http.ResponseWriter, adapter provider.ProviderAdapter, chatReq *openai.ChatCompletionRequest, upstream *balancer.Upstream) error {
	resp, err := adapter.ChatCompletionStream(chatReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, resp.Body)
	return err
}

func (g *Gateway) getUpstream(name string) *balancer.Upstream {
	for _, u := range g.balancer.Upstreams() {
		if u.Name == name {
			return u
		}
	}
	return nil
}

func (g *Gateway) proxyDirect(w http.ResponseWriter, r *http.Request, endpoint string, body []byte) {
	proxyReq, err := http.NewRequest(r.Method, endpoint+r.URL.Path, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create proxy request")
		return
	}
	for k, v := range r.Header {
		proxyReq.Header[k] = v
	}
	httpResp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Upstream request failed")
		return
	}
	defer httpResp.Body.Close()
	for k, v := range httpResp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(httpResp.StatusCode)
	io.Copy(w, httpResp.Body)
}

// NewForTest creates a Gateway for testing without starting the HTTP server.
func NewForTest(cfg *config.Config) (*Gateway, error) {
	return New(cfg, "")
}

// Handler returns the HTTP handler for use with httptest.
func (g *Gateway) Handler() http.Handler {
	logger := initLogger(g.cfg.Log)

	tp, err := tracing.Init("ai-gateway-test")
	if err != nil {
		logger.Warn("failed to init tracing for test", "error", err)
	}

	m := router.NewMatcher(g.cfg.Routes)
	b := balancer.New(logger)

	adapters := make(map[string]provider.ProviderAdapter)
	providerMap := make(map[string]string)

	for _, u := range g.cfg.Upstream {
		b.AddUpstream(u.Name, u.Endpoint, u.Provider, u.Weight)
		switch u.Provider {
		case "openai":
			adapters[u.Name] = openai.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "anthropic":
			adapters[u.Name] = anthropic.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "google", "gemini":
			adapters[u.Name] = google.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		case "azure":
			adapters[u.Name] = azure.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout, u.APIVersion)
		}
		providerMap[u.Name] = u.Provider
	}

	// Initialize Redis (nil if not configured)
	var redisClient *cache.Client
	if g.cfg.Redis.Addr != "" {
		redisClient = cache.New(cache.Config{
			Addr:     g.cfg.Redis.Addr,
			Password: g.cfg.Redis.Password,
			DB:       g.cfg.Redis.DB,
		})
	}

	// Semantic middleware
	var semanticMiddleware *middleware.SemanticMiddleware
	if g.semanticRouter != nil || g.semanticCache != nil {
		semanticMiddleware = middleware.NewSemantic(g.semanticRouter, g.semanticCache, logger)
	}

	// RAG middleware
	var ragMiddleware *middleware.RAGMiddleware
	if g.cfg.RAG.Enabled && g.ragStore != nil {
		ragMiddleware = middleware.NewRAG(g.cfg.RAG, g.ragStore, g.ragEmbedder, logger)
	}

	// Cost middleware
	var costMiddleware *middleware.CostMiddleware
	if g.costTracker != nil {
		costMiddleware = middleware.NewCost(g.costTracker, logger)
	}

	// Security middleware
	var securityMiddleware *middleware.SecurityMiddleware
	if g.cfg.Security.PromptInjection.Enabled || g.cfg.Security.PII.Enabled {
		securityMiddleware = &middleware.SecurityMiddleware{}
	}

	// Initialize metrics collector for the test handler
	reg := prometheus.NewRegistry()
	collector := metrics.NewCollector(reg)

	gw := &Gateway{
		cfg:          g.cfg,
		router:       m,
		balancer:     b,
		logger:       logger,
		adapters:     adapters,
		providerMap:  providerMap,
		tp:           tp,
		redisClient:  redisClient,
		semanticRouter: g.semanticRouter,
		semanticCache:  g.semanticCache,
		prewarmer:      g.prewarmer,
		costTracker:    g.costTracker,
		metrics:        collector,
		auditStore:     g.auditStore,
		promptManager:  g.promptManager,
		webhookNotifier: g.webhookNotifier,
		pluginManager:    g.pluginManager,
		experimentEngine: g.experimentEngine,
		evaluationRunner:  g.evaluationRunner,
		mcpHub:             g.mcpHub,
		ragStore:           g.ragStore,
		ragEmbedder:        g.ragEmbedder,
	}

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.NewRecovery(logger).Middleware)
	if gw.auditStore != nil {
		r.Use(middleware.NewAudit(logger, gw.auditStore).Middleware)
	}
		authMiddleware := middleware.NewAuth(g.cfg.Auth.Enabled, buildKeyMap(g.cfg.Auth.Keys), buildRoleMap(g.cfg.Auth.Keys), logger)
	if g.userStore != nil {
		authMiddleware.SetUserStore(g.userStore)
	}
	r.Use(authMiddleware.Middleware)
	r.Use(middleware.NewAdminPermission().Middleware)
	if securityMiddleware != nil {
		r.Use(securityMiddleware.Middleware)
	}
	if semanticMiddleware != nil {
		r.Use(semanticMiddleware.Middleware)
	}
	if ragMiddleware != nil {
		r.Use(ragMiddleware.Middleware)
	}
	if costMiddleware != nil {
		r.Use(costMiddleware.Middleware)
	}

	r.Get("/admin/health", gw.handleHealth)
	r.Get("/admin/status", gw.handleStatus)
	r.Get("/admin/upstreams", gw.handleListUpstreams)
	r.Get("/admin/routes", gw.handleListRoutes)
	r.Get("/admin/keys", gw.handleListKeys)
	r.Post("/admin/routes", gw.handleReloadRoutes)
	r.Post("/admin/upstreams", gw.handleReloadUpstreams)
	r.Post("/admin/keys", gw.handleAddKey)
	r.Delete("/admin/keys/{key}", gw.handleDeleteKey)
	r.Post("/admin/upstreams/single", gw.handleAddUpstream)
	r.Delete("/admin/upstreams/{name}", gw.handleDeleteUpstream)
	r.Post("/admin/routes/single", gw.handleAddRoute)
	r.Delete("/admin/routes/{id}", gw.handleDeleteRoute)
	r.Get("/admin/config", gw.handleGetConfig)
	r.Put("/admin/config", gw.handleUpdateConfig)
	r.Get("/admin/audit-logs", gw.handleGetAuditLogs)
	r.Get("/admin/cost-stats", gw.handleGetCostStats)
	r.Get("/admin/config/versions", gw.handleConfigVersions)
	r.Post("/admin/config/rollback/{version}", gw.handleConfigRollback)
	r.Get("/admin/health/history", gw.handleHealthHistory)
	r.Get("/admin/plugins", gw.handleListPlugins)
	r.Post("/admin/plugins", gw.handleReloadPlugins)
	r.Get("/admin/webhook", gw.handleGetWebhookConfig)
	r.Put("/admin/webhook", gw.handleUpdateWebhookConfig)
	r.Get("/admin/prompts", gw.handleListPrompts)
	r.Post("/admin/prompts", gw.handleSavePrompt)
	r.Get("/admin/prompts/{id}", gw.handleGetPrompt)
	r.Delete("/admin/prompts/{id}", gw.handleDeletePrompt)
	r.Post("/admin/prompts/{id}/versions", gw.handleAddPromptVersion)
		r.Post("/admin/login", gw.handleLogin)
	r.Get("/admin/users", gw.handleListUsers)
	r.Post("/admin/users", gw.handleCreateUser)
	r.Delete("/admin/users/{username}", gw.handleDeleteUser)
	r.Put("/admin/users/{username}/role", gw.handleUpdateUserRole)
	r.Post("/admin/keys/{key}/rotate", gw.handleRotateKey)
	r.Get("/admin/security/policies", gw.handleGetSecurityPolicies)
	r.Put("/admin/security/policies", gw.handleUpdateSecurityPolicies)

	r.Get("/admin/experiments", gw.handleListExperiments)
	r.Post("/admin/experiments", gw.handleCreateExperiment)
	r.Get("/admin/experiments/{id}", gw.handleGetExperiment)
	r.Put("/admin/experiments/{id}", gw.handleUpdateExperiment)
	r.Delete("/admin/experiments/{id}", gw.handleDeleteExperiment)
	r.Get("/admin/experiments/{id}/results", gw.handleExperimentResults)
	r.Post("/admin/experiments/{id}/start", gw.handleStartExperiment)
	r.Post("/admin/experiments/{id}/stop", gw.handleStopExperiment)

	r.Get("/admin/evaluations", gw.handleListEvalSuites)
	r.Post("/admin/evaluations", gw.handleCreateEvalSuite)
	r.Get("/admin/evaluations/{id}", gw.handleGetEvalSuite)
	r.Delete("/admin/evaluations/{id}", gw.handleDeleteEvalSuite)
	r.Post("/admin/evaluations/{id}/run", gw.handleRunEvaluation)
	r.Get("/admin/evaluations/{id}/runs", gw.handleListEvalRuns)
	r.Get("/admin/evaluations/{id}/runs/{runId}", gw.handleGetEvalRun)
	r.Post("/admin/evaluations/{id}/runs/{runId}/cancel", gw.handleCancelEvalRun)
	r.Post("/admin/mcp/servers", gw.handleAddMCPServer)
	r.Get("/admin/mcp/servers", gw.handleListMCPServers)
	r.Get("/admin/mcp/servers/{name}", gw.handleGetMCPServer)
	r.Put("/admin/mcp/servers/{name}", gw.handleUpdateMCPServer)
	r.Delete("/admin/mcp/servers/{name}", gw.handleDeleteMCPServer)
		r.Get("/admin/cache/stats", gw.handleCacheStats)
	r.Post("/admin/cache/prewarm", gw.handleCachePrewarm)
	r.Get("/admin/cache/prewarm/stats", gw.handleCachePrewarmStats)
r.Get("/admin/mcp/tools", gw.handleListMCPTools)
	r.Post("/admin/mcp/servers/{name}/reconnect", gw.handleReconnectMCPServer)
	r.Post("/admin/mcp/reload", gw.handleReloadMCP)
	r.Get("/admin/rag/knowledge-bases", gw.handleListKnowledgeBases)
	r.Post("/admin/rag/knowledge-bases", gw.handleCreateKnowledgeBase)
	r.Get("/admin/rag/knowledge-bases/{id}", gw.handleGetKnowledgeBase)
	r.Delete("/admin/rag/knowledge-bases/{id}", gw.handleDeleteKnowledgeBase)
	r.Post("/admin/rag/knowledge-bases/{id}/ingest", gw.handleIngestDocument)
	r.Post("/admin/rag/knowledge-bases/{id}/ingest-batch", gw.handleIngestBatch)
	r.Get("/admin/rag/knowledge-bases/{id}/stats", gw.handleKBStats)
	r.Delete("/admin/rag/knowledge-bases/{id}/chunks/{chunkID}", gw.handleDeleteChunk)

	r.Post("/v1/chat/completions", gw.handleChatCompletion)
	r.Post("/v1/embeddings", gw.handleEmbeddings)
	r.Get("/openapi.yaml", gw.handleOpenAPISpec)
	r.Get("/metrics", gw.handleMetrics)

	return r
}

func mcpError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": map[string]interface{}{
			"code":    -32000,
			"message": message,
		},
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{"message": message, "type": "gateway_error"},
	})
}

func buildKeyMap(keys []config.APIKey) map[string]string {
	m := make(map[string]string)
	for _, k := range keys {
		m[k.Key] = k.Name
	}
	return m
}

func buildRoleMap(keys []config.APIKey) map[string][]string {
	m := make(map[string][]string)
	for _, k := range keys {
		m[k.Name] = k.Roles
	}
	return m
}

func initLogger(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}



func (g *Gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if g.metrics != nil {
		g.metrics.Handler().ServeHTTP(w, r)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# No metrics collector configured\n"))
	}
}


// messagesToPlugin converts openai.Message slice to plugin-friendly []map[string]interface{}.
func messagesToPlugin(msgs []openai.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, len(msgs))
	for i, m := range msgs {
		result[i] = map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		}
	}
	return result
}

// pluginToMessages converts plugin-friendly messages back to openai.Message slice.
func pluginToMessages(msgs []map[string]interface{}) []openai.Message {
	result := make([]openai.Message, len(msgs))
	for i, m := range msgs {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		result[i] = openai.Message{Role: role, Content: content}
	}
	return result
}






