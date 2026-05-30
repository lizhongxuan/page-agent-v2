package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type MemoryContextService struct {
	repo registry.Repository
}

func NewMemoryContextService(repo registry.Repository) *MemoryContextService {
	return &MemoryContextService{repo: repo}
}

func (service *MemoryContextService) GetContext(ctx context.Context, request MemoryContextRequest) (MemoryContextResponse, error) {
	if service.repo == nil {
		return MemoryContextResponse{}, errors.New("workflow registry is not configured")
	}
	projectID := strings.TrimSpace(request.ProjectID)
	if projectID == "" {
		projectID = "default"
	}
	currentPage := service.resolveCurrentPage(ctx, projectID, request)
	site := ""
	if currentPage != nil {
		site = currentPage.Site
	}
	if site == "" && request.CurrentURL != "" {
		_, site = normalizeObservationURL(request.CurrentURL)
	}
	module := moduleFromRequest(request, currentPage)
	currentSurface := service.resolveCurrentSurface(ctx, projectID, site, currentPage, request)
	currentSurfaceIDValue := currentSurfaceID(currentSurface)

	response := MemoryContextResponse{
		ContextID:       newMemoryContextID(projectID, request.Task, request.CurrentURL),
		ProjectID:       projectID,
		CurrentPage:     pageSummary(currentPage),
		CurrentSurface:  surfaceSummary(currentSurface),
		RecommendedMode: registry.MemoryModeNormal,
	}
	debug := &MemoryContextDebug{PromptBudget: MemoryPromptBudgetDebug{MaxChars: defaultMemoryPromptBudgetChars}}
	if profile, err := service.repo.GetBusinessSystemProfile(ctx, registry.BusinessSystemProfileQuery{
		ProjectID:  projectID,
		Site:       site,
		Module:     module,
		SourceType: registry.MemorySourceProduction,
	}); err == nil && safeText(profile.Summary) != "" {
		response.BusinessContext = &BusinessContext{Summary: profile.Summary}
		if site == "" {
			site = profile.Site
		}
	}
	response.KnowledgeEvidence = service.knowledgeEvidence(ctx, registry.KnowledgeSearchQuery{
		ProjectID:   projectID,
		Site:        site,
		Module:      module,
		Task:        request.Task,
		URL:         request.CurrentURL,
		Title:       observationTitle(request),
		VisibleText: observationVisibleText(request),
		Limit:       10,
	})
	if request.CurrentURL != "" {
		appendFilteredEvidence(debug, "knowledge", "", "hard_gate_applied")
	}
	response.KnowledgeEvidence = service.applyKnowledgeUtility(ctx, projectID, site, response.KnowledgeEvidence, debug)
	if currentPage != nil {
		response.NavigationHints = service.navigationHints(ctx, projectID, site, currentPage.ID)
		response.NavigationHints = service.applyNavigationUtility(ctx, projectID, site, response.NavigationHints, debug)
		response.FailureWarnings = service.failureWarnings(ctx, registry.FailureMemorySearchQuery{
			ProjectID:   projectID,
			Site:        site,
			PageStateID: currentPage.ID,
			Task:        request.Task,
			Limit:       10,
		})
		response.FailureWarnings = service.applyFailureUtility(ctx, projectID, site, response.FailureWarnings, debug)
		response.ExperienceHints = service.experienceHints(ctx, registry.ExperienceMemorySearchQuery{
			ProjectID:      projectID,
			Site:           site,
			StartPageState: currentPage.ID,
			Task:           request.Task,
			Limit:          memoryCandidateLimit,
		})
		response.ExperienceHints = service.applyExperienceUtility(ctx, projectID, site, currentSurfaceIDValue, response.ExperienceHints, debug)
	}
	response = fitMemoryContextToBudget(response, debug, defaultMemoryPromptBudgetChars)
	response.RecommendedMode = recommendedMode(response)
	response.EvidenceRefs = buildMemoryEvidenceRefs(response)
	response.ContextPrompt = FormatMemoryContextPrompt(response)
	debug.PromptChars = len([]rune(response.ContextPrompt))
	debug.PromptBudget.UsedChars = debug.PromptChars
	response.Debug = debug
	if err := service.repo.SaveMemoryContextEvent(ctx, registry.MemoryContextEvent{
		ID:                        response.ContextID,
		ProjectID:                 projectID,
		Task:                      request.Task,
		CurrentURL:                request.CurrentURL,
		CurrentPageState:          currentPageID(currentPage),
		CurrentSurface:            currentSurfaceID(currentSurface),
		ContextPrompt:             response.ContextPrompt,
		SelectedExperienceIDs:     experienceHintIDs(response.ExperienceHints),
		SelectedKnowledgeChunkIDs: knowledgeEvidenceIDs(response.KnowledgeEvidence),
		EvidenceRefs:              response.EvidenceRefs,
		RecommendedMode:           response.RecommendedMode,
		Payload: map[string]any{
			"mode":           request.Mode,
			"debug":          response.Debug,
			"currentSurface": currentSurfaceIDValue,
		},
	}); err != nil {
		return MemoryContextResponse{}, err
	}
	return response, nil
}

