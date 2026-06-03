package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kongkkongtx/ai-gateway/internal/config"
	"github.com/kongkkongtx/ai-gateway/internal/experiment"
)

// JSON-friendly experiment request.
type experimentReq struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Model       string              `json:"model"`
	Variants    []experimentReqVariant `json:"variants"`
	Active      bool                `json:"active"`
}

type experimentReqVariant struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
	Weight   int    `json:"weight"`
}

func (r *experimentReq) toExperiment() experiment.Experiment {
	variants := make([]experiment.Variant, len(r.Variants))
	for i, v := range r.Variants {
		variants[i] = experiment.Variant{
			Name:     v.Name,
			Upstream: v.Upstream,
			Weight:   v.Weight,
		}
	}
	return experiment.Experiment{
		Name:        r.Name,
		Description: r.Description,
		Model:       r.Model,
		Variants:    variants,
		Active:      r.Active,
	}
}

func (g *Gateway) handleListExperiments(w http.ResponseWriter, r *http.Request) {
	if g.experimentEngine == nil {
		writeJSON(w, http.StatusOK, []experiment.Experiment{})
		return
	}
	writeJSON(w, http.StatusOK, g.experimentEngine.ListExperiments())
}

func (g *Gateway) handleCreateExperiment(w http.ResponseWriter, r *http.Request) {
	if g.experimentEngine == nil {
		writeError(w, http.StatusBadRequest, "Experiment engine is not enabled")
		return
	}

	var req experimentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate upstream references
	seenUpstreams := make(map[string]bool)
	for _, u := range g.cfg.Upstream {
		seenUpstreams[u.Name] = true
	}
	for _, v := range req.Variants {
		if !seenUpstreams[v.Upstream] {
			writeError(w, http.StatusBadRequest, "Variant references unknown upstream: "+v.Upstream)
			return
		}
	}

	exp := req.toExperiment()

	if err := g.experimentEngine.AddExperiment(exp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Persist the full experiment list
	g.cfg.Experiment.Experiments = g.experimentEngine.ListExperiments()
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "Experiment created: "+exp.Name); err != nil {
		g.logger.Warn("failed to persist experiment config", "error", err)
	}

	g.logger.Info("experiment created", "id", exp.ID, "name", exp.Name)
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "ok",
		"id":     exp.ID,
	})
}

func (g *Gateway) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	if g.experimentEngine == nil {
		writeError(w, http.StatusNotFound, "Experiment engine not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	exp := g.experimentEngine.GetExperiment(id)
	if exp == nil {
		writeError(w, http.StatusNotFound, "Experiment not found")
		return
	}
	writeJSON(w, http.StatusOK, exp)
}

func (g *Gateway) handleUpdateExperiment(w http.ResponseWriter, r *http.Request) {
	if g.experimentEngine == nil {
		writeError(w, http.StatusBadRequest, "Experiment engine is not enabled")
		return
	}
	id := chi.URLParam(r, "id")

	var req experimentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate upstream references
	seenUpstreams := make(map[string]bool)
	for _, u := range g.cfg.Upstream {
		seenUpstreams[u.Name] = true
	}
	for _, v := range req.Variants {
		if !seenUpstreams[v.Upstream] {
			writeError(w, http.StatusBadRequest, "Variant references unknown upstream: "+v.Upstream)
			return
		}
	}

	exp := req.toExperiment()
	exp.ID = id

	if err := g.experimentEngine.UpdateExperiment(exp); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Persist
	g.cfg.Experiment.Experiments = g.experimentEngine.ListExperiments()
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "Experiment updated: "+exp.Name); err != nil {
		g.logger.Warn("failed to persist experiment config", "error", err)
	}

	g.logger.Info("experiment updated", "id", id, "name", exp.Name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleDeleteExperiment(w http.ResponseWriter, r *http.Request) {
	if g.experimentEngine == nil {
		writeError(w, http.StatusNotFound, "Experiment engine not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	if !g.experimentEngine.RemoveExperiment(id) {
		writeError(w, http.StatusNotFound, "Experiment not found")
		return
	}

	g.cfg.Experiment.Experiments = g.experimentEngine.ListExperiments()
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "Experiment deleted: "+id); err != nil {
		g.logger.Warn("failed to persist experiment config", "error", err)
	}

	g.logger.Info("experiment deleted", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleExperimentResults(w http.ResponseWriter, r *http.Request) {
	if g.experimentEngine == nil {
		writeError(w, http.StatusNotFound, "Experiment engine not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	res := g.experimentEngine.GetResults(id)
	if res == nil {
		writeError(w, http.StatusNotFound, "Experiment not found")
		return
	}

	// Check significance
	sigMsg := g.experimentEngine.CheckSignificance(id)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results":            res,
		"significance_alert": sigMsg,
	})
}

func (g *Gateway) handleStartExperiment(w http.ResponseWriter, r *http.Request) {
	if g.experimentEngine == nil {
		writeError(w, http.StatusBadRequest, "Experiment engine is not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	if err := g.experimentEngine.StartExperiment(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	g.cfg.Experiment.Experiments = g.experimentEngine.ListExperiments()
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "Experiment started: "+id); err != nil {
		g.logger.Warn("failed to persist experiment config", "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleStopExperiment(w http.ResponseWriter, r *http.Request) {
	if g.experimentEngine == nil {
		writeError(w, http.StatusBadRequest, "Experiment engine is not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	if err := g.experimentEngine.StopExperiment(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	g.cfg.Experiment.Experiments = g.experimentEngine.ListExperiments()
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "Experiment stopped: "+id); err != nil {
		g.logger.Warn("failed to persist experiment config", "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
