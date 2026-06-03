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
	StatusHidden        Status = "hidden"
	StatusStale         Status = "stale"
	StatusReplaced      Status = "replaced"
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
	Embedding            []float32       `json:"embedding,omitempty"`
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
	ID              string         `json:"id"`
	ProjectID       string         `json:"projectId"`
	Source          string         `json:"source"`
	SourceTaskRunID string         `json:"sourceTaskRunId,omitempty"`
	Task            string         `json:"task"`
	StartURL        string         `json:"startUrl"`
	RecipeDraft     WorkflowRecipe `json:"recipeDraft"`
	Status          Status         `json:"status"`
	Searchable      bool           `json:"searchable"`
	CreatedAt       time.Time      `json:"createdAt"`
	ReviewedAt      time.Time      `json:"reviewedAt,omitempty"`
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
	Role     string `json:"role"`
	Name     string `json:"name"`
	Selected *bool  `json:"selected,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

type PageState struct {
	ID                string             `json:"id"`
	ProjectID         string             `json:"projectId"`
	Site              string             `json:"site"`
	App               string             `json:"app,omitempty"`
	URLPattern        string             `json:"urlPattern,omitempty"`
	HardRules         HardRules          `json:"hardRules,omitempty"`
	RequiredText      []string           `json:"requiredText,omitempty"`
	RequiredControls  []ControlSignature `json:"requiredControls,omitempty"`
	StableControls    []ControlSignature `json:"stableControls,omitempty"`
	TransientControls []ControlSignature `json:"transientControls,omitempty"`
	SurfaceIDs        []string           `json:"surfaceIds,omitempty"`
	CanonicalTitle    string             `json:"canonicalTitle,omitempty"`
	BaseFingerprint   string             `json:"baseFingerprint,omitempty"`
	Confidence        float64            `json:"confidence,omitempty"`
	UpdatePolicy      string             `json:"updatePolicy,omitempty"`
	LastStableSeenAt  time.Time          `json:"lastStableSeenAt,omitempty"`
	Status            Status             `json:"status"`
	Embedding         []float32          `json:"embedding,omitempty"`
	UpdatedAt         time.Time          `json:"updatedAt,omitempty"`
}

type SurfaceType string

const (
	SurfaceModal    SurfaceType = "modal"
	SurfaceDrawer   SurfaceType = "drawer"
	SurfacePopover  SurfaceType = "popover"
	SurfaceDropdown SurfaceType = "dropdown"
	SurfaceCarousel SurfaceType = "carousel"
	SurfaceToast    SurfaceType = "toast"
	SurfaceWizard   SurfaceType = "wizard"
	SurfaceUnknown  SurfaceType = "unknown"
)

type PageSurface struct {
	ID                  string             `json:"id"`
	ProjectID           string             `json:"projectId"`
	Site                string             `json:"site"`
	ParentPageStateID   string             `json:"parentPageStateId"`
	SurfaceType         SurfaceType        `json:"surfaceType"`
	SurfaceFingerprint  string             `json:"surfaceFingerprint"`
	Title               string             `json:"title,omitempty"`
	RequiredText        []string           `json:"requiredText,omitempty"`
	RequiredControls    []ControlSignature `json:"requiredControls,omitempty"`
	OpenTriggerTargets  []string           `json:"openTriggerTargets,omitempty"`
	CloseTriggerTargets []string           `json:"closeTriggerTargets,omitempty"`
	VisibilityRules     HardRules          `json:"visibilityRules,omitempty"`
	ObservationCount    int                `json:"observationCount,omitempty"`
	Status              Status             `json:"status"`
	FirstSeenAt         time.Time          `json:"firstSeenAt,omitempty"`
	LastSeenAt          time.Time          `json:"lastSeenAt,omitempty"`
}

type PageTransition struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"projectId"`
	Site             string     `json:"site"`
	FromPageState    string     `json:"fromPageState"`
	ToPageState      string     `json:"toPageState"`
	WorkflowID       string     `json:"workflowId"`
	Version          int        `json:"version"`
	ChunkID          string     `json:"chunkId"`
	ActionName       string     `json:"actionName,omitempty"`
	TargetName       string     `json:"targetName,omitempty"`
	TargetCandidates StepTarget `json:"targetCandidates,omitempty"`
	GuardRules       HardRules  `json:"guardRules,omitempty"`
	RiskLevel        RiskLevel  `json:"riskLevel,omitempty"`
	SuccessCount     int        `json:"successCount,omitempty"`
	FailureCount     int        `json:"failureCount,omitempty"`
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

type TaskRunStatus string

const (
	TaskRunSuccess TaskRunStatus = "success"
	TaskRunFailed  TaskRunStatus = "failed"
	TaskRunPartial TaskRunStatus = "partial"
)

