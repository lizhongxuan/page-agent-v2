package selector

import "time"

type RunResult string

const (
	RunResultSuccess  RunResult = "success"
	RunResultFailed   RunResult = "failed"
	RunResultFallback RunResult = "fallback"
)

type WorkflowRun struct {
	ID              string         `json:"id"`
	WorkflowID      string         `json:"workflowId,omitempty"`
	WorkflowVersion int            `json:"workflowVersion,omitempty"`
	ProjectID       string         `json:"projectId"`
	Task            string         `json:"task"`
	URL             string         `json:"url"`
	VariablesUsed   map[string]any `json:"variablesUsed,omitempty"`
	Result          RunResult      `json:"result"`
	FailedChunkID   string         `json:"failedChunkId,omitempty"`
	FailedStepID    string         `json:"failedStepId,omitempty"`
	FallbackReason  string         `json:"fallbackReason,omitempty"`
	DurationMS      int            `json:"durationMs,omitempty"`
	ArtifactRefs    []string       `json:"artifactRefs,omitempty"`
	CreatedAt       time.Time      `json:"createdAt,omitempty"`
}

type SelectorStatUpdate struct {
	WorkflowID      string `json:"workflowId"`
	WorkflowVersion int    `json:"workflowVersion"`
	StepID          string `json:"stepId"`
	Strategy        string `json:"strategy"`
	Selector        string `json:"selector"`
	Success         bool   `json:"success"`
	FailureReason   string `json:"failureReason,omitempty"`
}

type SelectorStat struct {
	WorkflowID        string    `json:"workflowId"`
	WorkflowVersion   int       `json:"workflowVersion"`
	StepID            string    `json:"stepId"`
	Strategy          string    `json:"strategy"`
	Selector          string    `json:"selector"`
	SuccessCount      int       `json:"successCount"`
	FailCount         int       `json:"failCount"`
	LastSuccessAt     time.Time `json:"lastSuccessAt,omitempty"`
	LastFailAt        time.Time `json:"lastFailAt,omitempty"`
	LastFailureReason string    `json:"lastFailureReason,omitempty"`
}
