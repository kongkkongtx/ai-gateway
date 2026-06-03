package server

import (
	"net/http"
)

func (g *Gateway) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	if g.semanticCache == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"message": "Semantic cache not configured",
		})
		return
	}
	stats := g.semanticCache.Stats()
	writeJSON(w, http.StatusOK, stats)
}

func (g *Gateway) handleCachePrewarm(w http.ResponseWriter, r *http.Request) {
	if g.prewarmer == nil {
		writeError(w, http.StatusBadRequest, "Cache prewarmer not initialized")
		return
	}
	g.prewarmer.Run(r.Context())
	stats := g.prewarmer.Stats()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"stats":  stats,
	})
}

func (g *Gateway) handleCachePrewarmStats(w http.ResponseWriter, r *http.Request) {
	if g.prewarmer == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
		})
		return
	}
	stats := g.prewarmer.Stats()
	writeJSON(w, http.StatusOK, stats)
}
