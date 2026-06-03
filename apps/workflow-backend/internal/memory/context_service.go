package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"

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
	observationSignal := buildPageObservationSignal(projectID, site, request)

	response := MemoryContextResponse{
		ContextID:             newMemoryContextID(projectID, request.Task, request.CurrentURL),
		ProjectID:             projectID,
		CurrentPage:           pageSummary(currentPage),
		CurrentSurface:        surfaceSummary(currentSurface),
		PageObservationSignal: observationSignal,
		RecommendedMode:       registry.MemoryModeNormal,
	}
	debug := &MemoryContextDebug{PromptBudget: MemoryPromptBudgetDebug{MaxChars: defaultMemoryPromptBudgetChars}}
	if request.CurrentURL != "" {
		appendFilteredEvidence(debug, "page", "", "hard_gate_applied")
	}
	response.SiteTaskGuides = service.siteTaskGuides(ctx, registry.SiteTaskGuideSearchQuery{
		ProjectID: projectID,
		Site:      site,
		Module:    module,
		Task:      request.Task,
		Limit:     memoryCandidateLimit,
	}, request, debug)
	response.SiteManualKnowledge = service.siteManualKnowledge(ctx, registry.SiteManualWikiSearchQuery{
		ProjectID:         projectID,
		Site:              site,
		Module:            module,
		Task:              request.Task,
		URL:               request.CurrentURL,
		Title:             observationTitle(request),
		VisibleTextSample: observationVisibleText(request),
		ActiveOverlayHint: observationActiveOverlayHint(request),
		Limit:             3,
		IncludeFiltered:   true,
	}, debug)
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
	}
	response = fitMemoryContextToBudget(response, debug, defaultMemoryPromptBudgetChars)
	response.RecommendedMode = recommendedMode(response)
	response.EvidenceRefs = buildMemoryEvidenceRefs(response)
	response.ContextPrompt = FormatMemoryContextPrompt(response)
	debug.PromptChars = len([]rune(response.ContextPrompt))
	debug.PromptBudget.UsedChars = debug.PromptChars
	response.Debug = debug
	if err := service.repo.SaveMemoryContextEvent(ctx, registry.MemoryContextEvent{
		ID:               response.ContextID,
		ProjectID:        projectID,
		Task:             request.Task,
		CurrentURL:       request.CurrentURL,
		CurrentPageState: currentPageID(currentPage),
		CurrentSurface:   currentSurfaceID(currentSurface),
		ContextPrompt:    response.ContextPrompt,
		EvidenceRefs:     response.EvidenceRefs,
		RecommendedMode:  response.RecommendedMode,
		Payload: map[string]any{
			"mode":                  request.Mode,
			"debug":                 response.Debug,
			"currentSurface":        currentSurfaceIDValue,
			"pageObservationSignal": response.PageObservationSignal,
			"siteTaskGuides":        guideHintIDs(response.SiteTaskGuides),
			"siteManualKnowledge":   manualHintIDs(response.SiteManualKnowledge),
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
	_ = ctx
	_ = projectID
	_ = site
	_ = currentPage
	_ = request
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

func (service *MemoryContextService) siteTaskGuides(ctx context.Context, query registry.SiteTaskGuideSearchQuery, request MemoryContextRequest, debug *MemoryContextDebug) []SiteTaskGuideHint {
	guideRepo, ok := service.repo.(registry.SiteTaskGuideRepository)
	if !ok || query.Site == "" {
		return nil
	}
	guides, err := guideRepo.SearchSiteTaskGuides(ctx, query)
	if err != nil {
		return nil
	}
	result := []SiteTaskGuideHint{}
	for _, guide := range guides {
		taskGate := evaluateTaskIntentGate(request.Task, guide)
		if !taskGate.Passed {
			appendCandidateSiteTaskGuide(debug, guide.ID, taskGate.Score, false, taskGate.Reason, "")
			appendFilteredEvidence(debug, "guide", guide.ID, taskGate.Reason)
			continue
		}
		if len(guide.UIStateEntries) == 0 {
			appendCandidateSiteTaskGuide(debug, guide.ID, taskGate.Score, false, "missing_ui_state_entries", "")
			appendFilteredEvidence(debug, "guide", guide.ID, "missing_ui_state_entries")
			continue
		}
		if reason := siteTaskGuideMismatchReason(guide, request); reason != "" {
			appendCandidateSiteTaskGuide(debug, guide.ID, taskGate.Score, false, reason, "")
			appendFilteredEvidence(debug, "guide", guide.ID, reason)
			continue
		}
		observationSignal := buildPageObservationSignal(query.ProjectID, query.Site, request)
		stateMatch := bestUIStateMatch(guide, observationSignal)
		appendUIStateMatch(debug, guide.ID, stateMatch)
		if !stateMatch.Passed {
			appendCandidateSiteTaskGuide(debug, guide.ID, taskGate.Score, false, stateMatch.Reason, stateMatch.Entry.ID)
			appendFilteredEvidence(debug, "guide", guide.ID, stateMatch.Reason)
			continue
		}
		if !guideStepAnchorMatches(guide, observationSignal, stateMatch) {
			appendCandidateSiteTaskGuide(debug, guide.ID, taskGate.Score, false, "step_anchor_mismatch", stateMatch.Entry.ID)
			appendFilteredEvidence(debug, "guide", guide.ID, "step_anchor_mismatch")
			continue
		}
		steps := guideStepTextsFromOffset(guide.Steps, stateMatch.Entry.StepOffset)
		if len(steps) == 0 {
			appendCandidateSiteTaskGuide(debug, guide.ID, taskGate.Score, false, "no_remaining_steps", stateMatch.Entry.ID)
			appendFilteredEvidence(debug, "guide", guide.ID, "no_remaining_steps")
			continue
		}
		confidence := guide.Confidence + minFloat(taskGate.Score*0.04, 0.2) + minFloat(stateMatch.Score*0.02, 0.2)
		result = append(result, SiteTaskGuideHint{
			ID:               guide.ID,
			WhenToUse:        guide.TaskIntentSummary,
			MatchedStateID:   stateMatch.Entry.ID,
			MatchedStateName: stateMatch.Entry.Name,
			StartStepOffset:  stateMatch.Entry.StepOffset,
			MatchReasons:     stateMatch.Matched,
			PageGuards:       guideGuardSummaries(guide),
			Steps:            steps,
			AbandonRules:     guide.AbandonRules,
			Confidence:       confidence,
		})
		appendCandidateSiteTaskGuide(debug, guide.ID, taskGate.Score, true, "matched", stateMatch.Entry.ID)
		if len(result) >= 3 {
			break
		}
	}
	return result
}

func (service *MemoryContextService) siteManualKnowledge(ctx context.Context, query registry.SiteManualWikiSearchQuery, debug *MemoryContextDebug) []SiteManualKnowledgeHint {
	manualRepo, ok := service.repo.(registry.SiteManualRepository)
	if !ok || query.Site == "" {
		return nil
	}
	matches, err := manualRepo.SearchSiteManualWikiChunks(ctx, query)
	if err != nil {
		return nil
	}
	result := []SiteManualKnowledgeHint{}
	for _, match := range matches {
		if match.Score < 0 {
			appendFilteredEvidence(debug, "manual", match.Chunk.ID, match.Reason)
			continue
		}
		result = append(result, SiteManualKnowledgeHint{
			ID:         match.Chunk.ID,
			Title:      string(match.Chunk.ChunkType),
			Summary:    match.Chunk.Text,
			SourceRefs: sourceRefStrings(match.Chunk.SourceRefs),
			Confidence: match.Score,
		})
		if len(result) >= 3 {
			break
		}
	}
	return result
}

func recommendedMode(response MemoryContextResponse) registry.MemoryMode {
	if len(response.NavigationHints) > 0 || len(response.SiteTaskGuides) > 0 || len(response.SiteManualKnowledge) > 0 || len(response.FailureWarnings) > 0 {
		return registry.MemoryModeGuided
	}
	return registry.MemoryModeNormal
}

func newMemoryContextID(projectID, task, currentURL string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "ctx_" + hex.EncodeToString(bytes[:])
	}
	seed := hex.EncodeToString([]byte(strings.Join([]string{projectID, task, currentURL, time.Now().UTC().Format(time.RFC3339Nano)}, "\x00")))
	if len(seed) > 16 {
		seed = seed[:16]
	}
	return "ctx_" + seed
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

func buildPageObservationSignal(projectID, site string, request MemoryContextRequest) *registry.PageObservationSignal {
	if request.PageObservation == nil && request.CurrentURL == "" {
		return nil
	}
	normalizedURL, normalizedSite := normalizeObservationURL(request.CurrentURL)
	if site == "" {
		site = normalizedSite
	}
	signal := &registry.PageObservationSignal{
		ProjectID:  projectID,
		Site:       site,
		URL:        request.CurrentURL,
		URLPattern: normalizedURL,
		URLFamily:  urlPathFamily(normalizedURL),
		ObservedAt: time.Now().UTC(),
	}
	if request.PageObservation != nil {
		signal.Title = request.PageObservation.Title
		signal.VisibleTextSample = TruncateSummary(strings.Join(request.PageObservation.VisibleText, " "))
		signal.Breadcrumbs = trimStringList(request.PageObservation.Breadcrumbs)
		signal.ActiveTabs = trimStringList(request.PageObservation.ActiveTabs)
		signal.Tables = request.PageObservation.Tables
		signal.ActiveSurfaces = request.PageObservation.ActiveSurfaces
		for _, control := range request.PageObservation.Controls {
			signal.ControlSignatures = append(signal.ControlSignatures, registry.ControlSignature{
				Role:     control.Role,
				Name:     control.Name,
				Selected: control.Selected,
				Enabled:  control.Enabled,
			})
		}
		signal.ActiveOverlayHint = observationActiveOverlayHint(request)
	}
	return signal
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

func appendCandidateSiteTaskGuide(debug *MemoryContextDebug, id string, score float64, passed bool, reason string, stateID string) {
	if debug == nil {
		return
	}
	debug.CandidateSiteTaskGuides = append(debug.CandidateSiteTaskGuides, MemoryCandidateSiteTaskGuide{
		ID:           id,
		TaskScore:    score,
		Passed:       passed,
		Reason:       reason,
		MatchedState: stateID,
	})
}

func appendUIStateMatch(debug *MemoryContextDebug, guideID string, match uiStateMatchResult) {
	if debug == nil || match.Entry.ID == "" {
		return
	}
	debug.UIStateMatches = append(debug.UIStateMatches, MemoryUIStateMatchDebug{
		GuideID:      guideID,
		StateID:      match.Entry.ID,
		Score:        match.Score,
		MinimumScore: match.MinimumScore,
		Passed:       match.Passed,
		Matched:      match.Matched,
		Missing:      match.Missing,
		Reason:       match.Reason,
	})
}

func guideStepAnchorMatches(guide registry.SiteTaskGuide, signal *registry.PageObservationSignal, stateMatch uiStateMatchResult) bool {
	if signal == nil {
		return false
	}
	if hasStrongUIStateMatch(stateMatch.Matched) {
		return true
	}
	anchors := guideStepAnchorTerms(guide, stateMatch.Entry.ID, weakStateAnchorExclusions(stateMatch.Entry))
	if len(anchors) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		signal.Title,
		signal.VisibleTextSample,
		strings.Join(signal.Breadcrumbs, " "),
		strings.Join(signal.ActiveTabs, " "),
		controlTextForGuide(allSignalControls(signal)),
		activeSurfaceText(signal.ActiveSurfaces),
	}, "\n"))
	return anyTextPresent(haystack, anchors)
}

func hasStrongUIStateMatch(matched []string) bool {
	for _, item := range matched {
		switch item {
		case "controls_all", "controls_any", "active_tab", "table_headers", "active_surface":
			return true
		}
	}
	return false
}

func guideStepAnchorTerms(guide registry.SiteTaskGuide, matchedStateID string, excluded map[string]bool) []string {
	anchors := []string{}
	anchors = appendGuideGuardAnchors(anchors, guide.StartPageGuard)
	for _, guard := range guide.PageGuards {
		anchors = appendGuideGuardAnchors(anchors, guard)
	}
	for _, state := range guide.UIStateEntries {
		if state.ID == matchedStateID {
			anchors = appendGuideStrongUIStateEvidenceAnchors(anchors, state.Evidence)
			continue
		}
		anchors = appendGuideAnchor(anchors, state.Name)
		anchors = appendGuideUIStateEvidenceAnchors(anchors, state.Evidence)
	}
	for _, step := range guide.Steps {
		anchors = appendGuideStepAnchors(anchors, step)
	}
	return uniqueLimited(filterExcludedGuideAnchors(anchors, excluded), 24)
}

func weakStateAnchorExclusions(entry registry.SiteTaskGuideUIStateEntry) map[string]bool {
	excluded := map[string]bool{}
	addExcludedGuideAnchor(excluded, entry.Name)
	for _, group := range [][]string{
		entry.Evidence.TitleAny,
		entry.Evidence.BreadcrumbAny,
		entry.Evidence.TextAll,
		entry.Evidence.TextAny,
	} {
		for _, value := range group {
			addExcludedGuideAnchor(excluded, value)
		}
	}
	return excluded
}

func addExcludedGuideAnchor(excluded map[string]bool, value string) {
	anchor := stableGuideStepAnchor(value)
	if anchor == "" {
		return
	}
	excluded[strings.ToLower(anchor)] = true
}

func filterExcludedGuideAnchors(anchors []string, excluded map[string]bool) []string {
	if len(excluded) == 0 {
		return anchors
	}
	result := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		if !excluded[strings.ToLower(anchor)] {
			result = append(result, anchor)
		}
	}
	return result
}

