package memory

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/page-agent/workflow-backend/internal/registry"
)

type SiteTaskGuideService struct {
	repo      registry.Repository
	guideRepo registry.SiteTaskGuideRepository
}

func NewSiteTaskGuideService(repo registry.Repository) *SiteTaskGuideService {
	guideRepo, _ := repo.(registry.SiteTaskGuideRepository)
	return &SiteTaskGuideService{repo: repo, guideRepo: guideRepo}
}

func (service *SiteTaskGuideService) UpsertGuideFromTaskRun(ctx context.Context, run registry.TaskRun) (registry.SiteTaskGuide, bool, error) {
	if service.guideRepo == nil {
		return registry.SiteTaskGuide{}, false, errors.New("site task guide repository is not configured")
	}
	if run.Status != registry.TaskRunSuccess {
		return registry.SiteTaskGuide{}, false, nil
	}
	rawTaskIntent := firstNonEmpty(run.TaskTemplate, run.Summary)
	taskIntentSummary := safeGuideText(rawTaskIntent)
	if !usefulGuideIntent(taskIntentSummary) {
		return registry.SiteTaskGuide{}, false, nil
	}
	steps := buildSiteTaskGuideSteps(run.ActionSteps)
	if summarySteps := buildSiteTaskGuideStepsFromTaskSummary(rawTaskIntent); len(summarySteps) >= 3 {
		steps = summarySteps
	}
	if len(steps) == 0 {
		return registry.SiteTaskGuide{}, false, nil
	}
	startGuard, pageGuards := service.guardsForRun(ctx, run)
	uiStateEntries := service.buildUIStateEntriesFromActionSteps(ctx, run.ActionSteps)
	if len(uiStateEntries) == 0 {
		return registry.SiteTaskGuide{}, false, nil
	}
	if uiStartGuard, uiPageGuards := guardsFromUIStateEntries(uiStateEntries); guideGuardsQualityScore(uiStartGuard, uiPageGuards) > guideGuardsQualityScore(startGuard, pageGuards) {
		startGuard = uiStartGuard
		pageGuards = uiPageGuards
	}
	module := moduleFromGuideRun(ctx, service.repo, run)
	intentKey := stableGuideIntentKey(run.Site, module, taskIntentSummary)
	now := time.Now().UTC()
	guide := registry.SiteTaskGuide{
		ID:                "guide_" + intentKey,
		ProjectID:         defaultGuideProjectID(run.ProjectID),
		Site:              strings.TrimSpace(run.Site),
		Module:            module,
		TaskIntentKey:     intentKey,
		TaskIntentSummary: taskIntentSummary,
		TaskIntentTerms:   deriveTaskIntentTerms(taskIntentSummary, steps),
		TaskExamples:      uniqueLimited([]string{taskIntentSummary}, 5),
		RouteScope:        routeScopeFromUIStates(uiStateEntries),
		StartURLPattern:   startGuard.URLPattern,
		StartPageGuard:    startGuard,
		PageGuards:        pageGuards,
		UIStateEntries:    uiStateEntries,
		Steps:             steps,
		AbandonRules: []string{
			"Abandon this guide if the current page does not match the URL, required text, or required controls.",
			"Abandon this guide if the stable target controls in the steps are missing.",
		},
		VariableRules: []string{"Use task variables from the live request; do not reuse previous instance names, IPs, IDs, or backup IDs."},
		Evidence:      []registry.MemorySourceRef{{Type: "task_run", ID: run.ID}},
		Summary:       taskIntentSummary,
		Confidence:    0.65,
		SuccessCount:  1,
		Status:        registry.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if guide.Site == "" {
		return registry.SiteTaskGuide{}, false, nil
	}
	existing, found := service.findExistingGuide(ctx, guide)
	if found {
		guide.ID = existing.ID
		guide.CreatedAt = existing.CreatedAt
		guide.SuccessCount = existing.SuccessCount + 1
		guide.AbandonedCount = existing.AbandonedCount
		guide.MisleadingCount = existing.MisleadingCount
		guide.UnusedCount = existing.UnusedCount
		guide.StaleCount = existing.StaleCount
		guide.TaskExamples = uniqueLimited(append(existing.TaskExamples, taskIntentSummary), 5)
		guide.Evidence = uniqueSourceRefs(append(existing.Evidence, guide.Evidence...))
		if shouldKeepExistingGuideSteps(existing.Steps, guide.Steps) {
			guide.Steps = existing.Steps
			if guideGuardsQualityScore(guide.StartPageGuard, guide.PageGuards) <= guideGuardsQualityScore(existing.StartPageGuard, existing.PageGuards) {
				guide.PageGuards = existing.PageGuards
				guide.StartPageGuard = existing.StartPageGuard
				guide.StartURLPattern = existing.StartURLPattern
			}
		}
		guide.UIStateEntries = mergeUIStateEntries(existing.UIStateEntries, guide.UIStateEntries)
		if len(existing.TaskIntentTerms.Positive) > 0 || len(existing.TaskIntentTerms.Negative) > 0 {
			guide.TaskIntentTerms = mergeIntentTerms(existing.TaskIntentTerms, guide.TaskIntentTerms)
		}
	}
	guide.Confidence = guideConfidence(guide)
	if err := service.guideRepo.SaveSiteTaskGuide(ctx, guide); err != nil {
		return registry.SiteTaskGuide{}, false, err
	}
	return guide, true, nil
}

func shouldKeepExistingGuideSteps(existing []registry.SiteTaskGuideStep, generated []registry.SiteTaskGuideStep) bool {
	if len(existing) == 0 || len(existing) > len(generated) {
		return false
	}
	for _, step := range existing {
		if dirtyGuideStep(step) {
			return false
		}
	}
	if guideStepsQualityScore(generated) > guideStepsQualityScore(existing)+2 {
		return false
	}
	return true
}

func guideStepsQualityScore(steps []registry.SiteTaskGuideStep) int {
	score := len(steps) * 2
	seen := map[string]bool{}
	for _, step := range steps {
		text := strings.TrimSpace(firstNonEmpty(step.Text, step.Goal, step.Target))
		target := strings.TrimSpace(step.Target)
		if text == "" && target == "" {
			score -= 4
			continue
		}
		if seen[text] {
			score -= 3
		}
		seen[text] = true
		if containsAnyLower(strings.ToLower(text+" "+target), []string{"next", "submit"}) {
			score -= 3
		}
		if step.ActionType != "" {
			score += 1
		}
		if target != "" {
			score += 1
		}
		if len(step.ExpectedOutcome.TextAny) > 0 ||
			len(step.ExpectedOutcome.TitleAny) > 0 ||
			len(step.ExpectedOutcome.ControlsAll) > 0 ||
			len(step.ExpectedOutcome.ControlsAny) > 0 {
			score += 1
		}
		if len([]rune(text)) >= 10 {
			score += 1
		}
	}
	return score
}

func dirtyGuideStep(step registry.SiteTaskGuideStep) bool {
	values := []string{step.Target, step.Text, step.Goal, step.SemanticTarget.Text}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		lower := strings.ToLower(value)
		if looksLikeDynamicTarget(value) ||
			strings.Contains(lower, "element_") ||
			strings.Contains(lower, "<button") ||
			strings.Contains(lower, "<div") ||
			strings.Contains(lower, "<input") ||
			containsInstanceValue(value) ||
			looksLikeBareInstanceIdentifier(value) {
			return true
		}
	}
	return false
}

func looksLikeBareInstanceIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	return regexp.MustCompile(`^[a-z][a-z0-9_-]{2,}$`).MatchString(value)
}

func (service *SiteTaskGuideService) ApplyFeedback(ctx context.Context, guideID string, feedback registry.SiteTaskGuideFeedback) error {
	if service.guideRepo == nil {
		return errors.New("site task guide repository is not configured")
	}
	guide, err := service.guideRepo.GetSiteTaskGuide(ctx, guideID)
	if err != nil {
		return err
	}
	feedback.GuideID = guideID
	if feedback.CreatedAt.IsZero() {
		feedback.CreatedAt = time.Now().UTC()
	}
	if err := service.guideRepo.SaveSiteTaskGuideFeedback(ctx, feedback); err != nil {
		return err
	}
	switch feedback.Label {
	case registry.SiteTaskGuideFeedbackUsedHelpful:
		guide.SuccessCount++
	case registry.SiteTaskGuideFeedbackAbandonedMismatch:
		guide.AbandonedCount++
	case registry.SiteTaskGuideFeedbackUsedMisleading:
		guide.MisleadingCount++
	case registry.SiteTaskGuideFeedbackUnused:
		guide.UnusedCount++
	case registry.SiteTaskGuideFeedbackStale:
		guide.StaleCount++
	}
	if feedback.StateID != "" {
		guide.UIStateEntries = updateUIStateFeedback(guide.UIStateEntries, feedback)
	}
	if guide.StaleCount >= 3 {
		guide.Status = registry.StatusStale
	} else if guide.MisleadingCount >= 3 {
		guide.Status = registry.StatusHidden
	}
	guide.Confidence = guideConfidence(guide)
	guide.UpdatedAt = time.Now().UTC()
	return service.guideRepo.SaveSiteTaskGuide(ctx, guide)
}

