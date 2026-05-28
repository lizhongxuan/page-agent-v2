package workflow

import "time"

type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusDisabled WorkflowStatus = "disabled"
)

type WorkflowRecipe struct {
	ID              string          `json:"id"`
	ProjectID       string          `json:"projectId"`
	Site            string          `json:"site"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Intent          string          `json:"intent"`
	Tags            []string        `json:"tags,omitempty"`
	URLPatterns     []string        `json:"urlPatterns,omitempty"`
	PageFingerprint PageFingerprint `json:"pageFingerprint"`
	Variables       []Variable      `json:"variables,omitempty"`
	Chunks          []WorkflowChunk `json:"chunks"`
	SafetyPolicy    SafetyPolicy    `json:"safetyPolicy"`
	Status          WorkflowStatus  `json:"status"`
	Version         int             `json:"version"`
	CreatedAt       time.Time       `json:"createdAt,omitempty"`
	UpdatedAt       time.Time       `json:"updatedAt,omitempty"`
}

type WorkflowChunk struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	RiskLevel   RiskLevel      `json:"riskLevel"`
	Steps       []WorkflowStep `json:"steps"`
	SuccessWhen []SuccessRule  `json:"successWhen,omitempty"`
}

type StepType string

const (
	StepTypeClick  StepType = "click"
	StepTypeInput  StepType = "input"
	StepTypeSelect StepType = "select"
	StepTypeScroll StepType = "scroll"
	StepTypeWait   StepType = "wait"
)

type WorkflowStep struct {
	ID          string     `json:"id"`
	Type        StepType   `json:"type"`
	Target      StepTarget `json:"target,omitempty"`
	Value       string     `json:"value,omitempty"`
	Description string     `json:"description,omitempty"`
}

type StepTarget struct {
	Preferred TargetCandidate   `json:"preferred,omitempty"`
	Fallbacks []TargetCandidate `json:"fallbacks,omitempty"`
}

type TargetStrategy string

const (
	TargetStrategyTestID      TargetStrategy = "test_id"
	TargetStrategyRole        TargetStrategy = "role"
	TargetStrategyLabel       TargetStrategy = "label"
	TargetStrategyPlaceholder TargetStrategy = "placeholder"
	TargetStrategyText        TargetStrategy = "text"
	TargetStrategyCSS         TargetStrategy = "css"
	TargetStrategyDOMIndex    TargetStrategy = "dom_index"
)

type TargetCandidate struct {
	Strategy  TargetStrategy `json:"strategy,omitempty"`
	Value     string         `json:"value,omitempty"`
	Role      string         `json:"role,omitempty"`
	Name      string         `json:"name,omitempty"`
	NearText  string         `json:"nearText,omitempty"`
	Container string         `json:"container,omitempty"`
	Index     int            `json:"index,omitempty"`
}

type SuccessRule struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
