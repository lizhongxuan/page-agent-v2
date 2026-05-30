package memory

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type ExperienceService struct {
	repo registry.Repository
}

func NewExperienceService(repo registry.Repository) *ExperienceService {
	return &ExperienceService{repo: repo}
}

func (service *ExperienceService) UpsertExperienceFromTaskRun(ctx context.Context, run registry.TaskRun) (registry.ExperienceMemory, error) {
	if service.repo == nil {
		return registry.ExperienceMemory{}, errors.New("workflow registry is not configured")
	}
	if run.Status != registry.TaskRunSuccess {
		return registry.ExperienceMemory{}, errors.New("task run is not successful")
	}
	reusableSteps := reusableSuccessfulActionSteps(run.ActionSteps)
	id := stableExperienceID(run)
	existing, _ := service.repo.GetExperienceMemory(ctx, id)
	now := time.Now().UTC()
	experience := existing
	intentKey := experienceIntentKey(run.TaskTemplate)
	targetSignature := targetSignatureFromActionSteps(reusableSteps)
	pathSignature := pathSignature(run.OptimizedPath)
	pathCandidate := registry.ExperiencePathCandidate{
		OptimizedPath:    append([]string(nil), run.OptimizedPath...),
		PathSignature:    pathSignature,
		TargetSignature:  targetSignature,
		SuccessCount:     1,
		AverageStepCount: float64(len(run.OptimizedPath)),
		LastSuccessAt:    now,
	}
	if experience.ID == "" {
		experience = registry.ExperienceMemory{
			ID:                   id,
			ProjectID:            run.ProjectID,
			Site:                 run.Site,
			TaskTemplate:         run.TaskTemplate,
			IntentKey:            intentKey,
			Intent:               run.TaskTemplate,
			Summary:              run.Summary,
			StartPageState:       firstPathItem(run.OptimizedPath),
			EndPageState:         lastPathItem(run.OptimizedPath),
			StartPageFamily:      firstPathItem(run.OptimizedPath),
			EndPageFamily:        lastPathItem(run.OptimizedPath),
			OptimizedPath:        append([]string(nil), run.OptimizedPath...),
			BestPath:             append([]string(nil), run.OptimizedPath...),
			PathSignature:        pathSignature,
			TargetSignature:      targetSignature,
			Variables:            variablesFromActionSteps(reusableSteps),
			TaskTemplateExamples: []string{run.TaskTemplate},
			CreatedAt:            now,
		}
	}
	experience.Summary = TruncateSummary(nonEmpty(run.Summary, experience.Summary))
	experience.IntentKey = nonEmpty(experience.IntentKey, intentKey)
	experience.Intent = nonEmpty(experience.Intent, run.TaskTemplate)
	experience.StartPageState = nonEmpty(experience.StartPageState, firstPathItem(run.OptimizedPath))
	experience.EndPageState = nonEmpty(experience.EndPageState, lastPathItem(run.OptimizedPath))
	experience.StartPageFamily = nonEmpty(experience.StartPageFamily, experience.StartPageState)
	experience.EndPageFamily = nonEmpty(experience.EndPageFamily, experience.EndPageState)
	experience.TargetSignature = targetSignature
	experience.StepsSummary = stepsSummaryFromActionSteps(reusableSteps)
	experience.Variables = variablesFromActionSteps(reusableSteps)
	experience.TaskTemplateExamples = appendUniqueLimited(experience.TaskTemplateExamples, run.TaskTemplate, 5)
	previousSuccessCount := experience.SuccessCount
	experience.SuccessCount = previousSuccessCount + 1
	experience.AlternativePaths = mergeExperiencePathCandidate(experience.AlternativePaths, pathCandidate)
	experience.AlternativePaths = scoreExperiencePathCandidates(experience.AlternativePaths, experience.FailureCount, now)
	best := bestExperiencePathCandidate(experience.AlternativePaths)
	if len(best.OptimizedPath) > 0 {
		experience.BestPath = append([]string(nil), best.OptimizedPath...)
		experience.OptimizedPath = append([]string(nil), best.OptimizedPath...)
		experience.PathSignature = best.PathSignature
	} else {
		experience.BestPath = append([]string(nil), run.OptimizedPath...)
		experience.OptimizedPath = append([]string(nil), run.OptimizedPath...)
		experience.PathSignature = pathSignature
	}
	experience.AverageStepCount = rollingAverageStepCount(experience.AverageStepCount, previousSuccessCount, len(run.OptimizedPath))
	experience.Confidence = experienceConfidence(experience)
	experience.LastSuccessAt = now
	experience.LastUsedAt = now
	experience.UpdatedAt = now
	if isLowRiskExperience(reusableSteps) {
		experience.Searchable = true
		if experience.ReviewStatus == "" || experience.ReviewStatus == registry.ReviewStatusPending {
			experience.ReviewStatus = registry.ReviewStatusAutoApproved
		}
	} else {
		experience.Searchable = false
		experience.ReviewStatus = registry.ReviewStatusPending
	}
	if err := service.repo.SaveExperienceMemory(ctx, experience); err != nil {
		return registry.ExperienceMemory{}, err
	}
	return service.repo.GetExperienceMemory(ctx, id)
}