func (service *SiteTaskGuideService) findExistingGuide(ctx context.Context, guide registry.SiteTaskGuide) (registry.SiteTaskGuide, bool) {
	guides, err := service.guideRepo.ListSiteTaskGuides(ctx, registry.SiteTaskGuideListQuery{
		ProjectID: guide.ProjectID,
		Site:      guide.Site,
		Module:    guide.Module,
	})
	if err != nil {
		return registry.SiteTaskGuide{}, false
	}
	for _, existing := range guides {
		if existing.TaskIntentKey == guide.TaskIntentKey && existing.Status != registry.StatusDeleted {
			return existing, true
		}
	}
	return registry.SiteTaskGuide{}, false
}

func (service *SiteTaskGuideService) guardsForRun(ctx context.Context, run registry.TaskRun) (registry.SiteTaskGuideGuard, []registry.SiteTaskGuideGuard) {
	path := run.OptimizedPath
	if len(path) == 0 {
		path = run.OriginalPath
	}
	guards := []registry.SiteTaskGuideGuard{}
	for _, pageID := range path {
		page, err := service.repo.GetPageState(ctx, pageID)
		if err != nil {
			continue
		}
		guard := registry.SiteTaskGuideGuard{
			URLPattern:       page.URLPattern,
			RequiredText:     firstNStrings(page.RequiredText, 3),
			RequiredControls: firstNControls(page.RequiredControls, 3),
		}
		if len(guard.RequiredControls) == 0 {
			guard.RequiredControls = firstNControls(page.HardRules.ControlsAll, 3)
		}
		if len(guard.RequiredText) == 0 {
			guard.RequiredText = firstNStrings(page.HardRules.TextAll, 3)
		}
		guards = append(guards, guard)
	}
	if len(guards) == 0 {
		return registry.SiteTaskGuideGuard{}, nil
	}
	return guards[0], guards
}

func guardsFromUIStateEntries(states []registry.SiteTaskGuideUIStateEntry) (registry.SiteTaskGuideGuard, []registry.SiteTaskGuideGuard) {
	guards := []registry.SiteTaskGuideGuard{}
	for _, state := range states {
		guard := registry.SiteTaskGuideGuard{
			URLPattern:  state.RouteScope.URLPattern,
			URLIncludes: firstNStrings(state.RouteScope.URLIncludes, 2),
		}
		evidence := state.Evidence
		guard.RequiredText = firstNStrings(append(append([]string{}, evidence.ActiveTabAny...), append(evidence.TextAll, evidence.TextAny...)...), 3)
		guard.RequiredControls = firstNControls(append(append([]registry.ControlSignature{}, evidence.ControlsAll...), evidence.ControlsAny...), 3)
		for _, surface := range evidence.ActiveSurfacesAny {
			if guard.ActiveOverlayHint == "" {
				guard.ActiveOverlayHint = safeGuideText(surface.Title)
			}
			guard.RequiredText = firstNStrings(append(guard.RequiredText, surface.Text...), 3)
			guard.RequiredControls = firstNControls(append(guard.RequiredControls, surface.Controls...), 3)
		}
		if !emptyGuideGuard(guard) {
			guards = append(guards, guard)
		}
	}
	if len(guards) == 0 {
		return registry.SiteTaskGuideGuard{}, nil
	}
	return guards[0], guards
}

func guideGuardsQualityScore(start registry.SiteTaskGuideGuard, guards []registry.SiteTaskGuideGuard) int {
	score := 0
	for _, guard := range append([]registry.SiteTaskGuideGuard{start}, guards...) {
		if guard.URLPattern != "" {
			score += 1
		}
		score += len(guard.URLIncludes)
		score += len(guard.RequiredText)
		score += len(guard.RequiredControls) * 3
		if guard.ActiveOverlayHint != "" {
			score += 2
		}
	}
	return score
}

func buildSiteTaskGuideSteps(steps []registry.ActionStep) []registry.SiteTaskGuideStep {
	result := []registry.SiteTaskGuideStep{}
	for _, step := range steps {
		if step.IsBranchNoise {
			continue
		}
		target := guideStepTarget(step)
		if target == "" {
			continue
		}
		text := guideStepText(step, target)
		if text == "" || looksLikeDynamicTarget(text) {
			continue
		}
		result = append(result, registry.SiteTaskGuideStep{
			ID:         firstNonEmpty(step.ID, "step_"+stableTextID(target)),
			Index:      len(result),
			Text:       text,
			Goal:       text,
			ActionType: step.ActionType,
			Target:     target,
			PageTitle:  safeGuideText(step.PageStateID),
			SemanticTarget: registry.SiteTaskGuideStepTarget{
				Text: target,
			},
		})
		if len(result) >= 6 {
			break
		}
	}
	return result
}