type HardRules struct {
	URLIncludes []string           `json:"urlIncludes,omitempty"`
	URLPattern  string             `json:"urlPattern,omitempty"`
	TextAll     []string           `json:"textAll,omitempty"`
	TextAny     []string           `json:"textAny,omitempty"`
	ControlsAll []ControlSignature `json:"controlsAll,omitempty"`
	ControlsAny []ControlSignature `json:"controlsAny,omitempty"`
}

type ObservationTableSignal struct {
	Caption string   `json:"caption,omitempty"`
	Headers []string `json:"headers,omitempty"`
}

type ActiveSurfaceSignal struct {
	SurfaceType SurfaceType        `json:"surfaceType,omitempty"`
	Title       string             `json:"title,omitempty"`
	Text        []string           `json:"text,omitempty"`
	Controls    []ControlSignature `json:"controls,omitempty"`
}

type ActionStep struct {
	ID                string                 `json:"id"`
	TaskRunID         string                 `json:"taskRunId,omitempty"`
	PageStateID       string                 `json:"pageStateId"`
	SurfaceID         string                 `json:"surfaceId,omitempty"`
	StepIndex         int                    `json:"stepIndex"`
	ActionType        StepType               `json:"actionType"`
	TargetName        string                 `json:"targetName"`
	ValueTemplate     string                 `json:"valueTemplate,omitempty"`
	ReasoningSummary  string                 `json:"reasoningSummary,omitempty"`
	ResultSummary     string                 `json:"resultSummary,omitempty"`
	IsBranchNoise     bool                   `json:"isBranchNoise,omitempty"`
	BeforeObservation *PageObservationSignal `json:"beforeObservation,omitempty"`
	AfterObservation  *PageObservationSignal `json:"afterObservation,omitempty"`
	CreatedAt         time.Time              `json:"createdAt,omitempty"`
}

func (step ActionStep) IsReusableSuccessful() bool {
	if step.IsBranchNoise {
		return false
	}
	result := strings.TrimSpace(step.ResultSummary)
	if result == "" {
		return true
	}
	lower := strings.ToLower(result)
	return result != "failed" &&
		!strings.HasPrefix(result, "❌") &&
		!strings.Contains(lower, "failed to") &&
		!strings.Contains(lower, "dom tree not indexed yet")
}

type TaskRun struct {
	ID                 string              `json:"id"`
	ProjectID          string              `json:"projectId"`
	Site               string              `json:"site"`
	TaskTemplate       string              `json:"taskTemplate"`
	Summary            string              `json:"summary"`
	OriginalPath       []string            `json:"originalPath"`
	OptimizedPath      []string            `json:"optimizedPath"`
	MemoryContextID    string              `json:"memoryContextId,omitempty"`
	MemoryEvidenceRefs []MemoryEvidenceRef `json:"memoryEvidenceRefs,omitempty"`
	Status             TaskRunStatus       `json:"status"`
	ActionSteps        []ActionStep        `json:"actionSteps,omitempty"`
	CreatedAt          time.Time           `json:"createdAt,omitempty"`
}

type SiteTaskGuideGuard struct {
	URLIncludes       []string           `json:"urlIncludes,omitempty"`
	URLPattern        string             `json:"urlPattern,omitempty"`
	RequiredText      []string           `json:"requiredText,omitempty"`
	RequiredControls  []ControlSignature `json:"requiredControls,omitempty"`
	ActiveOverlayHint string             `json:"activeOverlayHint,omitempty"`
}

type SiteTaskGuideStep struct {
	ID              string                       `json:"id,omitempty"`
	Index           int                          `json:"index,omitempty"`
	Text            string                       `json:"text"`
	Goal            string                       `json:"goal,omitempty"`
	ActionType      StepType                     `json:"actionType,omitempty"`
	Target          string                       `json:"target,omitempty"`
	PageTitle       string                       `json:"pageTitle,omitempty"`
	SemanticTarget  SiteTaskGuideStepTarget      `json:"semanticTarget,omitempty"`
	ExpectedOutcome SiteTaskGuideExpectedOutcome `json:"expectedOutcome,omitempty"`
}

type SiteTaskIntentTerms struct {
	Positive []string `json:"positive,omitempty"`
	Negative []string `json:"negative,omitempty"`
}

type SiteTaskGuideRouteScope struct {
	URLIncludes []string `json:"urlIncludes,omitempty"`
	URLPattern  string   `json:"urlPattern,omitempty"`
}

type SiteTaskGuideUIStateType string