const (
	defaultMemoryPromptBudgetChars = 5000
	memoryCandidateLimit           = 50
)

func (service *MemoryContextService) applyKnowledgeUtility(ctx context.Context, projectID, site string, evidence []KnowledgeEvidence, debug *MemoryContextDebug) []KnowledgeEvidence {
	items := make([]KnowledgeEvidence, 0, len(evidence))
	for _, item := range evidence {
		stats, found := service.memoryEvidenceStats(ctx, projectID, registry.MemoryEvidenceSourceKnowledge, item.ChunkID)
		if shouldFilterEvidence(stats, found) {
			appendFilteredEvidence(debug, "knowledge", item.ChunkID, evidenceFilterReason(stats, found))
			continue
		}
		item.Score = rerankedScore(item.Score, stats, found)
		items = append(items, item)
		_ = site
	}
	sortKnowledgeEvidence(items)
	return limitKnowledgeEvidence(items, 3)
}

func (service *MemoryContextService) applyNavigationUtility(ctx context.Context, projectID, site string, hints []NavigationHint, debug *MemoryContextDebug) []NavigationHint {
	items := make([]NavigationHint, 0, len(hints))
	for _, item := range hints {
		id := navigationEvidenceID(item.From, item.To, item.Action)
		stats, found := service.memoryEvidenceStats(ctx, projectID, registry.MemoryEvidenceSourceNavigation, id)
		if shouldFilterEvidence(stats, found) {
			appendFilteredEvidence(debug, "navigation", id, evidenceFilterReason(stats, found))
			continue
		}
		item.Confidence = rerankedScore(item.Confidence, stats, found)
		items = append(items, item)
		_ = site
	}
	sortNavigationHints(items)
	return limitNavigationHints(items, 3)
}

func (service *MemoryContextService) applyExperienceUtility(ctx context.Context, projectID, site, currentSurfaceID string, hints []ExperienceHint, debug *MemoryContextDebug) []ExperienceHint {
	type scoredExperience struct {
		hint         ExperienceHint
		filterReason string
	}
	items := make([]scoredExperience, 0, len(hints))
	for _, item := range hints {
		if experienceSurfaceMismatch(item, currentSurfaceID) {
			appendFilteredEvidence(debug, "experience", item.ID, "surface_mismatch")
			continue
		}
		stats, found := service.memoryEvidenceStats(ctx, projectID, registry.MemoryEvidenceSourceExperience, item.ID)
		item.Confidence = rerankedScore(item.Confidence, stats, found)
		items = append(items, scoredExperience{hint: item, filterReason: evidenceFilterReason(stats, found)})
		_ = site
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].hint.Confidence == items[j].hint.Confidence {
			return items[i].hint.ID < items[j].hint.ID
		}
		return items[i].hint.Confidence > items[j].hint.Confidence
	})
	result := make([]ExperienceHint, 0, len(items))
	for _, item := range items {
		if item.filterReason != "" {
			appendFilteredEvidence(debug, "experience", item.hint.ID, item.filterReason)
			continue
		}
		result = append(result, item.hint)
	}
	return limitExperienceHints(result, 3)
}

