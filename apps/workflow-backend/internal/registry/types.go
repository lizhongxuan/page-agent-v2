package registry

import (
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusPendingReview Status = "pending_review"
	StatusActive        Status = "active"
	StatusDisabled      Status = "disabled"
	StatusArchived      Status = "archived"
	StatusDeleted       Status = "deleted"
	StatusIndexed       Status = "indexed"
	StatusIndexFailed   Status = "index_failed"
	StatusRejected      Status = "rejected"
)

type RiskLevel string

const (
	RiskReadOnly     RiskLevel = "read_only"
	RiskReadOrSearch RiskLevel = "read_or_search"
	RiskDraftChange  RiskLevel = "draft_change"
	RiskExternalSend RiskLevel = "external_send"
	RiskDestructive  RiskLevel = "destructive"
)

type VariableType string

const (
	VariableString  VariableType = "string"
	VariableNumber  VariableType = "number"
	VariableBoolean VariableType = "boolean"
)

type VariableSource string

const (
	VariableSourceTask      VariableSource = "task"
	VariableSourceURL       VariableSource = "url"
	VariableSourceTaskOrURL VariableSource = "task_or_url"
	VariableSourcePageState VariableSource = "page_state"
	VariableSourceUser      VariableSource = "user"
)

type StepType string

const (
	StepClick  StepType = "click"
	StepFill   StepType = "fill"
	StepPress  StepType = "press"
	StepSelect StepType = "select"
	StepWait   StepType = "wait"
)

type TargetStrategy string

const (
	TargetRole        TargetStrategy = "role"
	TargetLabel       TargetStrategy = "label"
	TargetPlaceholder TargetStrategy = "placeholder"
	TargetTestID      TargetStrategy = "test_id"
	TargetText        TargetStrategy = "text"
	TargetCSS         TargetStrategy = "css"
	TargetXPath       TargetStrategy = "xpath"
)

type Variable struct {
	Name      string         `json:"name"`
	Type      VariableType   `json:"type"`
	Required  bool           `json:"required"`
	Source    VariableSource `json:"source"`
	Examples  []string       `json:"examples,omitempty"`
	Sensitive bool           `json:"sensitive,omitempty"`
}

type TargetCandidate struct {
	Strategy TargetStrategy `json:"strategy"`
	Value    string         `json:"value,omitempty"`
	Role     string         `json:"role,omitempty"`
	Name     string         `json:"name,omitempty"`
}

func (target TargetCandidate) Text() string {
	return strings.TrimSpace(strings.Join([]string{
		string(target.Strategy),
		target.Value,
		target.Role,
		target.Name,
	}, " "))
}

type StepTarget struct {
	Primary   TargetCandidate   `json:"primary"`
	Fallbacks []TargetCandidate `json:"fallbacks,omitempty"`
}

type WorkflowStep struct {
	ID        string     `json:"id"`
	Type      StepType   `json:"type"`
	Target    StepTarget `json:"target,omitempty"`
	Value     string     `json:"value,omitempty"`
	Key       string     `json:"key,omitempty"`
	RiskLevel RiskLevel  `json:"riskLevel"`
}

type WorkflowChunk struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	FromPageState      string         `json:"fromPageState,omitempty"`
	ToPageState        string         `json:"toPageState,omitempty"`
	PreconditionText   string         `json:"preconditionText,omitempty"`
	PostconditionText  string         `json:"postconditionText,omitempty"`
	StepSummary        string         `json:"stepSummary,omitempty"`
	RiskLevel          RiskLevel      `json:"riskLevel"`
	Steps              []WorkflowStep `json:"steps"`
	VariableNames      []string       `json:"variableNames,omitempty"`
	SuccessRate        float64        `json:"successRate,omitempty"`
	SelectorHealth     float64        `json:"selectorHealth,omitempty"`
	RecentFailureCount int            `json:"recentFailureCount,omitempty"`
	LastFailureAt      time.Time      `json:"lastFailureAt,omitempty"`
}

type WorkflowRecipe struct {
	ID                   string          `json:"id"`
	Version              int             `json:"version"`
	ProjectID            string          `json:"projectId"`
	TenantID             string          `json:"tenantId,omitempty"`
	Status               Status          `json:"status"`
	Searchable           bool            `json:"searchable"`
	Site                 string          `json:"site"`
	App                  string          `json:"app,omitempty"`
	Name                 string          `json:"name"`
	Intent               string          `json:"intent"`
	Description          string          `json:"description,omitempty"`
	Tags                 []string        `json:"tags,omitempty"`
	RiskLevel            RiskLevel       `json:"riskLevel"`
	RequiresConfirmation bool            `json:"requiresConfirmation"`
	Variables            []Variable      `json:"variables"`
	Chunks               []WorkflowChunk `json:"chunks"`
	StartPageStates      []string        `json:"startPageStates,omitempty"`
	EndPageStates        []string        `json:"endPageStates,omitempty"`
	CreatedAt            time.Time       `json:"createdAt,omitempty"`
	UpdatedAt            time.Time       `json:"updatedAt,omitempty"`
}

