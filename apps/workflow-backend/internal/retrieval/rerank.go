package retrieval

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type RawCandidate struct {
	WorkflowID         string
	Version            int
	Status             registry.Status
	Searchable         bool
	Site               string
	RiskLevel          registry.RiskLevel
	VariableNames      []string
	WorkflowDense      float64
	WorkflowSparse     float64
	BestChunk          float64
	PageState          float64
	SuccessRate        float64
	SelectorHealth     float64
	Recency            float64
	UserPreference     float64
	RecentFailureCount int
	LastFailureAt      time.Time
	Now                time.Time
	UpdatedAt          time.Time
	StepText           string
}

type RerankInput struct {
	CandidateSlots map[string]string
	RiskPolicy     RiskPolicy
	Site           string
	Task           string
}

func Rerank(candidates []RawCandidate, input RerankInput) []WorkflowCandidate {
	results := make([]WorkflowCandidate, 0, len(candidates))
	requestedStepLabels := requestedStepLabels(input.Task, candidates)
	for _, candidate := range candidates {
		stepCoverage := stepCoverage(candidate.StepText, requestedStepLabels)
		if hardReject(candidate, input, stepCoverage) {
			continue
		}
		bindability := variableBindability(candidate.VariableNames, input.CandidateSlots)
		riskPenalty := riskPenalty(candidate.RiskLevel, input.RiskPolicy)
		recentFailurePenalty := recentFailurePenalty(candidate)
		selectorHealth := adjustedSelectorHealth(candidate, recentFailurePenalty)
		score := 0.24*candidate.WorkflowDense +
			0.16*candidate.WorkflowSparse +
			0.12*candidate.BestChunk +
			0.10*stepCoverage +
			0.14*candidate.PageState +
			0.12*bindability +
			0.08*candidate.SuccessRate +
			0.06*selectorHealth +
			0.04*candidate.Recency +
			0.04*candidate.UserPreference -
			0.20*riskPenalty -
			0.15*recentFailurePenalty
		results = append(results, WorkflowCandidate{
			WorkflowID: candidate.WorkflowID,
			Version:    candidate.Version,
			FinalScore: score,
			RiskLevel:  candidate.RiskLevel,
			Variables:  append([]string(nil), candidate.VariableNames...),
			Reasons:    reasons(candidate, input, bindability, stepCoverage),
			ScoreBreakdown: ScoreBreakdown{
				WorkflowDenseScore:    candidate.WorkflowDense,
				WorkflowSparseScore:   candidate.WorkflowSparse,
				BestChunkScore:        candidate.BestChunk,
				StepCoverage:          stepCoverage,
				PageStateScore:        candidate.PageState,
				VariableBindability:   bindability,
				HistoricalSuccessRate: candidate.SuccessRate,
				SelectorHealth:        selectorHealth,
				RiskPenalty:           riskPenalty,
				RecentFailurePenalty:  recentFailurePenalty,
			},
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if coverageDelta := results[i].ScoreBreakdown.StepCoverage - results[j].ScoreBreakdown.StepCoverage; abs(coverageDelta) >= 0.15 {
			return coverageDelta > 0
		}
		if results[i].FinalScore == results[j].FinalScore {
			return results[i].WorkflowID < results[j].WorkflowID
		}
		return results[i].FinalScore > results[j].FinalScore
	})
	return results
}

func hardReject(candidate RawCandidate, input RerankInput, stepCoverage float64) bool {
	if candidate.Status != registry.StatusActive || !candidate.Searchable {
		return true
	}
	if riskBlocked(candidate.RiskLevel, input.RiskPolicy) {
		return true
	}
	if variableBindability(candidate.VariableNames, input.CandidateSlots) < 1 {
		return true
	}
	selectorHealth := adjustedSelectorHealth(candidate, recentFailurePenalty(candidate))
	if selectorHealth > 0 && selectorHealth < 0.2 {
		return true
	}
	if !hasMeaningfulRetrievalSignal(candidate, stepCoverage) {
		return true
	}
	return false
}

func hasMeaningfulRetrievalSignal(candidate RawCandidate, stepCoverage float64) bool {
	const minimumSemanticSignal = 0.2
	return candidate.WorkflowDense >= minimumSemanticSignal ||
		candidate.WorkflowSparse >= minimumSemanticSignal ||
		candidate.BestChunk >= minimumSemanticSignal ||
		candidate.PageState >= minimumSemanticSignal ||
		stepCoverage > 0
}

func adjustedSelectorHealth(candidate RawCandidate, recentFailurePenalty float64) float64 {
	health := candidate.SelectorHealth
	if health == 0 {
		health = candidate.SuccessRate
	}
	if recentFailurePenalty == 0 {
		return health
	}
	return clamp01(health * (1 - 0.5*recentFailurePenalty))
}

func recentFailurePenalty(candidate RawCandidate) float64 {
	if candidate.RecentFailureCount <= 0 {
		return 0
	}
	countPenalty := clamp01(float64(candidate.RecentFailureCount) / 5)
	if candidate.LastFailureAt.IsZero() {
		return countPenalty
	}
	now := candidate.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	age := now.Sub(candidate.LastFailureAt)
	if age < 0 {
		age = 0
	}
	const fullPenaltyWindow = time.Hour
	const decayWindow = 7 * 24 * time.Hour
	recencyPenalty := 0.0
	switch {
	case age <= fullPenaltyWindow:
		recencyPenalty = 1
	case age >= decayWindow:
		recencyPenalty = 0.15
	default:
		recencyPenalty = 1 - 0.85*(float64(age-fullPenaltyWindow)/float64(decayWindow-fullPenaltyWindow))
	}
	return clamp01(countPenalty * recencyPenalty)
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func variableBindability(names []string, slots map[string]string) float64 {
	if len(names) == 0 {
		return 1
	}
	matched := 0
	for _, name := range names {
		if slots[name] != "" {
			matched++
		}
	}
	return float64(matched) / float64(len(names))
}

func riskBlocked(risk registry.RiskLevel, policy RiskPolicy) bool {
	for _, blocked := range policy.Blocked {
		if blocked == risk {
			return true
		}
	}
	return false
}

func riskPenalty(risk registry.RiskLevel, policy RiskPolicy) float64 {
	if riskBlocked(risk, policy) {
		return 1
	}
	for _, allowed := range policy.AutoAllowed {
		if allowed == risk {
			return 0
		}
	}
	return 0.5
}

func reasons(candidate RawCandidate, input RerankInput, bindability float64, stepCoverage float64) []string {
	result := []string{}
	if input.Site != "" && candidate.Site == input.Site {
		result = append(result, "site matched "+candidate.Site)
	}
	if candidate.WorkflowDense > 0 {
		result = append(result, "workflow semantic score matched")
	}
	if candidate.WorkflowSparse > 0 {
		result = append(result, "workflow lexical score matched")
	}
	if candidate.PageState > 0 {
		result = append(result, "current page state matched")
	}
	if stepCoverage > 0 {
		result = append(result, "recorded steps cover requested actions")
	}
	if bindability == 1 {
		result = append(result, "required variables are bindable")
	}
	return result
}

var actionLabelPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:\s+[A-Za-z][A-Za-z0-9]*){0,2}`)

func requestedStepLabels(task string, candidates []RawCandidate) []string {
	if task == "" {
		return nil
	}
	candidateText := strings.ToLower(strings.Join(candidateStepTexts(candidates), "\n"))
	labels := make([]string, 0)
	seen := map[string]bool{}
	for _, match := range actionLabelPattern.FindAllString(task, -1) {
		label := normalizeActionLabel(match)
		if label == "" || seen[label] || !strings.Contains(candidateText, label) {
			continue
		}
		seen[label] = true
		labels = append(labels, label)
	}
	return labels
}

func candidateStepTexts(candidates []RawCandidate) []string {
	values := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.StepText != "" {
			values = append(values, candidate.StepText)
		}
	}
	return values
}

func stepCoverage(stepText string, requestedLabels []string) float64 {
	if stepText == "" || len(requestedLabels) == 0 {
		return 0
	}
	normalizedStepText := strings.ToLower(stepText)
	matched := 0
	for _, label := range requestedLabels {
		if strings.Contains(normalizedStepText, label) {
			matched++
		}
	}
	return float64(matched) / float64(len(requestedLabels))
}

func normalizeActionLabel(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