func (service *MemoryContextService) applyFailureUtility(ctx context.Context, projectID, site string, warnings []FailureWarning, debug *MemoryContextDebug) []FailureWarning {
	items := make([]FailureWarning, 0, len(warnings))
	for _, item := range warnings {
		stats, found := service.memoryEvidenceStats(ctx, projectID, registry.MemoryEvidenceSourceFailure, item.ID)
		if shouldFilterEvidence(stats, found) {
			appendFilteredEvidence(debug, "failure", item.ID, evidenceFilterReason(stats, found))
			continue
		}
		items = append(items, item)
		_ = site
	}
	return limitFailureWarnings(items, 3)
}

func (service *MemoryContextService) memoryEvidenceStats(ctx context.Context, projectID string, source registry.MemoryEvidenceSource, evidenceID string) (registry.MemoryEvidenceStats, bool) {
	if evidenceID == "" {
		return registry.MemoryEvidenceStats{}, false
	}
	stats, err := service.repo.GetMemoryEvidenceStats(ctx, projectID, source, evidenceID)
	if err != nil {
		return registry.MemoryEvidenceStats{}, false
	}
	return stats, true
}

func shouldFilterEvidence(stats registry.MemoryEvidenceStats, found bool) bool {
	return evidenceFilterReason(stats, found) != ""
}

func evidenceFilterReason(stats registry.MemoryEvidenceStats, found bool) string {
	if !found {
		return ""
	}
	if stats.StaleCount >= 3 {
		return "stale"
	}
	if stats.MisleadingCount >= 3 {
		return "misleading"
	}
	return ""
}

func rerankedScore(base float64, stats registry.MemoryEvidenceStats, found bool) float64 {
	if !found {
		return base
	}
	utilityAdjustment := stats.UtilityScore * 0.15
	if utilityAdjustment > 0.75 {
		utilityAdjustment = 0.75
	}
	penalty := 0.0
	if stats.StaleCount > 0 {
		penalty += minFloat(float64(stats.StaleCount)*0.1, 0.5)
	}
	if stats.MisleadingCount > 0 {
		penalty += minFloat(float64(stats.MisleadingCount)*0.1, 0.5)
	}
	return base + utilityAdjustment - penalty
}

func (service *MemoryContextService) resolveCurrentPage(ctx context.Context, projectID string, request MemoryContextRequest) *registry.PageState {
	if request.CurrentPageStateID != "" {
		if page, err := service.repo.GetPageState(ctx, request.CurrentPageStateID); err == nil {
			return &page
		}
	}
	currentURL := request.CurrentURL
	title := ""
	controls := []PageObservationControl{}
	if request.PageObservation != nil {
		title = request.PageObservation.Title
		controls = request.PageObservation.Controls
	}
	if currentURL == "" {
		return nil
	}
	normalized, err := NormalizePageObservation(PageObservationRequest{
		ProjectID: projectID,
		URL:       currentURL,
		Title:     title,
		Controls:  controls,
	})
	if err != nil {
		return nil
	}
	pages, err := service.repo.ListPageStates(ctx, registry.PageStateListQuery{ProjectID: projectID, Site: normalized.Site})
	if err != nil {
		return nil
	}
	for _, page := range pages {
		if page.URLPattern == normalized.URLPattern &&
			(title == "" || strings.EqualFold(page.CanonicalTitle, title)) &&
			pageHardRulesMatch(page, normalized) {
			return &page
		}
	}
	return nil
}