func appendGuideGuardAnchors(anchors []string, guard registry.SiteTaskGuideGuard) []string {
	for _, value := range guard.RequiredText {
		anchors = appendGuideAnchor(anchors, value)
	}
	return appendGuideControlAnchors(anchors, guard.RequiredControls)
}

func appendGuideUIStateEvidenceAnchors(anchors []string, evidence registry.SiteTaskGuideUIStateEvidence) []string {
	for _, group := range [][]string{
		evidence.TitleAny,
		evidence.BreadcrumbAny,
		evidence.ActiveTabAny,
		evidence.TextAll,
		evidence.TextAny,
	} {
		for _, value := range group {
			anchors = appendGuideAnchor(anchors, value)
		}
	}
	anchors = appendGuideControlAnchors(anchors, evidence.ControlsAll)
	anchors = appendGuideControlAnchors(anchors, evidence.ControlsAny)
	for _, headers := range evidence.TableHeadersAny {
		for _, header := range headers {
			anchors = appendGuideAnchor(anchors, header)
		}
	}
	for _, surface := range evidence.ActiveSurfacesAny {
		anchors = appendGuideActiveSurfaceAnchors(anchors, surface)
	}
	for _, table := range evidence.TablesAny {
		anchors = appendGuideAnchor(anchors, table.Caption)
		for _, header := range table.Headers {
			anchors = appendGuideAnchor(anchors, header)
		}
	}
	return anchors
}