func buildSiteTaskGuideStepsFromTaskSummary(summary string) []registry.SiteTaskGuideStep {
	result := []registry.SiteTaskGuideStep{}
	for _, line := range strings.Split(summary, "\n") {
		text := numberedGuideInstructionText(line)
		if text == "" {
			continue
		}
		target := stableTargetFromStepText(text)
		if target == "" {
			target = stableTargetCandidate(text)
		}
		if target == "" {
			continue
		}
		result = append(result, registry.SiteTaskGuideStep{
			ID:         "summary_step_" + stableTextID(text),
			Index:      len(result),
			Text:       text,
			Goal:       text,
			ActionType: guideInstructionActionType(text),
			Target:     target,
			SemanticTarget: registry.SiteTaskGuideStepTarget{
				Text: target,
			},
		})
		if len(result) >= 6 {
			break
		}
	}
	return result
}

func numberedGuideInstructionText(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^\s*\d+[\.\)、）]\s*(.+)$`),
		regexp.MustCompile(`^\s*[-*•]\s*(.+)$`),
		regexp.MustCompile(`^\s*\*{0,2}第[一二三四五六七八九十\d]+步[:：]\s*(.+?)\*{0,2}\s*$`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		return normalizeGuideInstructionText(matches[1])
	}
	return ""
}

func normalizeGuideInstructionText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n-*•")
	value = regexp.MustCompile(`^\*+|\*+$`).ReplaceAllString(value, "")
	value = strings.NewReplacer(
		"`", "",
		"\"", "",
		"'", "",
		"“", "",
		"”", "",
		"‘", "",
		"’", "",
	).Replace(value)
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
	value = regexp.MustCompile(`([\p{Han}])\s+([\p{Han}])`).ReplaceAllString(value, "$1$2")
	value = regexp.MustCompile(`\s+([，。,.])`).ReplaceAllString(value, "$1")
	value = strings.Trim(value, " \t\r\n:：,，.。")
	if value == "" || containsInstanceValue(value) {
		return ""
	}
	if !regexp.MustCompile(`^(点击|进入|打开|选择|切换|切换到|输入|搜索|筛选|确认|提交|等待|跳转)`).MatchString(value) {
		return ""
	}
	value = safeGuideText(value)
	if value == "" || looksLikeDynamicTarget(value) {
		return ""
	}
	if !strings.HasSuffix(value, "。") && !strings.HasSuffix(value, ".") {
		value += "。"
	}
	return value
}

func guideInstructionActionType(text string) registry.StepType {
	if strings.Contains(text, "输入") || strings.Contains(strings.ToLower(text), "fill") {
		return registry.StepFill
	}
	if strings.Contains(text, "选择") || strings.Contains(strings.ToLower(text), "select") {
		return registry.StepSelect
	}
	if strings.Contains(text, "等待") || strings.Contains(strings.ToLower(text), "wait") {
		return registry.StepWait
	}
	return registry.StepClick
}

func guideStepTarget(step registry.ActionStep) string {
	target := safeGuideText(step.TargetName)
	if target != "" && !looksLikeDynamicTarget(target) {
		return target
	}
	for _, source := range []string{step.ReasoningSummary, step.ResultSummary} {
		if candidate := stableTargetFromStepText(source); candidate != "" {
			return candidate
		}
	}
	return ""
}