func (service *MemoryContextService) resolveCurrentSurface(ctx context.Context, projectID, site string, currentPage *registry.PageState, request MemoryContextRequest) *registry.PageSurface {
	if currentPage == nil || request.PageObservation == nil {
		return nil
	}
	normalized, err := NormalizePageObservation(PageObservationRequest{
		ProjectID:   projectID,
		URL:         request.CurrentURL,
		Title:       request.PageObservation.Title,
		VisibleText: request.PageObservation.VisibleText,
		Controls:    request.PageObservation.Controls,
	})
	if err != nil || normalized.Surface == nil {
		return nil
	}
	fingerprint := BuildSurfaceFingerprint(normalized, *normalized.Surface)
	surfaces, err := service.repo.ListPageSurfaces(ctx, registry.PageSurfaceListQuery{
		ProjectID:         projectID,
		Site:              site,
		ParentPageStateID: currentPage.ID,
		SurfaceType:       normalized.Surface.Type,
	})
	if err != nil {
		return nil
	}
	for _, surface := range surfaces {
		if surface.SurfaceFingerprint == fingerprint {
			return &surface
		}
	}
	return nil
}

func pageHardRulesMatch(page registry.PageState, observation NormalizedPageObservation) bool {
	rules := page.HardRules
	if rules.URLPattern != "" && rules.URLPattern != observation.URLPattern {
		return false
	}
	for _, include := range rules.URLIncludes {
		if include != "" && !strings.Contains(observation.URL, include) && !strings.Contains(observation.URLPattern, include) {
			return false
		}
	}
	observedText := strings.ToLower(strings.Join([]string{observation.Title, observation.VisibleTextSample}, "\n"))
	requiredText := rules.TextAll
	if len(requiredText) == 0 {
		requiredText = page.RequiredText
	}
	for _, text := range requiredText {
		text = strings.TrimSpace(text)
		if text != "" && !strings.Contains(observedText, strings.ToLower(text)) {
			return false
		}
	}
	requiredControls := rules.ControlsAll
	if len(requiredControls) == 0 {
		requiredControls = page.RequiredControls
	}
	if len(requiredControls) > 0 && !allControlsPresent(observation.Controls, requiredControls) {
		return false
	}
	if len(rules.ControlsAny) > 0 && !anyControlPresent(observation.Controls, rules.ControlsAny) {
		return false
	}
	return true
}

func allControlsPresent(observed []registry.ControlSignature, required []registry.ControlSignature) bool {
	for _, control := range required {
		if !controlPresent(observed, control) {
			return false
		}
	}
	return true
}

func anyControlPresent(observed []registry.ControlSignature, required []registry.ControlSignature) bool {
	for _, control := range required {
		if controlPresent(observed, control) {
			return true
		}
	}
	return false
}

func controlPresent(observed []registry.ControlSignature, required registry.ControlSignature) bool {
	for _, control := range observed {
		if strings.EqualFold(control.Role, required.Role) && strings.EqualFold(control.Name, required.Name) {
			return true
		}
	}
	return false
}

func (service *MemoryContextService) knowledgeEvidence(ctx context.Context, query registry.KnowledgeSearchQuery) []KnowledgeEvidence {
	chunks, err := service.repo.SearchKnowledgeChunks(ctx, query)
	if err != nil {
		return nil
	}
	result := make([]KnowledgeEvidence, 0, len(chunks))
	for _, chunk := range limitKnowledge(chunks, 3) {
		result = append(result, KnowledgeEvidence{
			ChunkID: chunk.ID,
			Title:   chunk.Title,
			Snippet: chunk.ChunkText,
			Score:   chunk.Score,
		})
	}
	return result
}

func (service *MemoryContextService) navigationHints(ctx context.Context, projectID, site, pageStateID string) []NavigationHint {
	transitions, err := service.repo.ListPageTransitions(ctx, registry.PageTransitionListQuery{
		ProjectID:       projectID,
		Site:            site,
		FromPageStateID: pageStateID,
	})
	if err != nil {
		return nil
	}
	result := make([]NavigationHint, 0, len(transitions))
	for _, transition := range limitTransitions(transitions, 3) {
		result = append(result, NavigationHint{
			From:       transition.FromPageState,
			To:         transition.ToPageState,
			Action:     strings.TrimSpace(transition.ActionName + " " + transition.TargetName),
			Confidence: 0.86,
		})
	}
	return result
}

