package selector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatibleSelector struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAICompatibleSelector(baseURL string, apiKey string, model string, httpClient *http.Client) *OpenAICompatibleSelector {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}
	return &OpenAICompatibleSelector{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: httpClient,
	}
}

func (selector *OpenAICompatibleSelector) SelectWorkflow(ctx context.Context, prompt SelectionPrompt) (Selection, error) {
	if selector.baseURL == "" {
		return Selection{}, errors.New("llm base url is required")
	}
	payload, err := json.Marshal(map[string]any{
		"model": selector.model,
		"messages": []map[string]string{
			{
				"role": "system",
				"content": strings.Join([]string{
					"You select a browser workflow replay candidate for the user's task.",
					"Return JSON only with fields: selectedWorkflowId, selectedVersion, confidence, bindings, decision, needsUserConfirmation, rejectReason.",
					"Use decision replay only when a candidate clearly matches. Use no_match, unsafe_risk, or binding_failed otherwise.",
					"Bindings must contain only non-sensitive workflow variables from the user task or current page context.",
				}, "\n"),
			},
			{
				"role":    "user",
				"content": selectionPromptJSON(prompt),
			},
		},
		"temperature":     0,
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return Selection{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, selector.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Selection{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if selector.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+selector.apiKey)
	}
	response, err := selector.httpClient.Do(request)
	if err != nil {
		return Selection{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return Selection{}, fmt.Errorf("selector request failed with status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return Selection{}, err
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return Selection{}, errors.New("selector response did not contain message content")
	}
	var selection Selection
	if err := json.Unmarshal([]byte(decoded.Choices[0].Message.Content), &selection); err != nil {
		return Selection{}, fmt.Errorf("decode selector JSON: %w", err)
	}
	return selection, nil
}

func selectionPromptJSON(prompt SelectionPrompt) string {
	content, err := json.Marshal(prompt)
	if err != nil {
		return "{}"
	}
	return string(content)
}

type HeuristicSelector struct{}

func (HeuristicSelector) SelectWorkflow(_ context.Context, prompt SelectionPrompt) (Selection, error) {
	if len(prompt.Candidates) == 0 {
		return Selection{Decision: "no_match", RejectReason: "no candidates"}, nil
	}
	candidate := prompt.Candidates[0]
	confidence, needsConfirmation := heuristicConfidence(candidate)
	if confidence == 0 {
		return Selection{
			Decision:     "no_match",
			RejectReason: "top candidate does not have enough retrieval evidence",
		}, nil
	}
	return Selection{
		SelectedWorkflowID:    candidate.WorkflowID,
		SelectedVersion:       candidate.Version,
		Confidence:            confidence,
		Bindings:              map[string]string{},
		NeedsUserConfirmation: needsConfirmation,
		Decision:              "replay",
	}, nil
}

func heuristicConfidence(candidate CandidateSummary) (float64, bool) {
	score := candidate.Retrieval.FinalScore
	breakdown := candidate.Retrieval.ScoreBreakdown
	bestEvidence := maxFloat(
		breakdown.WorkflowDenseScore,
		breakdown.WorkflowSparseScore,
		breakdown.BestChunkScore,
		breakdown.StepCoverage,
		breakdown.PageStateScore,
	)
	switch {
	case score >= 0.3 || bestEvidence >= 0.35:
		return 0.86, false
	case score >= 0.2 || bestEvidence >= 0.2:
		return 0.76, true
	default:
		return 0, false
	}
}

func maxFloat(values ...float64) float64 {
	max := 0.0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}
