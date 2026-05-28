package retrieval

import "github.com/page-agent/workflow-backend/internal/registry"

type SearchRequest struct {
	ProjectID       string          `json:"projectId,omitempty"`
	Task            string          `json:"task"`
	CurrentURL      string          `json:"currentUrl,omitempty"`
	PageObservation PageObservation `json:"pageObservation,omitempty"`
	RiskPolicy      RiskPolicy      `json:"riskPolicy,omitempty"`
	Limit           int             `json:"limit,omitempty"`
}

type PageObservation struct {
	Title       string    `json:"title,omitempty"`
	VisibleText []string  `json:"visibleText,omitempty"`
	Controls    []Control `json:"controls,omitempty"`
}

type Control struct {
	Role string `json:"role"`
	Name string `json:"name"`
}

type RiskPolicy struct {
	AutoAllowed          []registry.RiskLevel `json:"autoAllowed,omitempty"`
	ConfirmationRequired []registry.RiskLevel `json:"confirmationRequired,omitempty"`
	Blocked              []registry.RiskLevel `json:"blocked,omitempty"`
}

type NormalizedRequest struct {
	ProjectID           string
	Task                string
	CurrentURL          string
	Site                string
	App                 string
	CandidateSlots      map[string]string
	PageFingerprintText string
	RiskPolicy          RiskPolicy
	Limit               int
}

type WorkflowCandidate struct {
	WorkflowID     string             `json:"workflowId"`
	Version        int                `json:"version"`
	Name           string             `json:"name,omitempty"`
	Intent         string             `json:"intent,omitempty"`
	FinalScore     float64            `json:"finalScore"`
	RiskLevel      registry.RiskLevel `json:"riskLevel"`
	Variables      []string           `json:"variables"`
	Reasons        []string           `json:"reasons"`
	ScoreBreakdown ScoreBreakdown     `json:"scoreBreakdown,omitempty"`
}

type ScoreBreakdown struct {
	WorkflowDenseScore    float64 `json:"workflowDenseScore"`
	WorkflowSparseScore   float64 `json:"workflowSparseScore"`
	BestChunkScore        float64 `json:"bestChunkScore"`
	StepCoverage          float64 `json:"stepCoverage"`
	PageStateScore        float64 `json:"pageStateScore"`
	VariableBindability   float64 `json:"variableBindability"`
	HistoricalSuccessRate float64 `json:"historicalSuccessRate"`
	SelectorHealth        float64 `json:"selectorHealth"`
	RiskPenalty           float64 `json:"riskPenalty"`
	RecentFailurePenalty  float64 `json:"recentFailurePenalty"`
}