const (
	SiteTaskGuideUIStatePage       SiteTaskGuideUIStateType = "page"
	SiteTaskGuideUIStateTab        SiteTaskGuideUIStateType = "tab"
	SiteTaskGuideUIStateModal      SiteTaskGuideUIStateType = "modal"
	SiteTaskGuideUIStateDrawer     SiteTaskGuideUIStateType = "drawer"
	SiteTaskGuideUIStatePopover    SiteTaskGuideUIStateType = "popover"
	SiteTaskGuideUIStateWizard     SiteTaskGuideUIStateType = "wizard"
	SiteTaskGuideUIStateTableState SiteTaskGuideUIStateType = "table_state"
)

type SiteTaskGuideUIStateEvidence struct {
	TitleAny            []string                 `json:"titleAny,omitempty"`
	BreadcrumbAny       []string                 `json:"breadcrumbAny,omitempty"`
	ActiveTabAny        []string                 `json:"activeTabAny,omitempty"`
	TextAll             []string                 `json:"textAll,omitempty"`
	TextAny             []string                 `json:"textAny,omitempty"`
	ControlsAll         []ControlSignature       `json:"controlsAll,omitempty"`
	ControlsAny         []ControlSignature       `json:"controlsAny,omitempty"`
	TableHeadersAny     [][]string               `json:"tableHeadersAny,omitempty"`
	NegativeTextAny     []string                 `json:"negativeTextAny,omitempty"`
	NegativeControlsAny []ControlSignature       `json:"negativeControlsAny,omitempty"`
	ActiveSurfacesAny   []ActiveSurfaceSignal    `json:"activeSurfacesAny,omitempty"`
	TablesAny           []ObservationTableSignal `json:"tablesAny,omitempty"`
}

type SiteTaskGuideUIStateEntry struct {
	ID               string                       `json:"id"`
	Name             string                       `json:"name,omitempty"`
	StateType        SiteTaskGuideUIStateType     `json:"stateType"`
	StepOffset       int                          `json:"stepOffset"`
	RouteScope       SiteTaskGuideRouteScope      `json:"routeScope,omitempty"`
	Evidence         SiteTaskGuideUIStateEvidence `json:"evidence,omitempty"`
	MinimumScore     float64                      `json:"minimumScore,omitempty"`
	RequiredEvidence []string                     `json:"requiredEvidence,omitempty"`
	Confidence       float64                      `json:"confidence,omitempty"`
	LastMatchedAt    time.Time                    `json:"lastMatchedAt,omitempty"`
	Status           Status                       `json:"status,omitempty"`
}