func (service *ExperienceService) RecordFailureMemory(ctx context.Context, run registry.TaskRun) (registry.FailureMemory, error) {
	if service.repo == nil {
		return registry.FailureMemory{}, errors.New("workflow registry is not configured")
	}
	step := firstActionStep(run.ActionSteps)
	now := time.Now().UTC()
	failure := registry.FailureMemory{
		ID:             stableFailureID(run),
		ExperienceID:   stableExperienceID(run),
		ProjectID:      run.ProjectID,
		Site:           run.Site,
		PageStateID:    step.PageStateID,
		ActionName:     step.TargetName,
		FailureType:    "task_failed",
		FailureSummary: TruncateSummary(nonEmpty(run.Summary, "Task failed.")),
		AvoidHint:      TruncateSummary(nonEmpty(step.ResultSummary, "Avoid repeating the same failed branch.")),
	}
	if err := service.repo.SaveFailureMemory(ctx, failure); err != nil {
		return registry.FailureMemory{}, err
	}
	service.recordExperienceFailureSignal(ctx, run, step, now)
	failures, err := service.repo.SearchFailureMemories(ctx, registry.FailureMemorySearchQuery{
		ProjectID:   run.ProjectID,
		Site:        run.Site,
		PageStateID: failure.PageStateID,
		Limit:       1,
	})
	if err == nil && len(failures) > 0 {
		return failures[0], nil
	}
	return failure, err
}

func (service *ExperienceService) recordExperienceFailureSignal(ctx context.Context, run registry.TaskRun, step registry.ActionStep, now time.Time) {
	experience, err := service.repo.GetExperienceMemory(ctx, stableExperienceID(run))
	if err != nil {
		return
	}
	experience.FailureCount++
	experience.Confidence = experienceConfidence(experience)
	experience.LastFailureAt = now
	experience.UpdatedAt = now
	warning := TruncateSummary(nonEmpty(step.ResultSummary, run.Summary))
	if ContainsSensitiveMaterial(warning) {
		warning = "Prior run failed; avoid repeating the same branch."
	}
	if warning != "" && !containsString(experience.FailureWarnings, warning) {
		experience.FailureWarnings = append(experience.FailureWarnings, warning)
	}
	_ = service.repo.SaveExperienceMemory(ctx, experience)
}

func stableExperienceID(run registry.TaskRun) string {
	targetSignature := targetSignatureFromActionSteps(reusableSuccessfulActionSteps(run.ActionSteps))
	hash := sha1.Sum([]byte(strings.Join([]string{
		run.ProjectID,
		run.Site,
		experienceIntentKey(run.TaskTemplate),
		firstPathItem(run.OptimizedPath),
		lastPathItem(run.OptimizedPath),
		targetSignature,
	}, "\x00")))
	return "exp_" + hex.EncodeToString(hash[:8])
}