func stableTargetFromStepText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`<(?:button|div|span|a)[^>]*>([^<>/]{1,40})\s*/?>`),
		regexp.MustCompile(`[\"“”']([^\"“”']{1,40})[\"“”'](?:按钮|标签|链接|菜单|选项)?`),
		regexp.MustCompile(`(?:点击|打开|选择|切换到|进入)\s*([^，。,.（）()]{2,40}?)(?:按钮|标签|链接|菜单|选项|页面|页|记录|$)`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(value, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			candidate := stableTargetCandidate(match[1])
			if candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func stableTargetCandidate(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n:：,，.。/\\[]()（）\"'“”")
	value = removeGeneralizedInstancePlaceholder(value)
	value = regexp.MustCompile(`^\d+\s*`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
	if value == "" || len([]rune(value)) > 40 {
		return ""
	}
	if looksLikeDynamicTarget(value) || ContainsSensitiveMaterial(value) || containsInstanceValue(value) {
		return ""
	}
	if regexp.MustCompile(`(?i)^(clicked element|input text|waited for|element|index|node|完成|操作)$`).MatchString(value) {
		return ""
	}
	return safeGuideText(value)
}

func removeGeneralizedInstancePlaceholder(value string) string {
	generalized := generalizeInstanceValues(value)
	if generalized == value || !strings.Contains(generalized, "{{instance_name}}") {
		return value
	}
	stripped := strings.ReplaceAll(generalized, "{{instance_name}}", "")
	stripped = strings.Trim(stripped, " \t\r\n:：,，.。/\\[]()（）\"'“”")
	stripped = regexp.MustCompile(`\s+`).ReplaceAllString(stripped, " ")
	if stripped == "" {
		return value
	}
	return stripped
}

func (service *SiteTaskGuideService) buildUIStateEntriesFromActionSteps(ctx context.Context, steps []registry.ActionStep) []registry.SiteTaskGuideUIStateEntry {
	result := []registry.SiteTaskGuideUIStateEntry{}
	reusableIndex := 0
	for _, step := range steps {
		if step.IsBranchNoise {
			continue
		}
		target := guideStepTarget(step)
		if target == "" {
			continue
		}
		if step.BeforeObservation != nil {
			result = append(result, uiStateEntryFromObservation(*step.BeforeObservation, reusableIndex, target, step.ActionType))
		} else if page, ok := service.pageStateObservation(ctx, step.PageStateID); ok {
			result = append(result, uiStateEntryFromObservation(page, reusableIndex, target, step.ActionType))
		}
		if step.AfterObservation != nil {
			result = append(result, uiStateEntryFromObservation(*step.AfterObservation, reusableIndex+1, target, step.ActionType))
		}
		reusableIndex++
	}
	return mergeUIStateEntries(nil, result)
}

func (service *SiteTaskGuideService) pageStateObservation(ctx context.Context, pageStateID string) (registry.PageObservationSignal, bool) {
	pageStateID = strings.TrimSpace(pageStateID)
	if pageStateID == "" || service.repo == nil {
		return registry.PageObservationSignal{}, false
	}
	page, err := service.repo.GetPageState(ctx, pageStateID)
	if err != nil {
		return registry.PageObservationSignal{}, false
	}
	return registry.PageObservationSignal{
		ProjectID:         page.ProjectID,
		Site:              page.Site,
		URLPattern:        page.URLPattern,
		Title:             page.CanonicalTitle,
		VisibleTextSample: strings.Join(firstNStrings(append(page.RequiredText, page.HardRules.TextAll...), 5), " "),
		ControlSignatures: firstNControls(append(page.RequiredControls, page.HardRules.ControlsAll...), 5),
	}, true
}

func uiStateEntryFromObservation(signal registry.PageObservationSignal, stepOffset int, targetName string, actionType registry.StepType) registry.SiteTaskGuideUIStateEntry {
	evidence := registry.SiteTaskGuideUIStateEvidence{
		TitleAny:        firstNStrings([]string{signal.Title}, 1),
		BreadcrumbAny:   firstNStrings(signal.Breadcrumbs, 3),
		ActiveTabAny:    firstNStrings(signal.ActiveTabs, 3),
		TextAny:         firstNStrings(splitVisibleText(signal.VisibleTextSample), 4),
		ControlsAll:     firstNControls(stableControlsForState(signal.ControlSignatures, targetName), 4),
		TableHeadersAny: tableHeaderEvidence(signal.Tables),
	}
	if len(evidence.ControlsAll) == 0 && strings.TrimSpace(targetName) != "" {
		evidence.ControlsAny = []registry.ControlSignature{{Name: safeGuideText(targetName)}}
	}
	for _, surface := range signal.ActiveSurfaces {
		evidence.ActiveSurfacesAny = append(evidence.ActiveSurfacesAny, registry.ActiveSurfaceSignal{
			SurfaceType: surface.SurfaceType,
			Title:       safeGuideText(surface.Title),
			Text:        firstNStrings(surface.Text, 3),
			Controls:    firstNControls(surface.Controls, 4),
		})
	}
	stateType := registry.SiteTaskGuideUIStatePage
	if len(signal.ActiveTabs) > 0 {
		stateType = registry.SiteTaskGuideUIStateTab
	}
	if len(signal.ActiveSurfaces) > 0 {
		switch signal.ActiveSurfaces[0].SurfaceType {
		case registry.SurfaceModal:
			stateType = registry.SiteTaskGuideUIStateModal
		case registry.SurfaceDrawer:
			stateType = registry.SiteTaskGuideUIStateDrawer
		case registry.SurfacePopover:
			stateType = registry.SiteTaskGuideUIStatePopover
		case registry.SurfaceWizard:
			stateType = registry.SiteTaskGuideUIStateWizard
		}
	}
	name := safeGuideText(firstNonEmpty(signal.Title, strings.Join(signal.ActiveTabs, " "), targetName, string(actionType)))
	idParts := []string{signal.Site, signal.URLPattern, signal.Title, strings.Join(signal.ActiveTabs, "|"), controlTextForGuide(signal.ControlSignatures), string(stateType)}
	urlFamily := firstNonEmpty(signal.URLFamily, urlPathFamily(signal.URLPattern), urlPathFamily(signal.URL))
	urlPattern := firstNonEmpty(urlFamily, signal.URLPattern, signal.URL)
	return registry.SiteTaskGuideUIStateEntry{
		ID:         "state_" + stableTextID(strings.Join(idParts, "\x00")),
		Name:       name,
		StateType:  stateType,
		StepOffset: stepOffset,
		RouteScope: registry.SiteTaskGuideRouteScope{
			URLIncludes: firstNStrings([]string{urlFamily}, 1),
			URLPattern:  urlPattern,
		},
		Evidence:         evidence,
		MinimumScore:     defaultGeneratedUIStateMinimumScore(evidence, stateType),
		RequiredEvidence: []string{"site", "task_intent", "ui_state"},
		Confidence:       0.72,
		Status:           registry.StatusActive,
	}
}

func stableControlsForState(controls []registry.ControlSignature, targetName string) []registry.ControlSignature {
	if strings.TrimSpace(targetName) == "" {
		return controls
	}
	result := []registry.ControlSignature{}
	for _, control := range controls {
		if strings.EqualFold(control.Name, targetName) || strings.Contains(strings.ToLower(control.Name), strings.ToLower(targetName)) {
			result = append(result, control)
		}
	}
	if len(result) > 0 {
		return result
	}
	return controls
}

func splitVisibleText(value string) []string {
	parts := strings.Fields(strings.ReplaceAll(value, "\n", " "))
	return firstNStrings(parts, 6)
}

func tableHeaderEvidence(tables []registry.ObservationTableSignal) [][]string {
	result := [][]string{}
	for _, table := range tables {
		headers := firstNStrings(table.Headers, 6)
		if len(headers) > 0 {
			result = append(result, headers)
		}
	}
	return result
}

func defaultGeneratedUIStateMinimumScore(evidence registry.SiteTaskGuideUIStateEvidence, stateType registry.SiteTaskGuideUIStateType) float64 {
	score := 2.0
	if len(evidence.ControlsAll) > 0 {
		score += 2
	}
	if len(evidence.ActiveTabAny) > 0 {
		score += 1
	}
	if len(evidence.ActiveSurfacesAny) > 0 ||
		stateType == registry.SiteTaskGuideUIStateModal ||
		stateType == registry.SiteTaskGuideUIStateDrawer ||
		stateType == registry.SiteTaskGuideUIStatePopover ||
		stateType == registry.SiteTaskGuideUIStateWizard {
		score += 2
	}
	return score
}

func routeScopeFromUIStates(states []registry.SiteTaskGuideUIStateEntry) registry.SiteTaskGuideRouteScope {
	for _, state := range states {
		if state.RouteScope.URLPattern != "" || len(state.RouteScope.URLIncludes) > 0 {
			return state.RouteScope
		}
	}
	return registry.SiteTaskGuideRouteScope{}
}

func mergeUIStateEntries(existing, incoming []registry.SiteTaskGuideUIStateEntry) []registry.SiteTaskGuideUIStateEntry {
	result := append([]registry.SiteTaskGuideUIStateEntry{}, existing...)
	byID := map[string]int{}
	for index, state := range result {
		byID[state.ID] = index
	}
	for _, state := range incoming {
		if state.ID == "" {
			continue
		}
		if index, ok := byID[state.ID]; ok {
			if state.Confidence > result[index].Confidence {
				result[index].Confidence = state.Confidence
			}
			if len(state.Evidence.ControlsAll) > len(result[index].Evidence.ControlsAll) {
				result[index].Evidence.ControlsAll = state.Evidence.ControlsAll
			}
			continue
		}
		byID[state.ID] = len(result)
		result = append(result, state)
	}
	return result
}

func updateUIStateFeedback(states []registry.SiteTaskGuideUIStateEntry, feedback registry.SiteTaskGuideFeedback) []registry.SiteTaskGuideUIStateEntry {
	for index := range states {
		if states[index].ID != feedback.StateID {
			continue
		}
		switch feedback.Label {
		case registry.SiteTaskGuideFeedbackUsedHelpful:
			states[index].Confidence += 0.05
		case registry.SiteTaskGuideFeedbackAbandonedMismatch, registry.SiteTaskGuideFeedbackUsedMisleading:
			states[index].Confidence -= 0.12
		case registry.SiteTaskGuideFeedbackStale:
			states[index].Status = registry.StatusStale
			states[index].Confidence -= 0.2
		}
		if states[index].Confidence < 0.05 {
			states[index].Confidence = 0.05
		}
		if states[index].Confidence > 0.95 {
			states[index].Confidence = 0.95
		}
	}
	return states
}

func deriveTaskIntentTerms(summary string, steps []registry.SiteTaskGuideStep) registry.SiteTaskIntentTerms {
	text := strings.ToLower(strings.Join(append([]string{summary}, guideTaskStepTexts(steps)...), " "))
	positive := uniqueLimited(intentTermsFromText(text), 10)
	negative := []string{}
	if containsAnyLower(text, []string{"restore", "recover", "backup", "恢复", "备份"}) {
		positive = append(positive, "restore", "recover", "backup", "恢复", "备份")
		negative = append(negative, "delete", "remove", "drop", "cleanup", "删除", "移除", "销毁", "清理")
	}
	if containsAnyLower(text, []string{"reset", "credential", "password", "重置", "密码", "凭证"}) {
		positive = append(positive, "reset", "credential", "重置", "凭证")
		negative = append(negative, "delete", "remove", "disable", "删除", "禁用")
	}
	if containsAnyLower(text, []string{"delete", "remove", "drop", "cleanup", "删除", "移除", "销毁", "清理"}) {
		negative = append(negative, "restore", "recover", "恢复", "备份")
	}
	return registry.SiteTaskIntentTerms{Positive: uniqueLimited(positive, 12), Negative: uniqueLimited(negative, 12)}
}

func guideTaskStepTexts(steps []registry.SiteTaskGuideStep) []string {
	result := make([]string, 0, len(steps))
	for _, step := range steps {
		result = append(result, step.Text, step.Target)
	}
	return result
}

func intentTermsFromText(value string) []string {
	terms := []string{}
	for _, term := range strings.Fields(value) {
		term = strings.Trim(term, " \t\n\r,.，。:：;；/\\()[]{}")
		if len([]rune(term)) >= 2 && !strings.Contains(term, "{{") {
			terms = append(terms, term)
		}
	}
	for _, term := range []string{"恢复", "备份", "实例", "删除", "清理"} {
		if strings.Contains(value, term) {
			terms = append(terms, term)
		}
	}
	return terms
}

func mergeIntentTerms(left, right registry.SiteTaskIntentTerms) registry.SiteTaskIntentTerms {
	return registry.SiteTaskIntentTerms{
		Positive: uniqueLimited(append(left.Positive, right.Positive...), 12),
		Negative: uniqueLimited(append(left.Negative, right.Negative...), 12),
	}
}

func containsAnyLower(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func controlTextForGuide(controls []registry.ControlSignature) string {
	parts := make([]string, 0, len(controls))
	for _, control := range controls {
		parts = append(parts, strings.TrimSpace(control.Role+" "+control.Name))
	}
	return strings.Join(parts, "|")
}

func stableTextID(value string) string {
	raw := value
	slug := strings.ToLower(value)
	slug = regexp.MustCompile(`[^a-z0-9\p{Han}]+`).ReplaceAllString(slug, "_")
	slug = strings.Trim(slug, "_")
	hash := sha1.Sum([]byte(raw))
	suffix := hex.EncodeToString(hash[:4])
	if len([]rune(slug)) > 40 {
		slug = string([]rune(slug)[:40])
	}
	if slug == "" {
		slug = "entry"
	}
	return slug + "_" + suffix
}

func urlPathFamily(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(strings.TrimPrefix(value, "https://"), "http://")
	_, path, found := strings.Cut(value, "/")
	if !found {
		return ""
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	result := []string{}
	for _, part := range parts {
		if part == "" || likelyDynamicPathPart(part) {
			continue
		}
		result = append(result, part)
	}
	if len(result) == 0 {
		return ""
	}
	if len(result) == 1 {
		return "/" + result[0]
	}
	return "/" + strings.Join([]string{result[0], result[len(result)-1]}, "/")
}

func likelyDynamicPathPart(value string) bool {
	value = strings.TrimSpace(value)
	return strings.Contains(value, ":") ||
		strings.Contains(value, "-") ||
		regexp.MustCompile(`\d`).MatchString(value) ||
		regexp.MustCompile(`(?i)^[a-f0-9]{8,}$`).MatchString(value)
}

func guideStepText(step registry.ActionStep, target string) string {
	if summary := safeGuideText(step.ResultSummary); summary != "" && !containsInstanceValue(summary) && !looksLikeRawElementSummary(summary) {
		return summary
	}
	action := strings.TrimSpace(string(step.ActionType))
	switch step.ActionType {
	case registry.StepFill:
		value := strings.TrimSpace(step.ValueTemplate)
		if value == "" {
			value = "{{input_value}}"
		}
		return safeGuideText("Fill " + target + " with " + value + ".")
	case registry.StepSelect:
		value := strings.TrimSpace(step.ValueTemplate)
		if value == "" {
			value = "the required option"
		}
		return safeGuideText("Select " + value + " in " + target + ".")
	case registry.StepWait:
		return safeGuideText("Wait for " + target + ".")
	case registry.StepClick:
		return safeGuideText("点击" + target + "。")
	default:
		if action == "" {
			action = "click"
		}
		return safeGuideText(strings.Title(action) + " " + target + ".")
	}
}

func looksLikeRawElementSummary(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "clicked element") ||
		strings.Contains(lower, "input text") ||
		strings.Contains(lower, "<button") ||
		strings.Contains(lower, "<div") ||
		strings.Contains(lower, "<input") ||
		strings.Contains(lower, "element_")
}

func moduleFromGuideRun(ctx context.Context, repo registry.Repository, run registry.TaskRun) string {
	path := run.OptimizedPath
	if len(path) == 0 {
		path = run.OriginalPath
	}
	for _, pageID := range path {
		page, err := repo.GetPageState(ctx, pageID)
		if err != nil {
			continue
		}
		if module, ok := pagePayloadString(&page, "module"); ok {
			return module
		}
	}
	return ""
}

func guideConfidence(guide registry.SiteTaskGuide) float64 {
	score := 0.55 + float64(guide.SuccessCount)*0.08 - float64(guide.AbandonedCount)*0.04 - float64(guide.MisleadingCount)*0.15 - float64(guide.StaleCount)*0.2 - float64(guide.UnusedCount)*0.02
	if score < 0.05 {
		return 0.05
	}
	if score > 0.95 {
		return 0.95
	}
	return score
}

func usefulGuideIntent(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || ContainsSensitiveMaterial(value) || containsInstanceValue(value) {
		return false
	}
	withoutTemplates := regexp.MustCompile(`\{\{[^}]+\}\}`).ReplaceAllString(value, "")
	withoutTemplates = strings.TrimSpace(regexp.MustCompile(`[\s\pP]+`).ReplaceAllString(withoutTemplates, ""))
	return len([]rune(withoutTemplates)) >= 2
}

func stableGuideIntentKey(site, module, value string) string {
	key := strings.ToLower(strings.Join([]string{site, module, value}, " "))
	key = regexp.MustCompile(`\{\{[^}]+\}\}`).ReplaceAllString(key, "var")
	key = regexp.MustCompile(`[^a-z0-9\p{Han}]+`).ReplaceAllString(key, "_")
	key = strings.Trim(key, "_")
	if len([]rune(key)) > 80 {
		key = string([]rune(key)[:80])
	}
	if key == "" {
		key = "task"
	}
	return key
}

func safeGuideText(value string) string {
	value = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(value, " "))
	if value == "" || ContainsSensitiveMaterial(value) {
		return ""
	}
	value = generalizeInstanceValues(value)
	return TruncateSummary(value)
}

func generalizeInstanceValues(value string) string {
	replacements := []struct {
		pattern *regexp.Regexp
		token   string
	}{
		{regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`), "{{ip_address}}"},
		{regexp.MustCompile(`\b[A-Za-z]+-[A-Za-z0-9]+-[A-Za-z0-9]+\b`), "{{resource_name}}"},
		{regexp.MustCompile(`\b\d{8,}\b`), "{{numeric_id}}"},
	}
	result := regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_-]{2,}(\s*实例)`).ReplaceAllString(value, "{{instance_name}}$1")
	result = regexp.MustCompile(`(?i)\b[A-Za-z][A-Za-z0-9_-]*(?:\d|[-_])[A-Za-z0-9_-]*(\s+instance\b)`).ReplaceAllString(result, "{{instance_name}}$1")
	for _, replacement := range replacements {
		result = replacement.pattern.ReplaceAllString(result, replacement.token)
	}
	return result
}

func containsInstanceValue(value string) bool {
	generalized := generalizeInstanceValues(value)
	return generalized != value && !strings.Contains(value, "{{")
}

func looksLikeDynamicTarget(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return regexp.MustCompile(`(^|[_\s-])(element|index|node)[_\s=-]*\d+`).MatchString(lower) ||
		regexp.MustCompile(`^\d+$`).MatchString(lower)
}

func defaultGuideProjectID(projectID string) string {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "default"
	}
	return projectID
}

func uniqueLimited(values []string, limit int) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func uniqueSourceRefs(refs []registry.MemorySourceRef) []registry.MemorySourceRef {
	seen := map[string]bool{}
	result := []registry.MemorySourceRef{}
	for _, ref := range refs {
		key := ref.Type + ":" + ref.ID
		if ref.ID == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, ref)
	}
	return result
}

func firstNStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstNControls(values []registry.ControlSignature, limit int) []registry.ControlSignature {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