func appendGuideStrongUIStateEvidenceAnchors(anchors []string, evidence registry.SiteTaskGuideUIStateEvidence) []string {
	for _, value := range evidence.ActiveTabAny {
		anchors = appendGuideAnchor(anchors, value)
	}
	anchors = appendGuideControlAnchors(anchors, evidence.ControlsAll)
	anchors = appendGuideControlAnchors(anchors, evidence.ControlsAny)
	for _, headers := range evidence.TableHeadersAny {
		for _, header := range headers {
			anchors = appendGuideAnchor(anchors, header)
		}
	}
	for _, surface := range evidence.ActiveSurfacesAny {
		anchors = appendGuideActiveSurfaceAnchors(anchors, surface)
	}
	for _, table := range evidence.TablesAny {
		anchors = appendGuideAnchor(anchors, table.Caption)
		for _, header := range table.Headers {
			anchors = appendGuideAnchor(anchors, header)
		}
	}
	return anchors
}

func appendGuideStepAnchors(anchors []string, step registry.SiteTaskGuideStep) []string {
	for _, value := range []string{
		step.Target,
		step.PageTitle,
		step.SemanticTarget.Text,
		step.SemanticTarget.ContainerHint,
		stableTargetFromStepText(step.Text),
	} {
		anchors = appendGuideAnchor(anchors, value)
	}
	for _, alias := range step.SemanticTarget.Aliases {
		anchors = appendGuideAnchor(anchors, alias)
	}
	for _, value := range step.ExpectedOutcome.TitleAny {
		anchors = appendGuideAnchor(anchors, value)
	}
	for _, value := range step.ExpectedOutcome.TextAny {
		anchors = appendGuideAnchor(anchors, value)
	}
	for _, value := range step.ExpectedOutcome.ActiveTabAny {
		anchors = appendGuideAnchor(anchors, value)
	}
	anchors = appendGuideControlAnchors(anchors, step.ExpectedOutcome.ControlsAll)
	anchors = appendGuideControlAnchors(anchors, step.ExpectedOutcome.ControlsAny)
	return anchors
}