type WorkflowVersion struct {
	WorkflowID string         `json:"workflowId"`
	Version    int            `json:"version"`
	Recipe     WorkflowRecipe `json:"recipe"`
	Summary    string         `json:"summary"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type WorkflowCandidate struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	Source      string         `json:"source"`
	Task        string         `json:"task"`
	StartURL    string         `json:"startUrl"`
	RecipeDraft WorkflowRecipe `json:"recipeDraft"`
	Status      Status         `json:"status"`
	Searchable  bool           `json:"searchable"`
	CreatedAt   time.Time      `json:"createdAt"`
	ReviewedAt  time.Time      `json:"reviewedAt,omitempty"`
}

type WorkflowCard struct {
	WorkflowID           string    `json:"workflowId"`
	Version              int       `json:"version"`
	ProjectID            string    `json:"projectId"`
	TenantID             string    `json:"tenantId,omitempty"`
	Status               Status    `json:"status"`
	Searchable           bool      `json:"searchable"`
	Site                 string    `json:"site"`
	App                  string    `json:"app,omitempty"`
	Name                 string    `json:"name"`
	Intent               string    `json:"intent"`
	Description          string    `json:"description,omitempty"`
	Examples             []string  `json:"examples,omitempty"`
	Tags                 []string  `json:"tags,omitempty"`
	StartPageStates      []string  `json:"startPageStates,omitempty"`
	EndPageStates        []string  `json:"endPageStates,omitempty"`
	VariableNames        []string  `json:"variableNames,omitempty"`
	ActionTypes          []string  `json:"actionTypes,omitempty"`
	RiskLevel            RiskLevel `json:"riskLevel"`
	RequiresConfirmation bool      `json:"requiresConfirmation"`
	SuccessRate          float64   `json:"successRate,omitempty"`
	RecentFailureCount   int       `json:"recentFailureCount,omitempty"`
	LastFailureAt        time.Time `json:"lastFailureAt,omitempty"`
	UpdatedAt            time.Time `json:"updatedAt,omitempty"`
}

func (card WorkflowCard) EmbeddingText() string {
	parts := []string{
		"Workflow: " + card.Name,
		"Intent: " + card.Intent,
		"Description: " + card.Description,
		"Site: " + card.Site,
		"App: " + card.App,
		"Examples: " + strings.Join(card.Examples, "\n"),
		"Tags: " + strings.Join(card.Tags, ", "),
		"Start pages: " + strings.Join(card.StartPageStates, ", "),
		"End pages: " + strings.Join(card.EndPageStates, ", "),
		"Variables: " + strings.Join(card.VariableNames, ", "),
		"Actions: " + strings.Join(card.ActionTypes, ", "),
		fmt.Sprintf("Risk: %s", card.RiskLevel),
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

type ControlSignature struct {
	Role string `json:"role"`
	Name string `json:"name"`
}

type PageState struct {
	ID               string             `json:"id"`
	ProjectID        string             `json:"projectId"`
	Site             string             `json:"site"`
	App              string             `json:"app,omitempty"`
	URLPattern       string             `json:"urlPattern,omitempty"`
	RequiredText     []string           `json:"requiredText,omitempty"`
	RequiredControls []ControlSignature `json:"requiredControls,omitempty"`
	CanonicalTitle   string             `json:"canonicalTitle,omitempty"`
	Status           Status             `json:"status"`
	UpdatedAt        time.Time          `json:"updatedAt,omitempty"`
}

type PageTransition struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	Site          string `json:"site"`
	FromPageState string `json:"fromPageState"`
	ToPageState   string `json:"toPageState"`
	WorkflowID    string `json:"workflowId"`
	Version       int    `json:"version"`
	ChunkID       string `json:"chunkId"`
}

type InterruptHandler struct {
	ID                  string             `json:"id"`
	ProjectID           string             `json:"projectId"`
	Site                string             `json:"site"`
	App                 string             `json:"app,omitempty"`
	Status              Status             `json:"status"`
	AppliesToPageStates []string           `json:"appliesToPageStates,omitempty"`
	InterruptType       string             `json:"interruptType"`
	FingerprintText     string             `json:"fingerprintText"`
	RequiredText        []string           `json:"requiredText,omitempty"`
	TargetControls      []ControlSignature `json:"targetControls,omitempty"`
	RiskLevel           RiskLevel          `json:"riskLevel"`
	WorkflowID          string             `json:"workflowId"`
	Version             int                `json:"version"`
	SuccessRate         float64            `json:"successRate,omitempty"`
	UpdatedAt           time.Time          `json:"updatedAt,omitempty"`
}

type RepairPatch struct {
	ID                  string     `json:"id"`
	ProjectID           string     `json:"projectId"`
	Status              Status     `json:"status"`
	WorkflowID          string     `json:"workflowId"`
	WorkflowVersion     int        `json:"workflowVersion"`
	ChunkID             string     `json:"chunkId"`
	StepID              string     `json:"stepId"`
	Site                string     `json:"site"`
	App                 string     `json:"app,omitempty"`
	FailureType         string     `json:"failureType"`
	FailureSignature    string     `json:"failureSignature"`
	OldTarget           string     `json:"oldTarget,omitempty"`
	NewTargetSummary    string     `json:"newTargetSummary"`
	NewTarget           StepTarget `json:"newTarget,omitempty"`
	AppliesToPageStates []string   `json:"appliesToPageStates,omitempty"`
	RiskLevel           RiskLevel  `json:"riskLevel"`
	SuccessRate         float64    `json:"successRate,omitempty"`
	SourceRunID         string     `json:"sourceRunId,omitempty"`
	SourceFailureReason string     `json:"sourceFailureReason,omitempty"`
	CreatedAt           time.Time  `json:"createdAt,omitempty"`
	ReviewedAt          time.Time  `json:"reviewedAt,omitempty"`
	UpdatedAt           time.Time  `json:"updatedAt,omitempty"`
}

type WorkflowRun struct {
	ID             string            `json:"id"`
	ProjectID      string            `json:"projectId"`
	WorkflowID     string            `json:"workflowId"`
	Version        int               `json:"version"`
	Task           string            `json:"task"`
	URL            string            `json:"url"`
	Variables      map[string]string `json:"variables,omitempty"`
	Result         string            `json:"result"`
	FailedChunkID  string            `json:"failedChunkId,omitempty"`
	FailedStepID   string            `json:"failedStepId,omitempty"`
	FallbackReason string            `json:"fallbackReason,omitempty"`
	DurationMillis int               `json:"durationMillis,omitempty"`
	ArtifactRefs   []ArtifactRef     `json:"artifactRefs,omitempty"`
	Logs           []WorkflowRunLog  `json:"logs,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
}

