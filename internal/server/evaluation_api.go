package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kongkkongtx/ai-gateway/internal/config"
	"github.com/kongkkongtx/ai-gateway/internal/evaluation"
)

// JSON-friendly evaluation suite request.
type evalSuiteReq struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	TestPrompts []evalTestPromptReq     `json:"test_prompts"`
	TargetModels []evalTargetModelReq   `json:"target_models"`
	Judge       evaluation.JudgeConfig `json:"judge"`
}

type evalTestPromptReq struct {
	ID              string `json:"id"`
	Prompt          string `json:"prompt"`
	ReferenceAnswer string `json:"reference_answer,omitempty"`
}

type evalTargetModelReq struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
	Model    string `json:"model"`
}

func (r *evalSuiteReq) toSuite() evaluation.EvalSuite {
	prompts := make([]evaluation.TestPrompt, len(r.TestPrompts))
	for i, p := range r.TestPrompts {
		prompts[i] = evaluation.TestPrompt{
			ID:              p.ID,
			Prompt:          p.Prompt,
			ReferenceAnswer: p.ReferenceAnswer,
		}
	}
	models := make([]evaluation.TargetModel, len(r.TargetModels))
	for i, m := range r.TargetModels {
		models[i] = evaluation.TargetModel{
			Name:     m.Name,
			Upstream: m.Upstream,
			Model:    m.Model,
		}
	}
	return evaluation.EvalSuite{
		Name:        r.Name,
		Description: r.Description,
		TestPrompts: prompts,
		TargetModels: models,
		Judge:       r.Judge,
	}
}

func (g *Gateway) handleListEvalSuites(w http.ResponseWriter, r *http.Request) {
	if g.evaluationRunner == nil {
		writeJSON(w, http.StatusOK, []evaluation.EvalSuite{})
		return
	}
	writeJSON(w, http.StatusOK, g.cfg.Evaluation.Suites)
}

func (g *Gateway) handleCreateEvalSuite(w http.ResponseWriter, r *http.Request) {
	if g.evaluationRunner == nil {
		writeError(w, http.StatusBadRequest, "Evaluation engine is not enabled")
		return
	}

	var req evalSuiteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Suite name is required")
		return
	}
	if len(req.TestPrompts) == 0 {
		writeError(w, http.StatusBadRequest, "At least one test prompt is required")
		return
	}
	if len(req.TargetModels) == 0 {
		writeError(w, http.StatusBadRequest, "At least one target model is required")
		return
	}

	// Validate upstream references
	seenUpstreams := make(map[string]bool)
	for _, u := range g.cfg.Upstream {
		seenUpstreams[u.Name] = true
	}
	for _, m := range req.TargetModels {
		if !seenUpstreams[m.Upstream] {
			writeError(w, http.StatusBadRequest, "Target model references unknown upstream: "+m.Upstream)
			return
		}
	}

	suite := req.toSuite()
	id := "eval-" + time.Now().Format("20060102-150405")
	suite.ID = id
	suite.CreatedAt = time.Now()
	suite.UpdatedAt = time.Now()

	g.cfg.Evaluation.Suites = append(g.cfg.Evaluation.Suites, suite)
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "Evaluation suite created: "+suite.Name); err != nil {
		g.logger.Warn("failed to persist evaluation config", "error", err)
	}

	g.logger.Info("evaluation suite created", "id", id, "name", suite.Name)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok", "id": id})
}

func (g *Gateway) handleGetEvalSuite(w http.ResponseWriter, r *http.Request) {
	if g.evaluationRunner == nil {
		writeError(w, http.StatusNotFound, "Evaluation engine not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	for _, s := range g.cfg.Evaluation.Suites {
		if s.ID == id {
			writeJSON(w, http.StatusOK, s)
			return
		}
	}
	writeError(w, http.StatusNotFound, "Evaluation suite not found")
}

func (g *Gateway) handleDeleteEvalSuite(w http.ResponseWriter, r *http.Request) {
	if g.evaluationRunner == nil {
		writeError(w, http.StatusNotFound, "Evaluation engine not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	suites := g.cfg.Evaluation.Suites
	for i, s := range suites {
		if s.ID == id {
			g.cfg.Evaluation.Suites = append(suites[:i], suites[i+1:]...)
			if _, err := config.SaveVersioned(g.cfg, g.configPath, "Evaluation suite deleted: "+id); err != nil {
				g.logger.Warn("failed to persist evaluation config", "error", err)
			}
			g.logger.Info("evaluation suite deleted", "id", id)
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
	}
	writeError(w, http.StatusNotFound, "Evaluation suite not found")
}

func (g *Gateway) handleRunEvaluation(w http.ResponseWriter, r *http.Request) {
	if g.evaluationRunner == nil {
		writeError(w, http.StatusBadRequest, "Evaluation engine is not enabled")
		return
	}
	id := chi.URLParam(r, "id")

	var suite *evaluation.EvalSuite
	for _, s := range g.cfg.Evaluation.Suites {
		if s.ID == id {
			suite = &s
			break
		}
	}
	if suite == nil {
		writeError(w, http.StatusNotFound, "Evaluation suite not found")
		return
	}

	runID, err := g.evaluationRunner.Execute(*suite)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	g.logger.Info("evaluation run triggered", "suite_id", id, "run_id", runID)
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
		"run_id": runID,
	})
}

func (g *Gateway) handleListEvalRuns(w http.ResponseWriter, r *http.Request) {
	if g.evaluationRunner == nil {
		writeJSON(w, http.StatusOK, []evaluation.RunSummary{})
		return
	}
	id := chi.URLParam(r, "id")
	runs := g.evaluationRunner.Store().ListRuns(id, 10)
	writeJSON(w, http.StatusOK, runs)
}

func (g *Gateway) handleGetEvalRun(w http.ResponseWriter, r *http.Request) {
	if g.evaluationRunner == nil {
		writeError(w, http.StatusNotFound, "Evaluation engine not enabled")
		return
	}
	runID := chi.URLParam(r, "runId")
	summary := g.evaluationRunner.Store().GetRun(runID)
	if summary == nil {
		writeError(w, http.StatusNotFound, "Evaluation run not found")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (g *Gateway) handleCancelEvalRun(w http.ResponseWriter, r *http.Request) {
	if g.evaluationRunner == nil {
		writeError(w, http.StatusBadRequest, "Evaluation engine is not enabled")
		return
	}
	runID := chi.URLParam(r, "runId")
	if !g.evaluationRunner.Cancel(runID) {
		writeError(w, http.StatusNotFound, "Evaluation run not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}
