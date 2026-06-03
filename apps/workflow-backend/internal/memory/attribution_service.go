package memory

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

const (
	SignalPathAligned      = "pathAligned"
	SignalTargetAligned    = "targetAligned"
	SignalSelectorFailure  = "selectorFailure"
	SignalBacktracked      = "backtracked"
	SignalBranchNoise      = "branchNoise"
	SignalPageRuleMismatch = "pageRuleMismatch"
	SignalExplorationOnly  = "explorationOnly"
)

type AttributionInput struct {
	TaskRun         registry.TaskRun
	ContextEvent    registry.MemoryContextEvent
	EvidenceRefs    []registry.MemoryEvidenceRef
	PageObservation *NormalizedPageObservation
}

type AttributionResult struct {
	Events []registry.MemoryAttributionEvent `json:"events,omitempty"`
	Stats  []registry.MemoryEvidenceStats    `json:"stats,omitempty"`
}

type AttributionService struct {
	repo registry.Repository
}

func NewAttributionService(repo registry.Repository) *AttributionService {
	return &AttributionService{repo: repo}
}

func (service *AttributionService) Evaluate(input AttributionInput) []registry.MemoryAttributionEvent {
	run := input.TaskRun
	refs := attributionEvidenceRefs(input)
	events := make([]registry.MemoryAttributionEvent, 0, len(refs))
	for _, ref := range refs {
		signals := service.evidenceSignals(input, ref)
		adoption := service.evidenceAdoption(input, ref, signals)
		label := attributionLabel(run, signals)
		events = append(events, registry.MemoryAttributionEvent{
			ID:             stableAttributionEventID(run, input.ContextEvent, ref),
			ProjectID:      firstNonEmpty(run.ProjectID, input.ContextEvent.ProjectID, "default"),
			Site:           run.Site,
			TaskRunID:      run.ID,
			ContextID:      firstNonEmpty(run.MemoryContextID, input.ContextEvent.ID),
			EvidenceID:     ref.ID,
			EvidenceSource: ref.Source,
			Label:          label,
			Reason:         attributionReason(label, signals),
			Signals:        signals,
			Adoption:       adoption,
			CreatedAt:      time.Now().UTC(),
		})
	}
	return events
}

func (service *AttributionService) evidenceAdoption(input AttributionInput, ref registry.MemoryEvidenceRef, signals []string) registry.MemoryAdoptionSignal {
	run := input.TaskRun
	targetNames := evidenceTargetNames(ref)
	evidencePath := evidenceOptimizedPath(ref)
	matchedSteps := matchedTargetActionSteps(targetNames, run.ActionSteps)
	adoption := registry.MemoryAdoptionSignal{
		EvidenceID: ref.ID,
	}
	hasSignal := signalSet(signals)
	adoption.AdoptedPath = hasSignal[SignalPathAligned]
	adoption.AdoptedTarget = len(matchedSteps) > 0
	adoption.FirstAdoptedStepIndex = firstMatchedStepIndex(matchedSteps)
	adoption.CausedSelectorFailure = anySelectorFailure(matchedSteps)
	adoption.CausedBacktrack = anyBranchNoise(matchedSteps) || (runBacktracked(run) && evidencePathBacktracked(evidencePath, run))
	adoption.LaterAbandoned = adoption.CausedBacktrack
	adoption.FinalPathCameFromMemory = adoption.AdoptedPath && !adoption.CausedBacktrack && !adoption.CausedSelectorFailure
	adoption.AdoptedSurface = evidenceSurfaceAdopted(input.ContextEvent.CurrentSurface, evidenceSurfaceIDs(ref))
	return adoption
}

func (service *AttributionService) EvaluateAndPersist(ctx context.Context, input AttributionInput) (AttributionResult, error) {
	events := service.Evaluate(input)
	if service.repo == nil {
		return AttributionResult{Events: events}, errors.New("workflow registry is not configured")
	}
	refs := attributionEvidenceRefs(input)
	stats := make([]registry.MemoryEvidenceStats, 0, len(events))
	for _, event := range events {
		if err := service.repo.SaveMemoryAttributionEvent(ctx, event); err != nil {
			return AttributionResult{}, err
		}
		updated, err := service.updateEvidenceStats(ctx, input.TaskRun, event)
		if err != nil {
			return AttributionResult{}, err
		}
		if err := service.updateSiteTaskGuideFeedback(ctx, input.TaskRun, event, findAttributionEvidenceRef(refs, event)); err != nil {
			return AttributionResult{}, err
		}
		stats = append(stats, updated)
	}
	return AttributionResult{Events: events, Stats: stats}, nil
}