func (service *MemoryContextService) experienceHints(ctx context.Context, query registry.ExperienceMemorySearchQuery) []ExperienceHint {
	memories, err := service.repo.SearchExperienceMemories(ctx, query)
	if err != nil {
		return nil
	}
	result := make([]ExperienceHint, 0, len(memories))
	for _, memory := range memories {
		result = append(result, ExperienceHint{
			ID:            memory.ID,
			Summary:       memory.Summary,
			OptimizedPath: memory.OptimizedPath,
			Variables:     variableNames(memory.Variables),
			StepTargets:   memoryStepTargets(memory.StepsSummary),
			Confidence:    0.8 + minFloat(memory.Score, 1)*0.1,
		})
	}
	return result
}

func (service *MemoryContextService) failureWarnings(ctx context.Context, query registry.FailureMemorySearchQuery) []FailureWarning {
	memories, err := service.repo.SearchFailureMemories(ctx, query)
	if err != nil {
		return nil
	}
	result := make([]FailureWarning, 0, len(memories))
	for _, memory := range limitFailures(memories, 3) {
		result = append(result, FailureWarning{ID: memory.ID, Summary: memory.FailureSummary, AvoidHint: memory.AvoidHint})
	}
	return result
}

func recommendedMode(response MemoryContextResponse) registry.MemoryMode {
	if response.CurrentPage != nil || len(response.NavigationHints) > 0 || len(response.ExperienceHints) > 0 || len(response.FailureWarnings) > 0 || len(response.KnowledgeEvidence) > 0 {
		return registry.MemoryModeGuided
	}
	return registry.MemoryModeNormal
}

func newMemoryContextID(projectID, task, currentURL string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "ctx_" + hex.EncodeToString(bytes[:])
	}
	return stableTransitionID(projectID, strings.Join([]string{task, time.Now().UTC().Format(time.RFC3339Nano)}, "\x00"), currentURL)
}

func pageSummary(page *registry.PageState) *MemoryPageSummary {
	if page == nil {
		return nil
	}
	return &MemoryPageSummary{
		ID:      page.ID,
		Name:    page.CanonicalTitle,
		Summary: TruncateSummary(strings.TrimSpace(strings.Join(append([]string{page.CanonicalTitle}, page.RequiredText...), " "))),
	}
}

func surfaceSummary(surface *registry.PageSurface) *MemorySurfaceSummary {
	if surface == nil {
		return nil
	}
	return &MemorySurfaceSummary{
		ID:           surface.ID,
		Type:         string(surface.SurfaceType),
		Name:         surface.Title,
		ParentPageID: surface.ParentPageStateID,
	}
}

func currentSurfaceID(surface *registry.PageSurface) string {
	if surface == nil {
		return ""
	}
	return surface.ID
}