func appendGuideActiveSurfaceAnchors(anchors []string, surface registry.ActiveSurfaceSignal) []string {
	anchors = appendGuideAnchor(anchors, surface.Title)
	for _, value := range surface.Text {
		anchors = appendGuideAnchor(anchors, value)
	}
	return appendGuideControlAnchors(anchors, surface.Controls)
}

func appendGuideControlAnchors(anchors []string, controls []registry.ControlSignature) []string {
	for _, control := range controls {
		anchors = appendGuideAnchor(anchors, control.Name)
	}
	return anchors
}

func appendGuideAnchor(anchors []string, value string) []string {
	if anchor := stableGuideStepAnchor(value); anchor != "" {
		return append(anchors, anchor)
	}
	return anchors
}

func stableGuideStepAnchor(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n:：,，.。/\\[]()（）\"'“”")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" || strings.Contains(value, "{{") || strings.Contains(value, "}}") {
		return ""
	}
	runeCount := len([]rune(value))
	if runeCount < 2 || runeCount > 48 {
		return ""
	}
	if !hasLetterOrDigit(value) {
		return ""
	}
	if looksLikeDynamicTarget(value) || ContainsSensitiveMaterial(value) || containsInstanceValue(value) {
		return ""
	}
	return safeGuideText(value)
}