func stableFailureID(run registry.TaskRun) string {
	step := firstActionStep(run.ActionSteps)
	hash := sha1.Sum([]byte(strings.Join([]string{
		run.ProjectID,
		run.Site,
		run.TaskTemplate,
		firstPathItem(run.OptimizedPath),
		step.PageStateID,
		step.TargetName,
		run.Summary,
	}, "\x00")))
	return "fail_" + hex.EncodeToString(hash[:8])
}

func stepsSummaryFromActionSteps(steps []registry.ActionStep) []registry.ExperienceStepSummary {
	result := make([]registry.ExperienceStepSummary, 0, len(steps))
	for _, step := range steps {
		result = append(result, registry.ExperienceStepSummary{
			ActionName:    string(step.ActionType),
			TargetName:    step.TargetName,
			ValueTemplate: step.ValueTemplate,
			ResultSummary: step.ResultSummary,
			PageStateID:   step.PageStateID,
			SurfaceID:     step.SurfaceID,
			IsBranchNoise: step.IsBranchNoise,
		})
	}
	return result
}

func experienceIntentKey(taskTemplate string) string {
	value := strings.ToLower(strings.TrimSpace(taskTemplate))
	value = strings.ReplaceAll(value, "{{service_name}}", "{{var}}")
	value = strings.ReplaceAll(value, "{{value}}", "{{var}}")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "task"
	}
	return value
}

func pathSignature(path []string) string {
	return strings.Join(trimStringList(path), " -> ")
}

func targetSignatureFromActionSteps(steps []registry.ActionStep) string {
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		target := strings.TrimSpace(step.TargetName)
		if target == "" {
			continue
		}
		parts = append(parts, strings.Join([]string{
			string(step.ActionType),
			target,
			strings.TrimSpace(step.ValueTemplate),
			step.PageStateID,
			step.SurfaceID,
		}, ":"))
	}
	return strings.Join(parts, "|")
}

func mergeExperiencePathCandidate(candidates []registry.ExperiencePathCandidate, candidate registry.ExperiencePathCandidate) []registry.ExperiencePathCandidate {
	if candidate.PathSignature == "" {
		candidate.PathSignature = pathSignature(candidate.OptimizedPath)
	}
	for index := range candidates {
		if candidates[index].PathSignature != candidate.PathSignature || candidates[index].TargetSignature != candidate.TargetSignature {
			continue
		}
		previousSuccessCount := candidates[index].SuccessCount
		candidates[index].SuccessCount++
		candidates[index].AverageStepCount = rollingAverageStepCount(candidates[index].AverageStepCount, previousSuccessCount, len(candidate.OptimizedPath))
		candidates[index].LastSuccessAt = candidate.LastSuccessAt
		return candidates
	}
	return append(candidates, candidate)
}

func scoreExperiencePathCandidates(candidates []registry.ExperiencePathCandidate, failureCount int, now time.Time) []registry.ExperiencePathCandidate {
	for index := range candidates {
		candidates[index].Score = scoreExperiencePathCandidate(candidates[index], failureCount, now)
	}
	sortExperiencePathCandidates(candidates)
	if len(candidates) > 5 {
		return candidates[:5]
	}
	return candidates
}

func scoreExperiencePathCandidate(candidate registry.ExperiencePathCandidate, failureCount int, now time.Time) float64 {
	successCount := candidate.SuccessCount
	if successCount <= 0 {
		successCount = 1
	}
	total := successCount + candidate.FailureCount
	if total <= 0 {
		total = 1
	}
	successRate := float64(successCount) / float64(total)
	stepCount := candidate.AverageStepCount
	if stepCount <= 0 {
		stepCount = float64(len(candidate.OptimizedPath))
	}
	if stepCount <= 0 {
		stepCount = 1
	}
	shorterPathScore := 1 / stepCount
	recencyScore := 1.0
	if !candidate.LastSuccessAt.IsZero() {
		days := now.Sub(candidate.LastSuccessAt).Hours() / 24
		if days > 0 {
			recencyScore = 1 / (1 + days/30)
		}
	}
	failurePenalty := minFloat(float64(failureCount+candidate.FailureCount)*0.1, 1)
	return successRate*0.45 + shorterPathScore*0.35 + recencyScore*0.15 - failurePenalty*0.25
}