type SiteTaskGuideStepTarget struct {
	Role          string   `json:"role,omitempty"`
	Text          string   `json:"text,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
	ContainerHint string   `json:"containerHint,omitempty"`
}

type SiteTaskGuideExpectedOutcome struct {
	SurfaceType  SurfaceType        `json:"surfaceType,omitempty"`
	TitleAny     []string           `json:"titleAny,omitempty"`
	TextAny      []string           `json:"textAny,omitempty"`
	ActiveTabAny []string           `json:"activeTabAny,omitempty"`
	ControlsAll  []ControlSignature `json:"controlsAll,omitempty"`
	ControlsAny  []ControlSignature `json:"controlsAny,omitempty"`
}

type SiteTaskGuide struct {
	ID                string                      `json:"id"`
	ProjectID         string                      `json:"projectId"`
	Site              string                      `json:"site"`
	Module            string                      `json:"module,omitempty"`
	TaskIntentKey     string                      `json:"taskIntentKey"`
	TaskIntentSummary string                      `json:"taskIntentSummary"`
	TaskIntentTerms   SiteTaskIntentTerms         `json:"taskIntentTerms,omitempty"`
	TaskExamples      []string                    `json:"taskExamples,omitempty"`
	InputSchema       []Variable                  `json:"inputSchema,omitempty"`
	RouteScope        SiteTaskGuideRouteScope     `json:"routeScope,omitempty"`
	StartURLPattern   string                      `json:"startUrlPattern,omitempty"`
	StartPageGuard    SiteTaskGuideGuard          `json:"startPageGuard,omitempty"`
	PageGuards        []SiteTaskGuideGuard        `json:"pageGuards,omitempty"`
	UIStateEntries    []SiteTaskGuideUIStateEntry `json:"uiStateEntries,omitempty"`
	Steps             []SiteTaskGuideStep         `json:"steps"`
	AbandonRules      []string                    `json:"abandonRules,omitempty"`
	RejectRules       []string                    `json:"rejectRules,omitempty"`
	VariableRules     []string                    `json:"variableRules,omitempty"`
	Evidence          []MemorySourceRef           `json:"evidence,omitempty"`
	Summary           string                      `json:"summary"`
	Confidence        float64                     `json:"confidence,omitempty"`
	SuccessCount      int                         `json:"successCount,omitempty"`
	AbandonedCount    int                         `json:"abandonedCount,omitempty"`
	MisleadingCount   int                         `json:"misleadingCount,omitempty"`
	UnusedCount       int                         `json:"unusedCount,omitempty"`
	StaleCount        int                         `json:"staleCount,omitempty"`
	Status            Status                      `json:"status"`
	CreatedAt         time.Time                   `json:"createdAt,omitempty"`
	UpdatedAt         time.Time                   `json:"updatedAt,omitempty"`
}

func (guide SiteTaskGuide) SearchableText() string {
	parts := []string{
		guide.TaskIntentKey,
		guide.TaskIntentSummary,
		guide.Summary,
		strings.Join(guide.TaskExamples, " "),
	}
	for _, step := range guide.Steps {
		parts = append(parts, step.Text, step.Target, step.Goal)
		parts = append(parts, step.SemanticTarget.Text, step.SemanticTarget.Role, strings.Join(step.SemanticTarget.Aliases, " "), step.SemanticTarget.ContainerHint)
	}
	parts = append(parts, strings.Join(guide.TaskIntentTerms.Positive, " "))
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

type SiteTaskGuideFeedbackLabel string

const (
	SiteTaskGuideFeedbackUsedHelpful       SiteTaskGuideFeedbackLabel = "used_helpful"
	SiteTaskGuideFeedbackAbandonedMismatch SiteTaskGuideFeedbackLabel = "abandoned_mismatch"
	SiteTaskGuideFeedbackUsedMisleading    SiteTaskGuideFeedbackLabel = "used_misleading"
	SiteTaskGuideFeedbackUnused            SiteTaskGuideFeedbackLabel = "unused"
	SiteTaskGuideFeedbackStale             SiteTaskGuideFeedbackLabel = "stale"
	SiteTaskGuideFeedbackManualHelpful     SiteTaskGuideFeedbackLabel = "manual_helpful"
	SiteTaskGuideFeedbackManualStale       SiteTaskGuideFeedbackLabel = "manual_stale"
	SiteTaskGuideFeedbackManualUnused      SiteTaskGuideFeedbackLabel = "manual_unused"
)

type SiteTaskGuideFeedback struct {
	ID               string                     `json:"id"`
	GuideID          string                     `json:"guideId"`
	StateID          string                     `json:"stateId,omitempty"`
	TaskRunID        string                     `json:"taskRunId"`
	ContextID        string                     `json:"contextId"`
	Label            SiteTaskGuideFeedbackLabel `json:"label"`
	Reason           string                     `json:"reason,omitempty"`
	MatchedStepCount int                        `json:"matchedStepCount,omitempty"`
	BacktrackCount   int                        `json:"backtrackCount,omitempty"`
	CreatedAt        time.Time                  `json:"createdAt,omitempty"`
}

type SiteManualSourceType string

const (
	SiteManualSourceMarkdown SiteManualSourceType = "markdown"
	SiteManualSourceHTML     SiteManualSourceType = "html"
	SiteManualSourcePDFText  SiteManualSourceType = "pdf_text"
	SiteManualSourceText     SiteManualSourceType = "text"
)

type SiteManualSource struct {
	ID          string               `json:"id"`
	ProjectID   string               `json:"projectId"`
	Site        string               `json:"site"`
	Module      string               `json:"module,omitempty"`
	Title       string               `json:"title"`
	SourceType  SiteManualSourceType `json:"sourceType"`
	ContentHash string               `json:"contentHash"`
	RawContent  string               `json:"rawContent,omitempty"`
	Metadata    map[string]any       `json:"metadata,omitempty"`
	Status      Status               `json:"status"`
	CreatedAt   time.Time            `json:"createdAt,omitempty"`
}

type SiteManualChunkType string

const (
	SiteManualChunkPageSummary      SiteManualChunkType = "page_summary"
	SiteManualChunkProcedure        SiteManualChunkType = "procedure"
	SiteManualChunkWarning          SiteManualChunkType = "warning"
	SiteManualChunkFieldExplanation SiteManualChunkType = "field_explanation"
)

type SiteManualWikiPage struct {
	ID           string            `json:"id"`
	ProjectID    string            `json:"projectId"`
	Site         string            `json:"site"`
	Module       string            `json:"module,omitempty"`
	PageKey      string            `json:"pageKey"`
	Title        string            `json:"title"`
	Summary      string            `json:"summary"`
	Facts        []string          `json:"facts,omitempty"`
	Procedures   []string          `json:"procedures,omitempty"`
	RelatedPages []string          `json:"relatedPages,omitempty"`
	SourceRefs   []MemorySourceRef `json:"sourceRefs,omitempty"`
	Confidence   float64           `json:"confidence,omitempty"`
	Status       Status            `json:"status"`
	UpdatedAt    time.Time         `json:"updatedAt,omitempty"`
}

type SiteManualWikiChunk struct {
	ID          string              `json:"id"`
	WikiPageID  string              `json:"wikiPageId"`
	ProjectID   string              `json:"projectId"`
	Site        string              `json:"site"`
	Module      string              `json:"module,omitempty"`
	ChunkType   SiteManualChunkType `json:"chunkType"`
	Text        string              `json:"text"`
	PageGuards  HardRules           `json:"pageGuards,omitempty"`
	TargetTerms []string            `json:"targetTerms,omitempty"`
	SourceRefs  []MemorySourceRef   `json:"sourceRefs,omitempty"`
	Embedding   []float32           `json:"embedding,omitempty"`
	Status      Status              `json:"status"`
	Score       float64             `json:"score,omitempty"`
}

type SiteManualWiki struct {
	Pages  []SiteManualWikiPage  `json:"pages"`
	Chunks []SiteManualWikiChunk `json:"chunks"`
}

type PageObservationSignal struct {
	ProjectID         string                   `json:"projectId"`
	Site              string                   `json:"site"`
	URL               string                   `json:"url"`
	URLPattern        string                   `json:"urlPattern,omitempty"`
	URLFamily         string                   `json:"urlFamily,omitempty"`
	Title             string                   `json:"title,omitempty"`
	Breadcrumbs       []string                 `json:"breadcrumbs,omitempty"`
	ActiveTabs        []string                 `json:"activeTabs,omitempty"`
	VisibleTextSample string                   `json:"visibleTextSample,omitempty"`
	ControlSignatures []ControlSignature       `json:"controlSignatures,omitempty"`
	Tables            []ObservationTableSignal `json:"tables,omitempty"`
	ActiveSurfaces    []ActiveSurfaceSignal    `json:"activeSurfaces,omitempty"`
	ActiveOverlayHint string                   `json:"activeOverlayHint,omitempty"`
	ObservedAt        time.Time                `json:"observedAt,omitempty"`
}

type SiteManualImportRequest struct {
	ProjectID  string               `json:"projectId"`
	Site       string               `json:"site"`
	Module     string               `json:"module,omitempty"`
	Title      string               `json:"title"`
	Content    string               `json:"content"`
	SourceType SiteManualSourceType `json:"sourceType"`
	Metadata   map[string]any       `json:"metadata,omitempty"`
}

type SiteManualSourceHashQuery struct {
	ProjectID   string
	Site        string
	Module      string
	ContentHash string
}

type SiteManualSourceListQuery struct {
	ProjectID     string
	Site          string
	Module        string
	IncludeHidden bool
}

type SiteManualWikiSearchQuery struct {
	ProjectID         string
	Site              string
	Module            string
	Task              string
	URL               string
	Title             string
	VisibleTextSample string
	ActiveOverlayHint string
	Embedding         []float32
	Limit             int
	IncludeFiltered   bool
}

type SiteManualKnowledgeMatch struct {
	Chunk  SiteManualWikiChunk `json:"chunk"`
	Reason string              `json:"reason,omitempty"`
	Score  float64             `json:"score,omitempty"`
}

type SiteManualFilteredReason struct {
	ChunkID string `json:"chunkId"`
	Reason  string `json:"reason"`
}

type SiteManualPreviewContext struct {
	Matches  []SiteManualKnowledgeMatch `json:"matches"`
	Filtered []SiteManualFilteredReason `json:"filtered"`
	Prompt   string                     `json:"prompt"`
}

type PageStatePurpose struct {
	ID               string             `json:"id"`
	ProjectID        string             `json:"projectId"`
	Site             string             `json:"site"`
	URLPattern       string             `json:"urlPattern,omitempty"`
	PageName         string             `json:"pageName"`
	PurposeSummary   string             `json:"purposeSummary"`
	HardRules        HardRules          `json:"hardRules,omitempty"`
	RequiredControls []ControlSignature `json:"requiredControls,omitempty"`
	RequiredText     []string           `json:"requiredText,omitempty"`
	UpdatedAt        time.Time          `json:"updatedAt,omitempty"`
}

type MemorySourceRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type PageObservationEvent struct {
	ID                string             `json:"id"`
	ProjectID         string             `json:"projectId"`
	Site              string             `json:"site"`
	URL               string             `json:"url"`
	URLPattern        string             `json:"urlPattern"`
	Title             string             `json:"title"`
	VisibleTextSample string             `json:"visibleTextSample,omitempty"`
	Controls          []ControlSignature `json:"controls,omitempty"`
	Links             []PageLinkSummary  `json:"links,omitempty"`
	Fingerprint       string             `json:"fingerprint"`
	Payload           map[string]any     `json:"payload,omitempty"`
	CreatedAt         time.Time          `json:"createdAt,omitempty"`
	LastSeenAt        time.Time          `json:"lastSeenAt,omitempty"`
	SeenCount         int                `json:"seenCount,omitempty"`
}

type PageLinkSummary struct {
	Text       string `json:"text"`
	URLPattern string `json:"urlPattern,omitempty"`
}

type ExperienceMemory struct {
	ID                   string                    `json:"id"`
	ProjectID            string                    `json:"projectId"`
	Site                 string                    `json:"site"`
	TaskTemplate         string                    `json:"taskTemplate"`
	IntentKey            string                    `json:"intentKey,omitempty"`
	Intent               string                    `json:"intent"`
	Summary              string                    `json:"summary"`
	StartPageState       string                    `json:"startPageState,omitempty"`
	EndPageState         string                    `json:"endPageState,omitempty"`
	StartPageFamily      string                    `json:"startPageFamily,omitempty"`
	EndPageFamily        string                    `json:"endPageFamily,omitempty"`
	OptimizedPath        []string                  `json:"optimizedPath,omitempty"`
	BestPath             []string                  `json:"bestPath,omitempty"`
	AlternativePaths     []ExperiencePathCandidate `json:"alternativePaths,omitempty"`
	PathSignature        string                    `json:"pathSignature,omitempty"`
	TargetSignature      string                    `json:"targetSignature,omitempty"`
	StepsSummary         []ExperienceStepSummary   `json:"stepsSummary,omitempty"`
	Variables            []Variable                `json:"variables,omitempty"`
	TaskTemplateExamples []string                  `json:"taskTemplateExamples,omitempty"`
	SuccessCount         int                       `json:"successCount,omitempty"`
	FailureCount         int                       `json:"failureCount,omitempty"`
	MisleadingCount      int                       `json:"misleadingCount,omitempty"`
	AverageStepCount     float64                   `json:"averageStepCount,omitempty"`
	Confidence           float64                   `json:"confidence,omitempty"`
	LastSuccessAt        time.Time                 `json:"lastSuccessAt,omitempty"`
	LastFailureAt        time.Time                 `json:"lastFailureAt,omitempty"`
	LastUsedAt           time.Time                 `json:"lastUsedAt,omitempty"`
	FailureWarnings      []string                  `json:"failureWarnings,omitempty"`
	Searchable           bool                      `json:"searchable"`
	ReviewStatus         ReviewStatus              `json:"reviewStatus"`
	Embedding            []float32                 `json:"embedding,omitempty"`
	Score                float64                   `json:"score,omitempty"`
	UpdatedAt            time.Time                 `json:"updatedAt,omitempty"`
	CreatedAt            time.Time                 `json:"createdAt,omitempty"`
}

func (memory ExperienceMemory) SearchableText() string {
	parts := []string{
		memory.TaskTemplate,
		memory.IntentKey,
		memory.Intent,
		memory.Summary,
		memory.PathSignature,
		memory.TargetSignature,
		strings.Join(memory.TaskTemplateExamples, " "),
		strings.Join(memory.FailureWarnings, " "),
	}
	for _, variable := range memory.Variables {
		parts = append(parts, variable.Name)
	}
	for _, step := range memory.StepsSummary {
		parts = append(parts, step.ActionName, step.TargetName, step.ValueTemplate, step.ResultSummary)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

type ExperiencePathCandidate struct {
	OptimizedPath    []string  `json:"optimizedPath,omitempty"`
	PathSignature    string    `json:"pathSignature,omitempty"`
	TargetSignature  string    `json:"targetSignature,omitempty"`
	SuccessCount     int       `json:"successCount,omitempty"`
	FailureCount     int       `json:"failureCount,omitempty"`
	AverageStepCount float64   `json:"averageStepCount,omitempty"`
	LastSuccessAt    time.Time `json:"lastSuccessAt,omitempty"`
	Score            float64   `json:"score,omitempty"`
}

type ExperienceStepSummary struct {
	ActionName    string    `json:"actionName"`
	TargetName    string    `json:"targetName,omitempty"`
	ValueTemplate string    `json:"valueTemplate,omitempty"`
	ResultSummary string    `json:"resultSummary,omitempty"`
	PageStateID   string    `json:"pageStateId,omitempty"`
	SurfaceID     string    `json:"surfaceId,omitempty"`
	RiskLevel     RiskLevel `json:"riskLevel,omitempty"`
	IsBranchNoise bool      `json:"isBranchNoise,omitempty"`
}

type MemoryStepTarget struct {
	ActionType    string `json:"actionType,omitempty"`
	Role          string `json:"role,omitempty"`
	TargetName    string `json:"targetName,omitempty"`
	ValueTemplate string `json:"valueTemplate,omitempty"`
	PageStateID   string `json:"pageStateId,omitempty"`
	SurfaceID     string `json:"surfaceId,omitempty"`
}

type FailureMemory struct {
	ID              string         `json:"id"`
	ExperienceID    string         `json:"experienceId,omitempty"`
	ProjectID       string         `json:"projectId"`
	Site            string         `json:"site"`
	PageStateID     string         `json:"pageStateId,omitempty"`
	ActionName      string         `json:"actionName,omitempty"`
	FailureType     string         `json:"failureType"`
	FailureSummary  string         `json:"failureSummary"`
	AvoidHint       string         `json:"avoidHint,omitempty"`
	Payload         map[string]any `json:"payload,omitempty"`
	CreatedAt       time.Time      `json:"createdAt,omitempty"`
	LastSeenAt      time.Time      `json:"lastSeenAt,omitempty"`
	OccurrenceCount int            `json:"occurrenceCount,omitempty"`
	Score           float64        `json:"score,omitempty"`
}

type MemoryMode string

const (
	MemoryModeGuided MemoryMode = "guided"
	MemoryModeNormal MemoryMode = "normal"
)

type MemoryEvidenceSource string

const (
	MemoryEvidenceSourceGuide      MemoryEvidenceSource = "guide"
	MemoryEvidenceSourceManual     MemoryEvidenceSource = "manual"
	MemoryEvidenceSourceExperience MemoryEvidenceSource = "experience"
	MemoryEvidenceSourceFailure    MemoryEvidenceSource = "failure"
	MemoryEvidenceSourceNavigation MemoryEvidenceSource = "navigation"
)

type MemoryAttributionLabel string

const (
	MemoryAttributionHelpful    MemoryAttributionLabel = "helpful"
	MemoryAttributionUnused     MemoryAttributionLabel = "unused"
	MemoryAttributionMisleading MemoryAttributionLabel = "misleading"
	MemoryAttributionStale      MemoryAttributionLabel = "stale"
	MemoryAttributionNeutral    MemoryAttributionLabel = "neutral"
)

type MemoryEvidenceRef struct {
	ID           string               `json:"id"`
	Source       MemoryEvidenceSource `json:"source"`
	Title        string               `json:"title,omitempty"`
	Rank         int                  `json:"rank,omitempty"`
	Score        float64              `json:"score,omitempty"`
	PageStateID  string               `json:"pageStateId,omitempty"`
	MatchedRules []string             `json:"matchedRules,omitempty"`
	Reason       string               `json:"reason,omitempty"`
	Payload      map[string]any       `json:"payload,omitempty"`
}

type MemoryAttributionEvent struct {
	ID             string                 `json:"id"`
	ProjectID      string                 `json:"projectId"`
	Site           string                 `json:"site"`
	TaskRunID      string                 `json:"taskRunId"`
	ContextID      string                 `json:"contextId"`
	EvidenceID     string                 `json:"evidenceId"`
	EvidenceSource MemoryEvidenceSource   `json:"evidenceSource"`
	Label          MemoryAttributionLabel `json:"label"`
	Reason         string                 `json:"reason,omitempty"`
	Signals        []string               `json:"signals,omitempty"`
	Adoption       MemoryAdoptionSignal   `json:"adoption,omitempty"`
	CreatedAt      time.Time              `json:"createdAt,omitempty"`
}

type MemoryAdoptionSignal struct {
	EvidenceID              string `json:"evidenceId,omitempty"`
	AdoptedPath             bool   `json:"adoptedPath,omitempty"`
	AdoptedTarget           bool   `json:"adoptedTarget,omitempty"`
	AdoptedSurface          bool   `json:"adoptedSurface,omitempty"`
	FirstAdoptedStepIndex   int    `json:"firstAdoptedStepIndex,omitempty"`
	LaterAbandoned          bool   `json:"laterAbandoned,omitempty"`
	CausedBacktrack         bool   `json:"causedBacktrack,omitempty"`
	CausedSelectorFailure   bool   `json:"causedSelectorFailure,omitempty"`
	FinalPathCameFromMemory bool   `json:"finalPathCameFromMemory,omitempty"`
}

type MemoryEvidenceStats struct {
	ProjectID       string               `json:"projectId"`
	Site            string               `json:"site"`
	EvidenceID      string               `json:"evidenceId"`
	EvidenceSource  MemoryEvidenceSource `json:"evidenceSource"`
	HelpfulCount    int                  `json:"helpfulCount"`
	UnusedCount     int                  `json:"unusedCount"`
	MisleadingCount int                  `json:"misleadingCount"`
	StaleCount      int                  `json:"staleCount"`
	NeutralCount    int                  `json:"neutralCount"`
	UtilityScore    float64              `json:"utilityScore"`
	LastFeedbackAt  time.Time            `json:"lastFeedbackAt,omitempty"`
}

type MemoryContextEvent struct {
	ID                    string              `json:"id"`
	ProjectID             string              `json:"projectId"`
	Task                  string              `json:"task"`
	CurrentURL            string              `json:"currentUrl"`
	CurrentPageState      string              `json:"currentPageState,omitempty"`
	CurrentSurface        string              `json:"currentSurface,omitempty"`
	ContextPrompt         string              `json:"contextPrompt,omitempty"`
	SelectedExperienceIDs []string            `json:"selectedExperienceIds,omitempty"`
	SelectedWorkflowIDs   []string            `json:"selectedWorkflowIds,omitempty"`
	EvidenceRefs          []MemoryEvidenceRef `json:"evidenceRefs,omitempty"`
	RecommendedMode       MemoryMode          `json:"recommendedMode"`
	Payload               map[string]any      `json:"payload,omitempty"`
	CreatedAt             time.Time           `json:"createdAt,omitempty"`
}

type ReviewStatus string

const (
	ReviewStatusPending      ReviewStatus = "pending_review"
	ReviewStatusAutoApproved ReviewStatus = "auto_approved"
	ReviewStatusApproved     ReviewStatus = "approved"
	ReviewStatusRejected     ReviewStatus = "rejected"
)

type ReviewTargetType string

const (
	ReviewTargetExperience ReviewTargetType = "experience"
	ReviewTargetWorkflow   ReviewTargetType = "workflow"
	ReviewTargetRepair     ReviewTargetType = "repair"
	ReviewTargetPageState  ReviewTargetType = "page_state"
)

type MemoryReview struct {
	ID         string           `json:"id"`
	ProjectID  string           `json:"projectId"`
	TargetType ReviewTargetType `json:"targetType"`
	TargetID   string           `json:"targetId"`
	Status     ReviewStatus     `json:"status"`
	Summary    string           `json:"summary,omitempty"`
	CreatedAt  time.Time        `json:"createdAt,omitempty"`
	ReviewedAt time.Time        `json:"reviewedAt,omitempty"`
}

type TaskRunListQuery struct {
	ProjectID string
	Site      string
	Status    TaskRunStatus
}

type PageStateListQuery struct {
	ProjectID string
	Site      string
}

type PageSurfaceListQuery struct {
	ProjectID         string
	Site              string
	ParentPageStateID string
	SurfaceType       SurfaceType
}

type PageTransitionListQuery struct {
	ProjectID       string
	Site            string
	FromPageStateID string
	ToPageStateID   string
}

type PageObservationEventListQuery struct {
	ProjectID  string
	Site       string
	URLPattern string
}

type ExperienceMemorySearchQuery struct {
	ProjectID      string
	Site           string
	Status         ReviewStatus
	SearchableOnly bool
	StartPageState string
	Task           string
	Embedding      []float32
	Limit          int
}

type FailureMemorySearchQuery struct {
	ProjectID   string
	Site        string
	PageStateID string
	Task        string
	Limit       int
}

type MemoryReviewListQuery struct {
	ProjectID  string
	TargetType ReviewTargetType
	Status     ReviewStatus
}

type MemoryAttributionEventListQuery struct {
	ProjectID      string
	Site           string
	TaskRunID      string
	ContextID      string
	EvidenceID     string
	EvidenceSource MemoryEvidenceSource
	Label          MemoryAttributionLabel
}

type MemoryEvidenceStatsListQuery struct {
	ProjectID      string
	Site           string
	EvidenceID     string
	EvidenceSource MemoryEvidenceSource
}

type MemoryPrunePolicy struct {
	ProjectID                string `json:"projectId,omitempty"`
	Site                     string `json:"site,omitempty"`
	MaxTaskRuns              int    `json:"maxTaskRuns,omitempty"`
	MaxPageObservationEvents int    `json:"maxPageObservationEvents,omitempty"`
	MaxMemoryContextEvents   int    `json:"maxMemoryContextEvents,omitempty"`
	MaxFailureMemories       int    `json:"maxFailureMemories,omitempty"`
}

type MemoryPruneResult struct {
	DeletedTaskRuns              int `json:"deletedTaskRuns"`
	DeletedPageObservationEvents int `json:"deletedPageObservationEvents"`
	DeletedMemoryContextEvents   int `json:"deletedMemoryContextEvents"`
	DeletedFailureMemories       int `json:"deletedFailureMemories"`
	UpdatedSiteTaskGuides        int `json:"updatedSiteTaskGuides,omitempty"`
	UpdatedSiteManualSources     int `json:"updatedSiteManualSources,omitempty"`
}

type WorkflowEmbeddingHit struct {
	Workflow WorkflowRecipe
	Score    float64
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
