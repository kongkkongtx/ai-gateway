package evaluation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kongkkongtx/ai-gateway/internal/provider"
	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
	"github.com/kongkkongtx/ai-gateway/internal/semantic"
)

// Judge defines the interface for scoring AI responses.
type Judge interface {
	// Score evaluates a response against a reference answer.
	// Returns a score between 0.0 and 1.0, optional feedback text, and any error.
	Score(response, reference string) (float64, string, error)
}

// ExactMatchJudge performs exact string comparison after normalization.
type ExactMatchJudge struct {
	CaseSensitive bool
}

func (j *ExactMatchJudge) Score(response, reference string) (float64, string, error) {
	a, b := response, reference
	if !j.CaseSensitive {
		a = strings.ToLower(strings.TrimSpace(a))
		b = strings.ToLower(strings.TrimSpace(b))
	}
	if a == b {
		return 1.0, "exact match", nil
	}
	return 0.0, "no match", nil
}

// SemanticJudge uses embedding similarity to score responses.
type SemanticJudge struct {
	embedder *semantic.EmbeddingClient
}

func NewSemanticJudge(embedder *semantic.EmbeddingClient) *SemanticJudge {
	return &SemanticJudge{embedder: embedder}
}

func (j *SemanticJudge) Score(response, reference string) (float64, string, error) {
	respEmb, err := j.embedder.Embed(response)
	if err != nil {
		return 0, "", fmt.Errorf("embedding response: %w", err)
	}
	refEmb, err := j.embedder.Embed(reference)
	if err != nil {
		return 0, "", fmt.Errorf("embedding reference: %w", err)
	}

	if len(respEmb.Data) == 0 || len(refEmb.Data) == 0 {
		return 0, "", fmt.Errorf("embedding returned empty data")
	}

	score := semantic.CosineSimilarity(respEmb.Data[0].Embedding, refEmb.Data[0].Embedding)
	return score, fmt.Sprintf("semantic similarity: %.4f", score), nil
}

// LLMJudge uses a language model to evaluate response quality.
type LLMJudge struct {
	adapter provider.ProviderAdapter
	model   string
}

func NewLLMJudge(adapter provider.ProviderAdapter, model string) *LLMJudge {
	return &LLMJudge{adapter: adapter, model: model}
}

var scoreRegex = regexp.MustCompile(`(\d+\.?\d*)`)

// judgePrompt is the template used to ask the LLM judge to score a response.
const judgePrompt = `You are an AI response evaluator. Rate how well the following response matches the reference answer on a scale of 0.0 to 1.0, where:
- 1.0 = Perfect match (same meaning, similar wording)
- 0.7 = Good match (same meaning, different wording)
- 0.4 = Partial match (some relevant content)
- 0.0 = No match (completely different)

Output ONLY a single number between 0.0 and 1.0, nothing else.

Reference answer: %s

Response to evaluate: %s

Score (0.0-1.0):`

func (j *LLMJudge) Score(response, reference string) (float64, string, error) {
	prompt := fmt.Sprintf(judgePrompt, reference, response)
	resp, err := j.adapter.ChatCompletion(&openai.ChatCompletionRequest{
		Model: j.model,
		Messages: []openai.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return 0, "", fmt.Errorf("judge LLM call failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return 0, "", fmt.Errorf("judge returned no choices")
	}

	text := openai.ExtractText(resp.Choices[0].Message.Content)
	matches := scoreRegex.FindString(text)
	if matches == "" {
		return 0, fmt.Sprintf("could not parse score from: %s", text), nil
	}

	var score float64
	if _, err := fmt.Sscanf(matches, "%f", &score); err != nil {
		return 0, fmt.Sprintf("could not parse score from: %s", text), nil
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score, fmt.Sprintf("llm judge score: %.4f", score), nil
}

// NewJudge creates a Judge from the given config and dependencies.
func NewJudge(cfg JudgeConfig, adapters map[string]provider.ProviderAdapter, embedder *semantic.EmbeddingClient) (Judge, error) {
	switch cfg.Type {
	case JudgeExactMatch:
		return &ExactMatchJudge{}, nil

	case JudgeSemantic:
		if embedder == nil {
			return nil, fmt.Errorf("semantic judge requires an embedding client")
		}
		return NewSemanticJudge(embedder), nil

	case JudgeLLM:
		if cfg.JudgeUpstream == "" {
			return nil, fmt.Errorf("llm judge requires judge_upstream")
		}
		adapter, ok := adapters[cfg.JudgeUpstream]
		if !ok {
			return nil, fmt.Errorf("judge upstream %q not found in adapters", cfg.JudgeUpstream)
		}
		model := cfg.JudgeModel
		if model == "" {
			model = "gpt-4o-mini"
		}
		return NewLLMJudge(adapter, model), nil

	default:
		return nil, fmt.Errorf("unknown judge type: %s", cfg.Type)
	}
}