func sortExperiencePathCandidates(candidates []registry.ExperiencePathCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if len(candidates[i].OptimizedPath) == len(candidates[j].OptimizedPath) {
				return candidates[i].PathSignature < candidates[j].PathSignature
			}
			return len(candidates[i].OptimizedPath) < len(candidates[j].OptimizedPath)
		}
		return candidates[i].Score > candidates[j].Score
	})
}

func bestExperiencePathCandidate(candidates []registry.ExperiencePathCandidate) registry.ExperiencePathCandidate {
	if len(candidates) == 0 {
		return registry.ExperiencePathCandidate{}
	}
	sortExperiencePathCandidates(candidates)
	return candidates[0]
}

func rollingAverageStepCount(existing float64, existingCount int, newStepCount int) float64 {
	if newStepCount <= 0 {
		return existing
	}
	if existing <= 0 || existingCount <= 0 {
		return float64(newStepCount)
	}
	return ((existing * float64(existingCount)) + float64(newStepCount)) / float64(existingCount+1)
}

func experienceConfidence(experience registry.ExperienceMemory) float64 {
	total := experience.SuccessCount + experience.FailureCount + experience.MisleadingCount
	if total <= 0 {
		return 0.5
	}
	successRate := float64(experience.SuccessCount) / float64(total)
	countBoost := minFloat(float64(experience.SuccessCount)*0.03, 0.2)
	return minFloat(0.55+successRate*0.25+countBoost, 0.95)
}

func appendUniqueLimited(values []string, value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" || containsString(values, value) {
		return values
	}
	values = append(values, value)
	if limit > 0 && len(values) > limit {
		return values[len(values)-limit:]
	}
	return values
}

func reusableSuccessfulActionSteps(steps []registry.ActionStep) []registry.ActionStep {
	result := make([]registry.ActionStep, 0, len(steps))
	for _, step := range steps {
		if !step.IsReusableSuccessful() {
			continue
		}
		result = append(result, step)
	}
	return result
}

func variablesFromActionSteps(steps []registry.ActionStep) []registry.Variable {
	seen := map[string]bool{}
	result := []registry.Variable{}
	for _, step := range steps {
		for _, name := range templateVariableNames(step.ValueTemplate) {
			if seen[name] {
				continue
			}
			seen[name] = true
			result = append(result, registry.Variable{Name: name, Type: registry.VariableString, Required: true, Source: registry.VariableSourceTask})
		}
	}
	return result
}

func templateVariableNames(value string) []string {
	result := []string{}
	remaining := value
	for {
		start := strings.Index(remaining, "{{")
		end := strings.Index(remaining, "}}")
		if start == -1 || end == -1 || end <= start+2 {
			break
		}
		name := strings.TrimSpace(remaining[start+2 : end])
		if name != "" {
			result = append(result, name)
		}
		remaining = remaining[end+2:]
	}
	return result
}

func isLowRiskExperience(steps []registry.ActionStep) bool {
	for _, step := range steps {
		if strings.Contains(step.TargetName, "删除") || strings.Contains(strings.ToLower(step.TargetName), "delete") {
			return false
		}
	}
	return true
}

func firstActionStep(steps []registry.ActionStep) registry.ActionStep {
	if len(steps) == 0 {
		return registry.ActionStep{}
	}
	return steps[0]
}

func firstPathItem(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return path[0]
}

func lastPathItem(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return path[len(path)-1]
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