func moduleFromRequest(request MemoryContextRequest, page *registry.PageState) string {
	if value, ok := request.Metadata["module"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if page != nil {
		if module, ok := pagePayloadString(page, "module"); ok {
			return module
		}
	}
	_, site := normalizeObservationURL(request.CurrentURL)
	path := strings.TrimPrefix(strings.TrimPrefix(request.CurrentURL, "https://"+site), "http://"+site)
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	first, _, _ := strings.Cut(path, "/")
	first, _, _ = strings.Cut(first, "?")
	first, _, _ = strings.Cut(first, "#")
	return strings.TrimSpace(first)
}

func pagePayloadString(page *registry.PageState, key string) (string, bool) {
	_ = page
	_ = key
	return "", false
}

func appendFilteredEvidence(debug *MemoryContextDebug, source, id, reason string) {
	if debug == nil || reason == "" {
		return
	}
	debug.FilteredEvidence = append(debug.FilteredEvidence, MemoryFilteredEvidence{
		Source: source,
		ID:     id,
		Reason: reason,
	})
}

func fitMemoryContextToBudget(response MemoryContextResponse, debug *MemoryContextDebug, maxChars int) MemoryContextResponse {
	if maxChars <= 0 {
		return response
	}
	for len([]rune(FormatMemoryContextPrompt(response))) > maxChars {
		if removeBudgetOverflowItem(&response, debug) {
			continue
		}
		break
	}
	return response
}

func removeBudgetOverflowItem(response *MemoryContextResponse, debug *MemoryContextDebug) bool {
	if len(response.KnowledgeEvidence) > 0 {
		last := response.KnowledgeEvidence[len(response.KnowledgeEvidence)-1]
		response.KnowledgeEvidence = response.KnowledgeEvidence[:len(response.KnowledgeEvidence)-1]
		appendFilteredEvidence(debug, "knowledge", last.ChunkID, "budget_exceeded")
		return true
	}
	if len(response.FailureWarnings) > 0 {
		last := response.FailureWarnings[len(response.FailureWarnings)-1]
		response.FailureWarnings = response.FailureWarnings[:len(response.FailureWarnings)-1]
		appendFilteredEvidence(debug, "failure", last.ID, "budget_exceeded")
		return true
	}
	if len(response.NavigationHints) > 0 {
		last := response.NavigationHints[len(response.NavigationHints)-1]
		response.NavigationHints = response.NavigationHints[:len(response.NavigationHints)-1]
		appendFilteredEvidence(debug, "navigation", navigationEvidenceID(last.From, last.To, last.Action), "budget_exceeded")
		return true
	}
	if len(response.ExperienceHints) > 0 {
		last := response.ExperienceHints[len(response.ExperienceHints)-1]
		response.ExperienceHints = response.ExperienceHints[:len(response.ExperienceHints)-1]
		appendFilteredEvidence(debug, "experience", last.ID, "budget_exceeded")
		return true
	}
	if response.BusinessContext != nil {
		response.BusinessContext = nil
		appendFilteredEvidence(debug, "business_system", "", "budget_exceeded")
		return true
	}
	return false
}

func experienceSurfaceMismatch(hint ExperienceHint, currentSurfaceID string) bool {
	surfaceIDs := experienceHintSurfaceIDs(hint)
	if len(surfaceIDs) == 0 {
		return false
	}
	currentSurfaceID = strings.TrimSpace(currentSurfaceID)
	if currentSurfaceID == "" {
		return true
	}
	return !containsString(surfaceIDs, currentSurfaceID)
}

func experienceHintSurfaceIDs(hint ExperienceHint) []string {
	ids := make([]string, 0, len(hint.StepTargets))
	for _, target := range hint.StepTargets {
		id := strings.TrimSpace(target.SurfaceID)
		if id != "" && !containsString(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func memoryStepTargets(steps []registry.ExperienceStepSummary) []registry.MemoryStepTarget {
	result := make([]registry.MemoryStepTarget, 0, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.TargetName) == "" {
			continue
		}
		result = append(result, registry.MemoryStepTarget{
			ActionType:    step.ActionName,
			TargetName:    step.TargetName,
			ValueTemplate: step.ValueTemplate,
			PageStateID:   step.PageStateID,
			SurfaceID:     step.SurfaceID,
		})
	}
	return result
}

func stepTargetNames(targets []registry.MemoryStepTarget) []string {
	result := []string{}
	for _, target := range targets {
		if target.TargetName != "" && !containsString(result, target.TargetName) {
			result = append(result, target.TargetName)
		}
	}
	return result
}

func stepActionTypes(targets []registry.MemoryStepTarget) []string {
	result := []string{}
	for _, target := range targets {
		if target.ActionType != "" && !containsString(result, target.ActionType) {
			result = append(result, target.ActionType)
		}
	}
	return result
}

func observationTitle(request MemoryContextRequest) string {
	if request.PageObservation == nil {
		return ""
	}
	return request.PageObservation.Title
}

func observationVisibleText(request MemoryContextRequest) string {
	if request.PageObservation == nil {
		return ""
	}
	return strings.Join(request.PageObservation.VisibleText, " ")
}

func currentPageID(page *registry.PageState) string {
	if page == nil {
		return ""
	}
	return page.ID
}

func experienceHintIDs(hints []ExperienceHint) []string {
	result := make([]string, 0, len(hints))
	for _, hint := range hints {
		result = append(result, hint.ID)
	}
	return result
}

func knowledgeEvidenceIDs(values []KnowledgeEvidence) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.ChunkID)
	}
	return result
}