type WorkflowRunLog struct {
	Timestamp                 time.Time            `json:"timestamp,omitempty"`
	Event                     string               `json:"event"`
	ChunkID                   string               `json:"chunkId,omitempty"`
	StepID                    string               `json:"stepId,omitempty"`
	PageState                 string               `json:"pageState,omitempty"`
	Message                   string               `json:"message,omitempty"`
	Reason                    string               `json:"reason,omitempty"`
	SelectorAttempts          []SelectorAttemptLog `json:"selectorAttempts,omitempty"`
	AppliedInterruptHandlerID string               `json:"appliedInterruptHandlerId,omitempty"`
	AppliedRepairPatchID      string               `json:"appliedRepairPatchId,omitempty"`
	ArtifactRefs              []ArtifactRef        `json:"artifactRefs,omitempty"`
}

type SelectorAttemptLog struct {
	Timestamp     time.Time `json:"timestamp,omitempty"`
	StepID        string    `json:"stepId,omitempty"`
	Strategy      string    `json:"strategy,omitempty"`
	Selector      string    `json:"selector,omitempty"`
	Result        string    `json:"result"`
	FailureReason string    `json:"failureReason,omitempty"`
}

type SelectorStats struct {
	WorkflowID        string    `json:"workflowId"`
	Version           int       `json:"version"`
	StepID            string    `json:"stepId"`
	Strategy          string    `json:"strategy"`
	Selector          string    `json:"selector"`
	SuccessCount      int       `json:"successCount"`
	FailCount         int       `json:"failCount"`
	LastSuccessAt     time.Time `json:"lastSuccessAt,omitempty"`
	LastFailAt        time.Time `json:"lastFailAt,omitempty"`
	LastFailureReason string    `json:"lastFailureReason,omitempty"`
}

type ArtifactRef struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
}

type OutboxEvent struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	IdempotencyKey string         `json:"idempotencyKey"`
	Payload        map[string]any `json:"payload"`
	Status         Status         `json:"status"`
	CreatedAt      time.Time      `json:"createdAt"`
	ProcessedAt    time.Time      `json:"processedAt,omitempty"`
}