func (service *AttributionService) updateEvidenceStats(ctx context.Context, run registry.TaskRun, event registry.MemoryAttributionEvent) (registry.MemoryEvidenceStats, error) {
	projectID := firstNonEmpty(event.ProjectID, run.ProjectID, "default")
	stats, err := service.repo.GetMemoryEvidenceStats(ctx, projectID, event.EvidenceSource, event.EvidenceID)
	if err != nil {
		stats = registry.MemoryEvidenceStats{
			ProjectID:      projectID,
			Site:           firstNonEmpty(event.Site, run.Site),
			EvidenceID:     event.EvidenceID,
			EvidenceSource: event.EvidenceSource,
		}
	}
	stats.Site = firstNonEmpty(stats.Site, event.Site, run.Site)
	switch event.Label {
	case registry.MemoryAttributionHelpful:
		stats.HelpfulCount++
		stats.UtilityScore += 1.00
	case registry.MemoryAttributionMisleading:
		stats.MisleadingCount++
		stats.UtilityScore -= 1.20
	case registry.MemoryAttributionStale:
		stats.StaleCount++
		stats.UtilityScore -= 1.50
	case registry.MemoryAttributionUnused:
		stats.UnusedCount++
		stats.UtilityScore -= 0.05
	case registry.MemoryAttributionNeutral:
		stats.NeutralCount++
	}
	stats.UtilityScore = clampFloat(stats.UtilityScore, -5, 5)
	stats.LastFeedbackAt = time.Now().UTC()
	if err := service.repo.SaveMemoryEvidenceStats(ctx, stats); err != nil {
		return registry.MemoryEvidenceStats{}, err
	}
	return stats, nil
}

func (service *AttributionService) updateSiteTaskGuideFeedback(ctx context.Context, run registry.TaskRun, event registry.MemoryAttributionEvent, ref registry.MemoryEvidenceRef) error {
	if event.EvidenceSource != registry.MemoryEvidenceSourceGuide {
		return nil
	}
	label, ok := siteTaskGuideFeedbackLabel(event)
	if !ok {
		return nil
	}
	if _, ok := service.repo.(registry.SiteTaskGuideRepository); !ok {
		return nil
	}
	serviceWithGuide := NewSiteTaskGuideService(service.repo)
	matchedStepCount := 0
	if event.Adoption.AdoptedTarget {
		matchedStepCount = 1
	}
	backtrackCount := 0
	if event.Adoption.CausedBacktrack {
		backtrackCount = 1
	}
	stateID := evidenceMatchedStateID(ref)
	feedbackID := strings.Join([]string{
		"guide_feedback",
		firstNonEmpty(run.ID, "run"),
		firstNonEmpty(event.ContextID, "ctx"),
		event.EvidenceID,
		firstNonEmpty(stateID, "guide"),
		string(label),
	}, "_")
	return serviceWithGuide.ApplyFeedback(ctx, event.EvidenceID, registry.SiteTaskGuideFeedback{
		ID:               feedbackID,
		StateID:          stateID,
		TaskRunID:        run.ID,
		ContextID:        event.ContextID,
		Label:            label,
		Reason:           event.Reason,
		MatchedStepCount: matchedStepCount,
		BacktrackCount:   backtrackCount,
	})
}

func siteTaskGuideFeedbackLabel(event registry.MemoryAttributionEvent) (registry.SiteTaskGuideFeedbackLabel, bool) {
	if containsString(event.Signals, SignalPageRuleMismatch) {
		return registry.SiteTaskGuideFeedbackAbandonedMismatch, true
	}
	switch event.Label {
	case registry.MemoryAttributionHelpful:
		return registry.SiteTaskGuideFeedbackUsedHelpful, true
	case registry.MemoryAttributionUnused:
		return registry.SiteTaskGuideFeedbackUnused, true
	case registry.MemoryAttributionMisleading:
		return registry.SiteTaskGuideFeedbackUsedMisleading, true
	case registry.MemoryAttributionStale:
		return registry.SiteTaskGuideFeedbackStale, true
	default:
		return "", false
	}
}