func buildMemoryEvidenceRefs(response MemoryContextResponse) []registry.MemoryEvidenceRef {
	refs := []registry.MemoryEvidenceRef{}
	rank := 1
	for _, evidence := range response.KnowledgeEvidence {
		refs = append(refs, registry.MemoryEvidenceRef{
			ID:     evidence.ChunkID,
			Source: registry.MemoryEvidenceSourceKnowledge,
			Title:  evidence.Title,
			Rank:   rank,
			Score:  evidence.Score,
			Reason: "Matched imported knowledge for the current task and page.",
		})
		rank++
	}
	rank = 1
	for _, hint := range response.NavigationHints {
		refs = append(refs, registry.MemoryEvidenceRef{
			ID:           navigationEvidenceID(hint.From, hint.To, hint.Action),
			Source:       registry.MemoryEvidenceSourceNavigation,
			Title:        hint.Action,
			Rank:         rank,
			Score:        hint.Confidence,
			PageStateID:  hint.From,
			MatchedRules: []string{"from_page_state"},
			Reason:       "Matched a known page transition from the current page.",
			Payload: map[string]any{
				"optimizedPath": []string{hint.From, hint.To},
				"targetNames":   []string{hint.Action},
			},
		})
		rank++
	}
	rank = 1
	for _, hint := range response.ExperienceHints {
		payload := map[string]any{
			"optimizedPath": hint.OptimizedPath,
			"targetNames":   stepTargetNames(hint.StepTargets),
			"actionTypes":   stepActionTypes(hint.StepTargets),
			"stepTargets":   hint.StepTargets,
		}
		surfaceIDs := experienceHintSurfaceIDs(hint)
		if len(surfaceIDs) == 1 {
			payload["surfaceId"] = surfaceIDs[0]
		}
		if len(surfaceIDs) > 1 {
			payload["surfaceIds"] = surfaceIDs
		}
		refs = append(refs, registry.MemoryEvidenceRef{
			ID:           hint.ID,
			Source:       registry.MemoryEvidenceSourceExperience,
			Title:        hint.Summary,
			Rank:         rank,
			Score:        hint.Confidence,
			PageStateID:  firstString(hint.OptimizedPath),
			MatchedRules: []string{"start_page_state", "task_semantics"},
			Reason:       "Matched a reusable successful experience for this task.",
			Payload:      payload,
		})
		rank++
	}
	rank = 1
	for _, warning := range response.FailureWarnings {
		refs = append(refs, registry.MemoryEvidenceRef{
			ID:     warning.ID,
			Source: registry.MemoryEvidenceSourceFailure,
			Title:  warning.Summary,
			Rank:   rank,
			Score:  0,
			Reason: "Matched a prior failure warning for this page or task.",
			Payload: map[string]any{
				"targetNames": []string{warning.AvoidHint},
			},
		})
		rank++
	}
	return refs
}

func sortKnowledgeEvidence(values []KnowledgeEvidence) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Score == values[j].Score {
			return values[i].ChunkID < values[j].ChunkID
		}
		return values[i].Score > values[j].Score
	})
}

func sortNavigationHints(values []NavigationHint) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Confidence == values[j].Confidence {
			return navigationEvidenceID(values[i].From, values[i].To, values[i].Action) < navigationEvidenceID(values[j].From, values[j].To, values[j].Action)
		}
		return values[i].Confidence > values[j].Confidence
	})
}

func variableNames(values []registry.Variable) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Name)
	}
	return result
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func limitKnowledge(values []registry.KnowledgeChunk, limit int) []registry.KnowledgeChunk {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func limitTransitions(values []registry.PageTransition, limit int) []registry.PageTransition {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func limitFailures(values []registry.FailureMemory, limit int) []registry.FailureMemory {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func limitKnowledgeEvidence(values []KnowledgeEvidence, limit int) []KnowledgeEvidence {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func limitNavigationHints(values []NavigationHint, limit int) []NavigationHint {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func limitExperienceHints(values []ExperienceHint, limit int) []ExperienceHint {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func limitFailureWarnings(values []FailureWarning, limit int) []FailureWarning {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}