func hasLetterOrDigit(value string) bool {
	for _, item := range value {
		if unicode.IsLetter(item) || unicode.IsDigit(item) {
			return true
		}
	}
	return false
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
	if len(response.SiteManualKnowledge) > 0 {
		last := response.SiteManualKnowledge[len(response.SiteManualKnowledge)-1]
		response.SiteManualKnowledge = response.SiteManualKnowledge[:len(response.SiteManualKnowledge)-1]
		appendFilteredEvidence(debug, "manual", last.ID, "budget_exceeded")
		return true
	}
	if len(response.SiteTaskGuides) > 0 {
		last := response.SiteTaskGuides[len(response.SiteTaskGuides)-1]
		response.SiteTaskGuides = response.SiteTaskGuides[:len(response.SiteTaskGuides)-1]
		appendFilteredEvidence(debug, "guide", last.ID, "budget_exceeded")
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

func observationActiveOverlayHint(request MemoryContextRequest) string {
	if request.Metadata != nil {
		value, _ := request.Metadata["activeOverlayHint"].(string)
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if request.PageObservation == nil {
		return ""
	}
	parts := []string{}
	for _, surface := range request.PageObservation.ActiveSurfaces {
		parts = append(parts, surface.Title)
		parts = append(parts, surface.Text...)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func siteTaskGuideMismatchReason(guide registry.SiteTaskGuide, request MemoryContextRequest) string {
	guards := append([]registry.SiteTaskGuideGuard{}, guide.StartPageGuard)
	guards = append(guards, guide.PageGuards...)
	nonEmpty := []registry.SiteTaskGuideGuard{}
	for _, guard := range guards {
		if !emptyGuideGuard(guard) {
			nonEmpty = append(nonEmpty, guard)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	for _, guard := range nonEmpty {
		if siteTaskGuideGuardMatches(guard, request) {
			return ""
		}
	}
	return "page_guard_mismatch"
}

func emptyGuideGuard(guard registry.SiteTaskGuideGuard) bool {
	return guard.URLPattern == "" &&
		len(guard.URLIncludes) == 0 &&
		len(guard.RequiredText) == 0 &&
		len(guard.RequiredControls) == 0 &&
		guard.ActiveOverlayHint == ""
}

func siteTaskGuideGuardMatches(guard registry.SiteTaskGuideGuard, request MemoryContextRequest) bool {
	searchURL := strings.ToLower(request.CurrentURL)
	normalizedURL, _ := normalizeObservationURL(request.CurrentURL)
	normalizedURL = strings.ToLower(normalizedURL)
	for _, include := range guard.URLIncludes {
		include = strings.ToLower(strings.TrimSpace(include))
		if include != "" && !strings.Contains(searchURL, include) && !strings.Contains(normalizedURL, include) {
			return false
		}
	}
	if guard.URLPattern != "" {
		pattern := strings.ToLower(strings.TrimRight(guard.URLPattern, "*"))
		if !strings.Contains(searchURL, pattern) && !strings.Contains(normalizedURL, pattern) {
			return false
		}
	}
	pageText := strings.ToLower(strings.Join([]string{
		observationTitle(request),
		observationVisibleText(request),
		observationActiveOverlayHint(request),
	}, "\n"))
	for _, text := range guard.RequiredText {
		text = strings.ToLower(strings.TrimSpace(text))
		if text != "" && !strings.Contains(pageText, text) {
			return false
		}
	}
	if len(guard.RequiredControls) > 0 {
		observed := []registry.ControlSignature{}
		if request.PageObservation != nil {
			for _, control := range request.PageObservation.Controls {
				observed = append(observed, registry.ControlSignature{Role: control.Role, Name: control.Name})
			}
		}
		if !anyControlPresent(observed, guard.RequiredControls) {
			return false
		}
	}
	if guard.ActiveOverlayHint != "" && !strings.Contains(pageText, strings.ToLower(guard.ActiveOverlayHint)) {
		return false
	}
	return true
}

func guideGuardSummaries(guide registry.SiteTaskGuide) []string {
	result := []string{}
	for _, guard := range append([]registry.SiteTaskGuideGuard{guide.StartPageGuard}, guide.PageGuards...) {
		if guard.URLPattern != "" {
			result = append(result, "URL matches "+guard.URLPattern)
		}
		if len(guard.RequiredText) > 0 {
			result = append(result, "Text includes "+strings.Join(guard.RequiredText, ", "))
		}
		for _, control := range guard.RequiredControls {
			if control.Name != "" {
				result = append(result, "Control exists "+strings.TrimSpace(control.Role+" "+control.Name))
			}
		}
	}
	return uniqueLimited(result, 6)
}

func guideStepTexts(steps []registry.SiteTaskGuideStep) []string {
	result := make([]string, 0, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.Text) != "" {
			result = append(result, step.Text)
		}
	}
	return result
}

func guideStepTextsFromOffset(steps []registry.SiteTaskGuideStep, offset int) []string {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(steps) {
		return nil
	}
	return guideStepTexts(steps[offset:])
}

func sourceRefStrings(refs []registry.MemorySourceRef) []string {
	result := []string{}
	for _, ref := range refs {
		if ref.ID == "" {
			continue
		}
		result = append(result, strings.TrimSpace(ref.Type+":"+ref.ID))
	}
	return result
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

func guideHintIDs(hints []SiteTaskGuideHint) []string {
	result := make([]string, 0, len(hints))
	for _, hint := range hints {
		result = append(result, hint.ID)
	}
	return result
}

func manualHintIDs(hints []SiteManualKnowledgeHint) []string {
	result := make([]string, 0, len(hints))
	for _, hint := range hints {
		result = append(result, hint.ID)
	}
	return result
}

func buildMemoryEvidenceRefs(response MemoryContextResponse) []registry.MemoryEvidenceRef {
	refs := []registry.MemoryEvidenceRef{}
	rank := 1
	for _, guide := range response.SiteTaskGuides {
		refs = append(refs, registry.MemoryEvidenceRef{
			ID:     guide.ID,
			Source: registry.MemoryEvidenceSourceGuide,
			Title:  guide.WhenToUse,
			Rank:   rank,
			Score:  guide.Confidence,
			Reason: "Matched a reusable site task guide for the current task and page.",
			Payload: map[string]any{
				"steps":            guide.Steps,
				"abandonRules":     guide.AbandonRules,
				"matchedStateId":   guide.MatchedStateID,
				"matchedStateName": guide.MatchedStateName,
				"startStepOffset":  guide.StartStepOffset,
				"targetNames":      guide.Steps,
				"matchReasons":     guide.MatchReasons,
			},
		})
		rank++
	}
	for _, item := range response.SiteManualKnowledge {
		refs = append(refs, registry.MemoryEvidenceRef{
			ID:     item.ID,
			Source: registry.MemoryEvidenceSourceManual,
			Title:  item.Title,
			Rank:   rank,
			Score:  item.Confidence,
			Reason: "Matched user-imported site manual wiki knowledge for this task and page.",
			Payload: map[string]any{
				"summary":    item.Summary,
				"sourceRefs": item.SourceRefs,
			},
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