func (service *AttributionService) evidenceSignals(input AttributionInput, ref registry.MemoryEvidenceRef) []string {
	signals := map[string]bool{}
	run := input.TaskRun
	targetNames := evidenceTargetNames(ref)
	evidencePath := evidenceOptimizedPath(ref)
	if len(evidencePath) == 0 && ref.PageStateID != "" {
		evidencePath = append([]string{ref.PageStateID}, evidencePath...)
	}
	if evidencePathAligned(evidencePath, run) {
		signals[SignalPathAligned] = true
	}
	matchedTargetSteps := matchedTargetActionSteps(targetNames, run.ActionSteps)
	if len(matchedTargetSteps) > 0 {
		signals[SignalTargetAligned] = true
	}
	if anyBranchNoise(matchedTargetSteps) {
		signals[SignalBranchNoise] = true
	}
	if anySelectorFailure(matchedTargetSteps) {
		signals[SignalSelectorFailure] = true
	}
	if runBacktracked(run) && evidencePathBacktracked(evidencePath, run) {
		signals[SignalBacktracked] = true
	}
	if pageRuleMismatchApplies(ref, input.PageObservation) && pageRuleMismatch(targetNames, input.PageObservation, run.ActionSteps) {
		signals[SignalPageRuleMismatch] = true
	}
	if len(targetNames) == 0 && len(trimStringList(evidencePath)) == 0 {
		signals[SignalExplorationOnly] = true
	}
	if len(run.ActionSteps) == 0 && len(run.OriginalPath) == 0 && len(run.OptimizedPath) == 0 {
		signals[SignalExplorationOnly] = true
	}
	return sortedSignals(signals)
}

func attributionEvidenceRefs(input AttributionInput) []registry.MemoryEvidenceRef {
	switch {
	case len(input.EvidenceRefs) > 0:
		return input.EvidenceRefs
	case len(input.TaskRun.MemoryEvidenceRefs) > 0:
		return input.TaskRun.MemoryEvidenceRefs
	default:
		return input.ContextEvent.EvidenceRefs
	}
}

func findAttributionEvidenceRef(refs []registry.MemoryEvidenceRef, event registry.MemoryAttributionEvent) registry.MemoryEvidenceRef {
	for _, ref := range refs {
		if ref.ID == event.EvidenceID && ref.Source == event.EvidenceSource {
			return ref
		}
	}
	return registry.MemoryEvidenceRef{}
}

func evidenceMatchedStateID(ref registry.MemoryEvidenceRef) string {
	return firstNonEmpty(
		payloadString(ref.Payload, "matchedStateId"),
		payloadString(ref.Payload, "stateId"),
	)
}

func attributionLabel(run registry.TaskRun, signals []string) registry.MemoryAttributionLabel {
	hasSignal := signalSet(signals)
	adopted := hasSignal[SignalPathAligned] || hasSignal[SignalTargetAligned]
	if hasSignal[SignalPageRuleMismatch] && !hasSignal[SignalTargetAligned] {
		return registry.MemoryAttributionStale
	}
	if !adopted {
		if hasSignal[SignalExplorationOnly] {
			return registry.MemoryAttributionNeutral
		}
		return registry.MemoryAttributionUnused
	}
	if hasSignal[SignalBranchNoise] || hasSignal[SignalSelectorFailure] || hasSignal[SignalBacktracked] {
		return registry.MemoryAttributionMisleading
	}
	if len(run.ActionSteps) == 0 {
		return registry.MemoryAttributionNeutral
	}
	return registry.MemoryAttributionHelpful
}

func pageRuleMismatchApplies(ref registry.MemoryEvidenceRef, observation *NormalizedPageObservation) bool {
	if observation == nil || observation.PageStateID == "" || ref.PageStateID == "" {
		return true
	}
	return observation.PageStateID == ref.PageStateID
}

func attributionReason(label registry.MemoryAttributionLabel, signals []string) string {
	switch label {
	case registry.MemoryAttributionHelpful:
		return "Evidence aligned with the executed path or target without immediate failure."
	case registry.MemoryAttributionMisleading:
		return "Evidence was adopted and correlated with branch noise, backtracking, or selector failure."
	case registry.MemoryAttributionStale:
		return "Evidence target or page rules did not match the current page observation."
	case registry.MemoryAttributionUnused:
		return "Evidence was recalled but the executed path and targets did not adopt it."
	default:
		if len(signals) == 0 {
			return "Not enough execution evidence to score the recalled memory."
		}
		return "Signals were insufficient for a positive or negative attribution."
	}
}

func evidenceOptimizedPath(ref registry.MemoryEvidenceRef) []string {
	return stringListFromPayload(ref.Payload, "optimizedPath")
}

func evidenceTargetNames(ref registry.MemoryEvidenceRef) []string {
	targets := stringListFromPayload(ref.Payload, "targetNames")
	if len(targets) == 0 {
		targets = append(targets, stringListFromPayload(ref.Payload, "targets")...)
	}
	if len(targets) == 0 {
		targets = append(targets, stepTargetNamesFromPayload(ref.Payload)...)
	}
	return compactStrings(targets)
}

func stepTargetNamesFromPayload(payload map[string]any) []string {
	value, ok := payload["stepTargets"]
	if !ok || value == nil {
		return nil
	}
	result := []string{}
	switch typed := value.(type) {
	case []registry.MemoryStepTarget:
		for _, target := range typed {
			result = append(result, target.TargetName)
		}
	case []any:
		for _, item := range typed {
			if fields, ok := item.(map[string]any); ok {
				if name, ok := fields["targetName"].(string); ok {
					result = append(result, name)
				}
			}
		}
	}
	return result
}

func evidenceSurfaceAdopted(currentSurfaceID string, evidenceSurfaceIDs []string) bool {
	currentSurfaceID = strings.TrimSpace(currentSurfaceID)
	if currentSurfaceID == "" || len(evidenceSurfaceIDs) == 0 {
		return false
	}
	return containsString(evidenceSurfaceIDs, currentSurfaceID)
}

func evidenceSurfaceIDs(ref registry.MemoryEvidenceRef) []string {
	if ref.Payload == nil {
		return nil
	}
	ids := []string{}
	if id := payloadString(ref.Payload, "surfaceId"); id != "" {
		ids = append(ids, id)
	}
	ids = append(ids, stringListFromPayload(ref.Payload, "surfaceIds")...)
	value, ok := ref.Payload["stepTargets"]
	if !ok || value == nil {
		return compactStrings(ids)
	}
	switch typed := value.(type) {
	case []registry.MemoryStepTarget:
		for _, target := range typed {
			ids = append(ids, target.SurfaceID)
		}
	case []any:
		for _, item := range typed {
			if fields, ok := item.(map[string]any); ok {
				if id, ok := fields["surfaceId"].(string); ok {
					ids = append(ids, id)
				}
			}
		}
	}
	return compactStrings(ids)
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func firstMatchedStepIndex(steps []registry.ActionStep) int {
	first := 0
	for _, step := range steps {
		if step.StepIndex <= 0 {
			continue
		}
		if first == 0 || step.StepIndex < first {
			first = step.StepIndex
		}
	}
	return first
}

func stringListFromPayload(payload map[string]any, key string) []string {
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func evidencePathAligned(evidencePath []string, run registry.TaskRun) bool {
	path := trimStringList(evidencePath)
	if len(path) == 0 {
		return false
	}
	actualPaths := [][]string{run.OptimizedPath, run.OriginalPath}
	for _, actual := range actualPaths {
		actual = trimStringList(actual)
		if len(actual) == 0 {
			continue
		}
		if len(path) == 1 && anyPathOverlap(actual, path) {
			return true
		}
		if containsSubsequence(actual, path) {
			return true
		}
	}
	return false
}

func evidencePathBacktracked(evidencePath []string, run registry.TaskRun) bool {
	path := trimStringList(evidencePath)
	if len(path) == 0 {
		return false
	}
	return pathAlignedInPath(path, run.OriginalPath) && !pathAlignedInPath(path, run.OptimizedPath)
}

func pathAlignedInPath(expected []string, actual []string) bool {
	actual = trimStringList(actual)
	if len(actual) == 0 {
		return false
	}
	if len(expected) == 1 {
		return anyPathOverlap(actual, expected)
	}
	return containsSubsequence(actual, expected)
}

func containsSubsequence(actual []string, expected []string) bool {
	if len(expected) == 0 || len(expected) > len(actual) {
		return false
	}
	for start := 0; start <= len(actual)-len(expected); start++ {
		matched := true
		for offset := range expected {
			if actual[start+offset] != expected[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func anyPathOverlap(actual []string, expected []string) bool {
	actualSet := map[string]bool{}
	for _, page := range actual {
		actualSet[page] = true
	}
	for _, page := range expected {
		if actualSet[page] {
			return true
		}
	}
	return false
}

func matchedTargetActionSteps(targetNames []string, steps []registry.ActionStep) []registry.ActionStep {
	result := []registry.ActionStep{}
	for _, step := range steps {
		if targetMatchesAny(step.TargetName, targetNames) {
			result = append(result, step)
		}
	}
	return result
}

func targetMatchesAny(target string, candidates []string) bool {
	target = normalizeSignalText(target)
	if target == "" {
		return false
	}
	for _, candidate := range candidates {
		candidate = normalizeSignalText(candidate)
		if candidate == "" {
			continue
		}
		if target == candidate || strings.Contains(target, candidate) || strings.Contains(candidate, target) {
			return true
		}
	}
	return false
}

func anyBranchNoise(steps []registry.ActionStep) bool {
	for _, step := range steps {
		if step.IsBranchNoise {
			return true
		}
	}
	return false
}

func anySelectorFailure(steps []registry.ActionStep) bool {
	for _, step := range steps {
		if selectorFailure(step.ResultSummary) {
			return true
		}
	}
	return false
}

func selectorFailure(summary string) bool {
	lower := strings.ToLower(summary)
	failures := []string{
		"not found",
		"not visible",
		"dom tree not indexed",
		"failed to click",
		"failed to input",
	}
	for _, failure := range failures {
		if strings.Contains(lower, failure) {
			return true
		}
	}
	return false
}

func runBacktracked(run registry.TaskRun) bool {
	if len(run.OptimizedPath) > 0 && len(run.OriginalPath) > len(run.OptimizedPath) {
		return true
	}
	seen := map[string]int{}
	for index, page := range run.OriginalPath {
		if first, ok := seen[page]; ok && index-first > 1 {
			return true
		}
		if _, ok := seen[page]; !ok {
			seen[page] = index
		}
	}
	return false
}

func pageRuleMismatch(targetNames []string, observation *NormalizedPageObservation, steps []registry.ActionStep) bool {
	if observation == nil || len(targetNames) == 0 {
		return false
	}
	for _, step := range steps {
		if targetMatchesAny(step.TargetName, targetNames) {
			return false
		}
	}
	for _, control := range observation.Controls {
		if controlMatchesAny(control.Name, targetNames) {
			return false
		}
	}
	return true
}

func controlMatchesAny(controlName string, candidates []string) bool {
	controlName = normalizeSignalText(controlName)
	if controlName == "" {
		return false
	}
	for _, candidate := range candidates {
		candidate = normalizeSignalText(candidate)
		if candidate == "" {
			continue
		}
		if controlName == candidate || strings.Contains(controlName, candidate) {
			return true
		}
	}
	return false
}

func sortedSignals(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value, enabled := range values {
		if enabled {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func signalSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func stableAttributionEventID(run registry.TaskRun, contextEvent registry.MemoryContextEvent, ref registry.MemoryEvidenceRef) string {
	hash := sha1.Sum([]byte(strings.Join([]string{
		firstNonEmpty(run.ProjectID, contextEvent.ProjectID, "default"),
		run.ID,
		firstNonEmpty(run.MemoryContextID, contextEvent.ID),
		string(ref.Source),
		ref.ID,
	}, "\x00")))
	return "attr_" + hex.EncodeToString(hash[:8])
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func trimStringList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func normalizeSignalText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clampFloat(value, min, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}

func navigationEvidenceID(from, to, action string) string {
	hash := sha1.Sum([]byte(strings.Join([]string{from, to, action}, "\x00")))
	return fmt.Sprintf("nav:%s:%s:%s", from, to, hex.EncodeToString(hash[:4]))
}
